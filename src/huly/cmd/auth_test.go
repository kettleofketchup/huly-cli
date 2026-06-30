package cmd

import (
	"errors"
	"testing"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/creds"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
)

func TestRunSetToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HULY_TOKEN", "")
	if err := runSetToken("https://e/api", "ws", "tok"); err != nil {
		t.Fatalf("runSetToken: %v", err)
	}
	got, _ := creds.Load()
	if got.Token != "tok" || got.Workspace != "ws" || got.Endpoint != "https://e/api" {
		t.Fatalf("creds = %+v", got)
	}
}

func TestMapAuthErr(t *testing.T) {
	// mapAuthErr is defined in cmd/client.go (Task 10).
	err := mapAuthErr(huly.ErrUnauthorized)
	if err == nil || !errors.Is(err, huly.ErrUnauthorized) {
		t.Fatalf("expected wrapped unauthorized, got %v", err)
	}
}
