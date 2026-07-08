package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectAgents(t *testing.T) {
	home := t.TempDir()
	cfg := filepath.Join(home, ".config")

	// Present: claude (~/.claude) and opencode (~/.config/opencode).
	// Absent: codex, cursor, pi.
	mustMkdir(t, filepath.Join(home, ".claude"))
	mustMkdir(t, filepath.Join(cfg, "opencode"))

	agents := DetectAgents(Dirs{Home: home, ConfigHome: cfg})

	byID := map[string]Agent{}
	for _, a := range agents {
		byID[a.ID] = a
	}
	if len(byID) != 5 {
		t.Fatalf("want 5 agents, got %d", len(byID))
	}

	claude := byID["claude"]
	if !claude.Present {
		t.Error("claude should be present")
	}
	if claude.SkillsDir != filepath.Join(home, ".claude", "skills") {
		t.Errorf("claude SkillsDir = %q", claude.SkillsDir)
	}

	oc := byID["opencode"]
	if !oc.Present {
		t.Error("opencode should be present")
	}
	// The macOS fix: opencode resolves under ConfigHome, not UserConfigDir.
	if oc.SkillsDir != filepath.Join(cfg, "opencode", "skills") {
		t.Errorf("opencode SkillsDir = %q (must be under ConfigHome)", oc.SkillsDir)
	}

	if byID["codex"].Present || byID["cursor"].Present || byID["pi"].Present {
		t.Error("codex/cursor/pi should be absent")
	}
	if byID["pi"].RootDir != filepath.Join(home, ".pi", "agent") {
		t.Errorf("pi RootDir = %q", byID["pi"].RootDir)
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}
