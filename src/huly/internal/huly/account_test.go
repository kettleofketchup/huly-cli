package huly

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// result wraps a value in the account service's JSON-RPC success envelope.
func result(v any) map[string]any { return map[string]any{"result": v} }

func TestRewriteScheme(t *testing.T) {
	cases := map[string]string{
		"wss://h/api": "https://h/api",
		"ws://h/api":  "http://h/api",
		"https://h":   "https://h",
	}
	for in, want := range cases {
		if got := rewriteScheme(in); got != want {
			t.Fatalf("rewriteScheme(%q)=%q want %q", in, got, want)
		}
	}
}

func TestLoginAndSelectWorkspace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "login":
			if req.Params["email"] != "a@b.c" {
				t.Errorf("bad email %v", req.Params["email"])
			}
			_ = json.NewEncoder(w).Encode(result(map[string]any{"account": "acc-1", "token": "tok-acct"}))
		case "selectWorkspace":
			_ = json.NewEncoder(w).Encode(result(map[string]any{
				"endpoint": "wss://transactor.example", "token": "tok-ws",
				"workspace": "ws-uuid", "workspaceUrl": "myws", "account": "acc-1",
			}))
		default:
			t.Errorf("unexpected method %q", req.Method)
		}
	}))
	defer srv.Close()

	ac := NewAccountClient(srv.URL)
	li, err := ac.Login(context.Background(), "a@b.c", "pw")
	if err != nil || li.Token != "tok-acct" || li.Account != "acc-1" {
		t.Fatalf("login = %+v, err=%v", li, err)
	}
	ws, err := ac.SelectWorkspace(context.Background(), li.Token, "myws")
	if err != nil {
		t.Fatalf("selectWorkspace err: %v", err)
	}
	if ws.Endpoint != "https://transactor.example" { // scheme rewritten
		t.Fatalf("endpoint = %q", ws.Endpoint)
	}
	if ws.Token != "tok-ws" || ws.Workspace != "ws-uuid" {
		t.Fatalf("ws = %+v", ws)
	}
}

func TestLoginOtpAndValidateOtp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "loginOtp":
			if req.Params["email"] != "a@b.c" {
				t.Errorf("bad email %v", req.Params["email"])
			}
			_ = json.NewEncoder(w).Encode(result(map[string]any{"sent": true, "retryOn": 1700000000000}))
		case "validateOtp":
			if req.Params["code"] != "123456" {
				t.Errorf("bad code %v", req.Params["code"])
			}
			_ = json.NewEncoder(w).Encode(result(map[string]any{"account": "acc-1", "token": "tok-otp"}))
		default:
			t.Errorf("unexpected method %q", req.Method)
		}
	}))
	defer srv.Close()

	ac := NewAccountClient(srv.URL)
	oi, err := ac.LoginOtp(context.Background(), "a@b.c")
	if err != nil || !oi.Sent {
		t.Fatalf("loginOtp = %+v, err=%v", oi, err)
	}
	li, err := ac.ValidateOtp(context.Background(), "a@b.c", "123456")
	if err != nil || li.Token != "tok-otp" || li.Account != "acc-1" {
		t.Fatalf("validateOtp = %+v, err=%v", li, err)
	}
}

// The account service returns failures as HTTP 200 with an {"error":...}
// envelope. rpc must surface these as errors, not silently yield a zero value.
func TestAccountErrorEnvelopeSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Note: status 200 even though this is an error.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "platform:status:Unauthorized", "params": map[string]any{}},
		})
	}))
	defer srv.Close()

	ac := NewAccountClient(srv.URL)
	ws, err := ac.SelectWorkspace(context.Background(), "", "myws")
	if err == nil {
		t.Fatalf("expected error from error-envelope, got ws=%+v", ws)
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

// A non-Unauthorized error code must also surface (not be swallowed).
func TestAccountErrorEnvelopeGenericSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"code": "platform:status:BadRequest", "params": map[string]any{}},
		})
	}))
	defer srv.Close()

	ac := NewAccountClient(srv.URL)
	_, err := ac.ValidateOtp(context.Background(), "a@b.c", "wrong")
	if err == nil {
		t.Fatal("expected error from BadRequest envelope, got nil")
	}
}
