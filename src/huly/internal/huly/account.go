package huly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func rewriteScheme(endpoint string) string {
	if strings.HasPrefix(endpoint, "wss://") {
		return "https://" + strings.TrimPrefix(endpoint, "wss://")
	}
	if strings.HasPrefix(endpoint, "ws://") {
		return "http://" + strings.TrimPrefix(endpoint, "ws://")
	}
	return endpoint
}

// LoginInfo is the account-service login result.
type LoginInfo struct {
	Account string `json:"account"`
	Token   string `json:"token"`
}

// WorkspaceLoginInfo is the selectWorkspace result (endpoint scheme rewritten).
type WorkspaceLoginInfo struct {
	Endpoint     string `json:"endpoint"`
	Token        string `json:"token"`
	Workspace    string `json:"workspace"`
	WorkspaceURL string `json:"workspaceUrl"`
	Account      string `json:"account"`
}

// AccountClient talks JSON-RPC to the Huly account service.
type AccountClient struct {
	url    string
	client *http.Client
}

func NewAccountClient(accountsURL string) *AccountClient {
	return &AccountClient{url: strings.TrimRight(accountsURL, "/"), client: http.DefaultClient}
}

// accountError is the account service's error payload. The service returns
// failures as an {"error":...} envelope — usually at HTTP 200, sometimes 404
// (unknown method) — so the status code alone cannot be trusted.
type accountError struct {
	Severity string         `json:"severity"`
	Code     string         `json:"code"`
	Params   map[string]any `json:"params"`
}

func (c *AccountClient) rpc(ctx context.Context, token, method string, params map[string]any, out any) error {
	body, _ := json.Marshal(map[string]any{"method": method, "params": params})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("account rpc %s: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}

	// The account service wraps responses in a JSON-RPC envelope:
	//   success -> {"result": <data>}
	//   failure -> {"error": {"code": "platform:status:...", ...}}  (often HTTP 200)
	// Decode the envelope, not the bare struct, or every field silently
	// stays zero and server errors are swallowed.
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("account rpc %s: read body: %w", method, err)
	}
	var env struct {
		Result json.RawMessage `json:"result"`
		Error  *accountError   `json:"error"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("account rpc %s: decode envelope (status %d): %w", method, resp.StatusCode, err)
	}
	if env.Error != nil {
		if strings.Contains(env.Error.Code, "Unauthorized") {
			return ErrUnauthorized
		}
		return fmt.Errorf("account rpc %s: %s", method, env.Error.Code)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("account rpc %s: status %d: %s", method, resp.StatusCode, string(raw))
	}
	if out != nil && len(env.Result) > 0 {
		if err := json.Unmarshal(env.Result, out); err != nil {
			return fmt.Errorf("account rpc %s: decode result: %w", method, err)
		}
	}
	return nil
}

func (c *AccountClient) Login(ctx context.Context, email, password string) (LoginInfo, error) {
	var li LoginInfo
	err := c.rpc(ctx, "", "login", map[string]any{"email": email, "password": password}, &li)
	return li, err
}

// OtpInfo is the result of requesting a one-time login code.
type OtpInfo struct {
	Sent    bool  `json:"sent"`
	RetryOn int64 `json:"retryOn"`
}

// LoginOtp requests a one-time login code be emailed to the account. This is the
// password-free path for accounts that use external (OAuth/SSO) login.
func (c *AccountClient) LoginOtp(ctx context.Context, email string) (OtpInfo, error) {
	var oi OtpInfo
	err := c.rpc(ctx, "", "loginOtp", map[string]any{"email": email}, &oi)
	return oi, err
}

// ValidateOtp exchanges an emailed one-time code for a login token.
func (c *AccountClient) ValidateOtp(ctx context.Context, email, code string) (LoginInfo, error) {
	var li LoginInfo
	err := c.rpc(ctx, "", "validateOtp", map[string]any{"email": email, "code": code}, &li)
	return li, err
}

func (c *AccountClient) SelectWorkspace(ctx context.Context, token, workspaceURL string) (WorkspaceLoginInfo, error) {
	var ws WorkspaceLoginInfo
	err := c.rpc(ctx, token, "selectWorkspace",
		map[string]any{"workspaceUrl": workspaceURL, "kind": "external"}, &ws)
	if err != nil {
		return ws, err
	}
	ws.Endpoint = rewriteScheme(ws.Endpoint)
	return ws, nil
}
