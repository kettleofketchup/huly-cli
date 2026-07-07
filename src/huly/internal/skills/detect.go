package skills

import (
	"os"
	"path/filepath"
)

// Dirs are the base directories agent detection resolves paths against.
// Injecting them keeps detection pure and testable.
type Dirs struct {
	Home       string // $HOME (or platform equivalent)
	ConfigHome string // $XDG_CONFIG_HOME, else $HOME/.config
}

// ResolveDirs reads Dirs from the environment. ConfigHome uses
// $XDG_CONFIG_HOME with a $HOME/.config fallback on ALL platforms — NOT
// os.UserConfigDir, which returns ~/Library/Application Support on macOS
// (where opencode still uses ~/.config/opencode).
func ResolveDirs() (Dirs, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Dirs{}, err
	}
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	return Dirs{Home: home, ConfigHome: cfg}, nil
}

// Agent is one supported coding agent and where its skills live.
type Agent struct {
	ID        string // "claude","codex","opencode","cursor","pi"
	Label     string // human label
	RootDir   string // detection marker dir
	SkillsDir string // <root>/skills
	Present   bool   // RootDir exists
}

type agentSpec struct {
	id, label string
	root      func(Dirs) string
}

// agentSpecs is the static table of supported agents and their root dirs.
var agentSpecs = []agentSpec{
	{"claude", "Claude Code", func(d Dirs) string { return filepath.Join(d.Home, ".claude") }},
	{"codex", "Codex", func(d Dirs) string { return filepath.Join(d.Home, ".codex") }},
	{"opencode", "opencode", func(d Dirs) string { return filepath.Join(d.ConfigHome, "opencode") }},
	{"cursor", "Cursor", func(d Dirs) string { return filepath.Join(d.Home, ".cursor") }},
	{"pi", "Pi", func(d Dirs) string { return filepath.Join(d.Home, ".pi", "agent") }},
}

// DetectAgents returns all supported agents, each flagged Present if its root
// dir exists, with SkillsDir = <root>/skills.
func DetectAgents(d Dirs) []Agent {
	agents := make([]Agent, 0, len(agentSpecs))
	for _, s := range agentSpecs {
		root := s.root(d)
		present := false
		if fi, err := os.Stat(root); err == nil && fi.IsDir() {
			present = true
		}
		agents = append(agents, Agent{
			ID:        s.id,
			Label:     s.label,
			RootDir:   root,
			SkillsDir: filepath.Join(root, "skills"),
			Present:   present,
		})
	}
	return agents
}

// Detect resolves Dirs from the environment and detects agents.
func Detect() ([]Agent, error) {
	d, err := ResolveDirs()
	if err != nil {
		return nil, err
	}
	return DetectAgents(d), nil
}
