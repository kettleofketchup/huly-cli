package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

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
