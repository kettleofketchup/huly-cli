package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/creds"
)

func TestRunLoginPersistsCredentials(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HULY_TOKEN", "")
	t.Setenv("HULY_ENDPOINT", "")
	t.Setenv("HULY_WORKSPACE", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/config.json" {
			_ = json.NewEncoder(w).Encode(map[string]any{"ACCOUNTS_URL": "http://" + r.Host + "/acct"})
			return
		}
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Method {
		case "login":
			_ = json.NewEncoder(w).Encode(map[string]any{"account": "acc-1", "token": "tok-acct"})
		case "selectWorkspace":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"endpoint": "wss://t.example", "token": "tok-ws",
				"workspace": "ws-uuid", "account": "acc-1",
			})
		}
	}))
	defer srv.Close()

	if err := runLogin(context.Background(), srv.URL, "a@b.c", "pw", "myws"); err != nil {
		t.Fatalf("runLogin: %v", err)
	}
	got, err := creds.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Token != "tok-ws" || got.Endpoint != "https://t.example" || got.Workspace != "ws-uuid" {
		t.Fatalf("creds = %+v", got)
	}
}

func TestRunLoginOTPPersistsCredentials(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HULY_TOKEN", "")
	t.Setenv("HULY_ENDPOINT", "")
	t.Setenv("HULY_WORKSPACE", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/config.json" {
			_ = json.NewEncoder(w).Encode(map[string]any{"ACCOUNTS_URL": "http://" + r.Host + "/acct"})
			return
		}
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		switch req.Method {
		case "loginOtp":
			_ = json.NewEncoder(w).Encode(map[string]any{"sent": true})
		case "validateOtp":
			if req.Params["code"] != "654321" {
				t.Errorf("bad code %v", req.Params["code"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"account": "acc-1", "token": "tok-acct"})
		case "selectWorkspace":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"endpoint": "wss://t.example", "token": "tok-ws",
				"workspace": "ws-uuid", "account": "acc-1",
			})
		}
	}))
	defer srv.Close()

	codeFn := func() (string, error) { return "654321", nil }
	if err := runLoginOTP(context.Background(), srv.URL, "a@b.c", "myws", codeFn); err != nil {
		t.Fatalf("runLoginOTP: %v", err)
	}
	got, err := creds.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Token != "tok-ws" || got.Endpoint != "https://t.example" || got.Workspace != "ws-uuid" || got.Account != "acc-1" {
		t.Fatalf("creds = %+v", got)
	}
}
