package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/creds"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
)

var (
	loginURL       string
	loginEmail     string
	loginWorkspace string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to a Huly workspace (interactive password prompt)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if loginURL == "" {
			loginURL = viper.GetString("server.url") // set by config (Task 13); empty until then
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
	rootCmd.AddCommand(loginCmd, logoutCmd, whoamiCmd)
}
