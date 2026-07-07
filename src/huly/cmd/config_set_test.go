package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteConfigValuesOnlyWritesGivenKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "huly.yaml")

	if err := writeConfigValues(path, map[string]string{"login.email": "me@corp.com"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := string(b)
	if !strings.Contains(got, "me@corp.com") {
		t.Fatalf("missing value; file=%q", got)
	}
	// Must NOT leak global defaults into the file.
	if strings.Contains(got, "log:") || strings.Contains(got, "output:") {
		t.Fatalf("file bloated with defaults; file=%q", got)
	}
}

func TestWriteConfigValuesPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huly.yaml")
	if err := os.WriteFile(path, []byte("server:\n  url: https://existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeConfigValues(path, map[string]string{"login.workspace": "acme"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	b, _ := os.ReadFile(path)
	got := string(b)
	if !strings.Contains(got, "https://existing") || !strings.Contains(got, "acme") {
		t.Fatalf("expected both values; file=%q", got)
	}
}

func TestWriteConfigValuesRefusesToClobberMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huly.yaml")
	bad := []byte("server:\n  url: \"unterminated\n\tbroken: [1,2\n")
	if err := os.WriteFile(path, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeConfigValues(path, map[string]string{"login.email": "x@y.com"}); err == nil {
		t.Fatal("expected error writing over malformed config, got nil")
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(bad) {
		t.Fatalf("malformed file was overwritten; file=%q", got)
	}
}

func TestConfigSetRejectsUnknownKey(t *testing.T) {
	if validConfigKeys["not.a.key"] {
		t.Fatal("unknown key unexpectedly allowed")
	}
	if !validConfigKeys["login.email"] {
		t.Fatal("login.email should be allowed")
	}
}
