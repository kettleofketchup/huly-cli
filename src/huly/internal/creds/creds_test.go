package creds

import (
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// ensure env overrides are absent
	t.Setenv("HULY_TOKEN", "")
	t.Setenv("HULY_ENDPOINT", "")
	t.Setenv("HULY_WORKSPACE", "")

	in := Credentials{Endpoint: "https://e/api", Workspace: "ws", Token: "tok", Account: "acc"}
	if err := Save(in); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != in {
		t.Fatalf("round trip = %+v want %+v", got, in)
	}
}

func TestEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	_ = Save(Credentials{Endpoint: "https://file/api", Workspace: "wsfile", Token: "tokfile"})
	t.Setenv("HULY_TOKEN", "envtok")
	t.Setenv("HULY_ENDPOINT", "https://env/api")
	t.Setenv("HULY_WORKSPACE", "wsenv")

	got, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.Token != "envtok" || got.Endpoint != "https://env/api" || got.Workspace != "wsenv" {
		t.Fatalf("env override failed: %+v", got)
	}
}
