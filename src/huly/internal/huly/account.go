package huly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("account rpc %s: status %d", method, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *AccountClient) Login(ctx context.Context, email, password string) (LoginInfo, error) {
	var li LoginInfo
	err := c.rpc(ctx, "", "login", map[string]any{"email": email, "password": password}, &li)
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
