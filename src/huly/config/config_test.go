package config

import (
	"testing"

	"github.com/spf13/viper"
)

func TestDefaultsAndUnmarshal(t *testing.T) {
	viper.Reset()
	Defaults()
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Output != "table" {
		t.Fatalf("default output = %q", cfg.Output)
	}
	viper.Set("defaults.project", "PROJ")
	viper.Set("server.url", "https://h")
	cfg, _ = Load()
	if cfg.Defaults.Project != "PROJ" || cfg.Server.URL != "https://h" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoginConfigUnmarshal(t *testing.T) {
	viper.Reset()
	Defaults()
	viper.Set("login.email", "me@corp.com")
	viper.Set("login.workspace", "acme")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Login.Email != "me@corp.com" || cfg.Login.Workspace != "acme" {
		t.Fatalf("login cfg = %+v", cfg.Login)
	}
}
