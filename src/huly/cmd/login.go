package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/creds"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
)

var (
	loginURL           string
	loginEmail         string
	loginWorkspace     string
	loginOTP           bool
	loginNoInteractive bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to a Huly workspace (password, or --otp for external/SSO accounts)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if loginOTP {
			return runLoginOTPInteractive(cmd.Context())
		}
		if loginURL == "" {
			loginURL = viper.GetString("server.url")
		}
		if loginURL == "" || loginEmail == "" || loginWorkspace == "" {
			return fmt.Errorf("--url, --email and --workspace are required")
		}
		fmt.Fprint(os.Stderr, "Password: ")
		pw, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}
		return runLogin(cmd.Context(), loginURL, loginEmail, string(pw), loginWorkspace)
	},
}

// otpPrefill resolves each field to its flag value, else the config value.
func otpPrefill(flagURL, flagEmail, flagWS string) otpInputs {
	pick := func(flag, key string) string {
		if flag != "" {
			return flag
		}
		return viper.GetString(key)
	}
	return otpInputs{
		URL:       pick(flagURL, "server.url"),
		Email:     pick(flagEmail, "login.email"),
		Workspace: pick(flagWS, "login.workspace"),
	}
}

// saveOTPInputs persists URL/email/workspace to the config file for autofill.
func saveOTPInputs(in otpInputs) error {
	return writeConfigValues(resolveConfigPath(), map[string]string{
		"server.url":      in.URL,
		"login.email":     in.Email,
		"login.workspace": in.Workspace,
	})
}

// otpInteractive reports whether we should show the TUI: a real terminal on
// both stdin and stderr, and --no-interactive was not passed.
func otpInteractive() bool {
	if loginNoInteractive {
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
}

// runLoginOTPInteractive resolves inputs (TUI when on a terminal, plain
// stdin otherwise) and runs the OTP login.
func runLoginOTPInteractive(ctx context.Context) error {
	prefill := otpPrefill(loginURL, loginEmail, loginWorkspace)

	if !otpInteractive() {
		// Non-interactive: require resolved fields, use the stdin code prompt.
		if prefill.URL == "" || prefill.Email == "" || prefill.Workspace == "" {
			return fmt.Errorf("--url, --email and --workspace are required")
		}
		return runLoginOTP(ctx, prefill.URL, prefill.Email, prefill.Workspace, promptCode)
	}

	in, err := collectOTPInputs(prefill)
	if err != nil {
		return err
	}
	if in.Save {
		if err := saveOTPInputs(in); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not save config: %v\n", err)
		}
	}
	return runLoginOTP(ctx, in.URL, in.Email, in.Workspace, promptCodeTUI)
}

// promptCode reads a one-time code from stdin (prompt on stderr).
func promptCode() (string, error) {
	fmt.Fprint(os.Stderr, "Enter the code sent to your email: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// runLoginOTP performs the email one-time-code login flow: request a code,
// read it, exchange it for a token, select the workspace, and persist creds.
func runLoginOTP(ctx context.Context, baseURL, email, workspace string, codeFn func() (string, error)) error {
	sc, err := huly.LoadServerConfig(ctx, baseURL)
	if err != nil {
		return err
	}
	ac := huly.NewAccountClient(sc.AccountsURL)
	if _, err := ac.LoginOtp(ctx, email); err != nil {
		return fmt.Errorf("request otp: %w", err)
	}
	fmt.Fprintf(os.Stderr, "A one-time login code was sent to %s.\n", email)
	code, err := codeFn()
	if err != nil {
		return fmt.Errorf("read code: %w", err)
	}
	li, err := ac.ValidateOtp(ctx, email, code)
	if err != nil {
		return fmt.Errorf("validate otp: %w", err)
	}
	ws, err := ac.SelectWorkspace(ctx, li.Token, workspace)
	if err != nil {
		return fmt.Errorf("select workspace: %w", err)
	}
	return creds.Save(creds.Credentials{
		Endpoint:  ws.Endpoint,
		Workspace: ws.Workspace,
		Token:     ws.Token,
		Account:   li.Account,
	})
}

// runLogin performs the full login + selectWorkspace + persist flow.
func runLogin(ctx context.Context, baseURL, email, password, workspace string) error {
	sc, err := huly.LoadServerConfig(ctx, baseURL)
	if err != nil {
		return err
	}
	ac := huly.NewAccountClient(sc.AccountsURL)
	li, err := ac.Login(ctx, email, password)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	ws, err := ac.SelectWorkspace(ctx, li.Token, workspace)
	if err != nil {
		return fmt.Errorf("select workspace: %w", err)
	}
	return creds.Save(creds.Credentials{
		Endpoint:  ws.Endpoint,
		Workspace: ws.Workspace,
		Token:     ws.Token,
		Account:   li.Account,
	})
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove stored credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := creds.Clear(); err != nil {
			return err
		}
		fmt.Println("Logged out.")
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the current account and workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := creds.Load()
		if err != nil {
			return err
		}
		rc := huly.NewRestClient(c.Endpoint, c.Workspace, c.Token)
		acct, err := rc.GetAccount(cmd.Context())
		if err != nil {
			return mapAuthErr(err)
		}
		fmt.Printf("account: %s\nrole:    %s\nworkspace: %s\nendpoint:  %s\n",
			acct.UUID, acct.Role, c.Workspace, c.Endpoint)
		return nil
	},
}

func init() {
	loginCmd.Flags().StringVar(&loginURL, "url", "", "Huly base URL (defaults to server.url in config)")
	loginCmd.Flags().StringVar(&loginEmail, "email", "", "account email")
	loginCmd.Flags().StringVar(&loginWorkspace, "workspace", "", "workspace url/name")
	loginCmd.Flags().BoolVar(&loginOTP, "otp", false, "log in with an emailed one-time code (for external/SSO accounts with no password)")
	loginCmd.Flags().BoolVar(&loginNoInteractive, "no-interactive", false, "skip the form; use plain stdin prompts (for scripts/CI)")
	rootCmd.AddCommand(loginCmd, logoutCmd, whoamiCmd)
}
