package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/creds"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
)

func TestRunSetToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/account/ws" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"uuid": "acc-9", "role": "owner"})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HULY_TOKEN", "")
	t.Setenv("HULY_ENDPOINT", "")
	t.Setenv("HULY_WORKSPACE", "")

	if err := runSetToken(context.Background(), srv.URL, "ws", "tok"); err != nil {
		t.Fatalf("runSetToken: %v", err)
	}
	got, _ := creds.Load()
	if got.Token != "tok" || got.Workspace != "ws" || got.Endpoint != srv.URL {
		t.Fatalf("creds = %+v", got)
	}
	if got.Account != "acc-9" {
		t.Fatalf("expected Account=acc-9, got %q", got.Account)
	}
}

func TestMapAuthErr(t *testing.T) {
	// mapAuthErr is defined in cmd/client.go (Task 10).
	err := mapAuthErr(huly.ErrUnauthorized)
	if err == nil || !errors.Is(err, huly.ErrUnauthorized) {
		t.Fatalf("expected wrapped unauthorized, got %v", err)
	}
}
