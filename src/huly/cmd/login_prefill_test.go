package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// newOTPFlagCmd builds a bare *cobra.Command mirroring loginCmd's three
// override flags, for testing otpSaveDefault without touching global state.
func newOTPFlagCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "login"}
	cmd.Flags().String("url", "", "")
	cmd.Flags().String("email", "", "")
	cmd.Flags().String("workspace", "", "")
	return cmd
}

func TestOtpSaveDefault(t *testing.T) {
	t.Run("no flags changed defaults to true", func(t *testing.T) {
		cmd := newOTPFlagCmd()
		if err := cmd.ParseFlags(nil); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := otpSaveDefault(cmd); got != true {
			t.Fatalf("otpSaveDefault = %v, want true", got)
		}
	})

	t.Run("--email changed defaults to false", func(t *testing.T) {
		cmd := newOTPFlagCmd()
		if err := cmd.ParseFlags([]string{"--email", "x@y.com"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := otpSaveDefault(cmd); got != false {
			t.Fatalf("otpSaveDefault = %v, want false", got)
		}
	})

	t.Run("--url changed defaults to false", func(t *testing.T) {
		cmd := newOTPFlagCmd()
		if err := cmd.ParseFlags([]string{"--url", "https://other"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := otpSaveDefault(cmd); got != false {
			t.Fatalf("otpSaveDefault = %v, want false", got)
		}
	})

	t.Run("--workspace changed defaults to false", func(t *testing.T) {
		cmd := newOTPFlagCmd()
		if err := cmd.ParseFlags([]string{"--workspace", "ws2"}); err != nil {
			t.Fatalf("parse: %v", err)
		}
		if got := otpSaveDefault(cmd); got != false {
			t.Fatalf("otpSaveDefault = %v, want false", got)
		}
	})
}

func TestOTPPrefillPrefersFlagThenConfig(t *testing.T) {
	viper.Reset()
	viper.Set("server.url", "https://cfg")
	viper.Set("login.email", "cfg@x.com")
	viper.Set("login.workspace", "cfgws")

	// Flags win when present.
	got := otpPrefill("https://flag", "", "")
	if got.URL != "https://flag" {
		t.Fatalf("url = %q, want flag", got.URL)
	}
	// Config fills the blanks.
	if got.Email != "cfg@x.com" || got.Workspace != "cfgws" {
		t.Fatalf("prefill = %+v", got)
	}
}

func TestSaveOTPInputsWritesThreeKeys(t *testing.T) {
	viper.Reset()
	path := filepath.Join(t.TempDir(), "huly.yaml")
	viper.SetConfigFile(path)

	err := saveOTPInputs(otpInputs{URL: "https://h", Email: "e@x.com", Workspace: "ws", Save: true})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	b, _ := os.ReadFile(path)
	got := string(b)
	for _, want := range []string{"https://h", "e@x.com", "ws"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q; file=%q", want, got)
		}
	}
}
