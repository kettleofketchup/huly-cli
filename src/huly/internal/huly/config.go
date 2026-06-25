package huly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ServerConfig is the subset of /config.json the CLI needs.
type ServerConfig struct {
	AccountsURL string `json:"ACCOUNTS_URL"`
}

// LoadServerConfig fetches {baseURL}/config.json.
func LoadServerConfig(ctx context.Context, baseURL string) (ServerConfig, error) {
	url := strings.TrimRight(baseURL, "/") + "/config.json"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ServerConfig{}, fmt.Errorf("load config.json: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ServerConfig{}, fmt.Errorf("load config.json: status %d", resp.StatusCode)
	}
	var cfg ServerConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return ServerConfig{}, fmt.Errorf("decode config.json: %w", err)
	}
	return cfg, nil
}
