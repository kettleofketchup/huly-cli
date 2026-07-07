# huly skills — Phase A2 (engine) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the UI-free engine that detects installed coding agents and installs/updates/uninstalls embedded skills into them, with a content-hash-gated, conflict-safe state machine.

**Architecture:** Extends the existing leaf package `src/huly/internal/skills` (built in Phase A1) with `detect.go` (agent detection, injectable dirs) and the install engine (`install_fs.go` tree-copy plumbing + `install.go` state machine). No Cobra, no charmbracelet — everything is unit-testable with `go test` against `t.TempDir()`. Phase B wraps this in `huly skills` Cobra commands; Phase C adds the TUI. Spec: `docs/superpowers/specs/2026-07-07-huly-skills-distribution-design.md` (§3 detection, §4 engine).

**Tech Stack:** Go 1.25 standard library (`os`, `io/fs`, `path/filepath`, `crypto/sha256` via A1, `time`). Reuses A1's `gopkg.in/yaml.v3`-backed frontmatter code.

## Global Constraints

- Module path `github.com/kettleofketchup/huly-cli/src/huly`; module root `src/huly/`. New code in package `skills` at `src/huly/internal/skills/`.
- **No Cobra and no charmbracelet imports** in `internal/skills` (Phase A2 stays UI-free).
- **A1 interfaces this phase consumes (all in-package `skills`):** `Skill{Name, Description, fsPath}`, `Catalog()`, `Get(name string) (Skill, bool)`, `assetsFS embed.FS`, `Parse(src []byte) (Frontmatter, error)`, `Frontmatter{Name, Description, ManagedBy, Version, ContentHash}`, `Stamp(src []byte, managedBy, version, contentHash string) ([]byte, error)`, `ContentHash(tree fs.FS) (string, error)`, `(s Skill) contentHash() (string, error)`.
- **opencode path is resolved from ConfigHome, never `os.UserConfigDir()`** (which is `~/Library/Application Support` on macOS). ConfigHome = `$XDG_CONFIG_HOME` else `$HOME/.config`.
- **Detection is injectable:** `DetectAgents(Dirs)` is pure; `Detect()` resolves `Dirs` from the environment. Tests drive `DetectAgents` with a `t.TempDir()`-based `Dirs`, never mutating global env.
- **Provenance in frontmatter:** installed `SKILL.md` carries `metadata.managed_by: "huly-cli"`, `metadata.huly_cli_version`, `metadata.content_hash` (values quoted — A1's `Stamp` does this).
- **The upgrade gate is the content hash, never the version.** Version is provenance/display only. A skill is "ours" iff its frontmatter `managed_by == "huly-cli"`.
- **Safe default:** never delete or overwrite a directory huly cannot prove it owns without `Force`. `Force` backs up (`<dest>.bak-<unixtime>`) before overwriting.
- **Directory replacement is not a bare rename:** write to a temp dir under the same parent (same filesystem), then `RemoveAll(dest)` + `Rename`. Sweep orphaned `.*.new-*` temp dirs before writing. Crash-recoverable, not atomic.
- TDD; commit per task; commit style `feat(skills): …` / `refactor(skills): …`; no Claude watermark/co-author lines.
- Test commands: full suite `just go::test huly` or `cd src/huly && go test ./...`; targeted `cd src/huly && go test ./internal/skills/ -run <Name> -v` (raw `go test` is allow-listed in this repo).

## File Structure

- `src/huly/internal/skills/detect.go` — `Dirs`, `ResolveDirs`, `Agent`, `DetectAgents`, `Detect`.
- `src/huly/internal/skills/detect_test.go`
- `src/huly/internal/skills/install_fs.go` — `writeTree` (embed→disk copy + stamp + atomic swap), `sweepStale`.
- `src/huly/internal/skills/install_fs_test.go`
- `src/huly/internal/skills/install.go` — `Status`, `Result`, `InstallOpts`, `Install`, `Update`, `Uninstall`, and the internal `apply`/`conflictOrForce`/`finish`/`backup`/`restampVersion` helpers.
- `src/huly/internal/skills/install_test.go`

---

### Task 1: Agent detection

**Files:**
- Create: `src/huly/internal/skills/detect.go`
- Create: `src/huly/internal/skills/detect_test.go`

**Interfaces:**
- Consumes: nothing from A1.
- Produces: `type Dirs struct { Home, ConfigHome string }`; `func ResolveDirs() (Dirs, error)`; `type Agent struct { ID, Label, RootDir, SkillsDir string; Present bool }`; `func DetectAgents(d Dirs) []Agent`; `func Detect() ([]Agent, error)`.

- [ ] **Step 1: Write the failing test**

Create `src/huly/internal/skills/detect_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./internal/skills/ -run TestDetectAgents -v`
Expected: FAIL — `DetectAgents`/`Dirs`/`Agent` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `src/huly/internal/skills/detect.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/huly && go test ./internal/skills/ -run TestDetectAgents -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/huly/internal/skills/detect.go src/huly/internal/skills/detect_test.go
git commit -m "feat(skills): agent detection with injectable dirs"
```

---

### Task 2: Tree copy + stamp (`writeTree`, `sweepStale`)

**Files:**
- Create: `src/huly/internal/skills/install_fs.go`
- Create: `src/huly/internal/skills/install_fs_test.go`

**Interfaces:**
- Consumes: `Skill.fsPath`, `assetsFS`, `(s Skill) contentHash()`, `Stamp`, `ContentHash`, `Parse` (all A1, in-package).
- Produces: `func writeTree(sk Skill, dest, version string) error` — copies the embedded skill subtree to `dest`, stamping the root `SKILL.md` with `managed_by`/`huly_cli_version=version`/`content_hash=<embedded hash>`, via a temp dir + atomic swap; `func sweepStale(parent string)` — removes orphaned `.*.new-*` temp dirs.

- [ ] **Step 1: Write the failing test**

Create `src/huly/internal/skills/install_fs_test.go`:

```go
package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTreeStampsAndCopies(t *testing.T) {
	sk, ok := Get("huly-issue-tracking")
	if !ok {
		t.Fatal("seed skill missing")
	}
	dest := filepath.Join(t.TempDir(), "skills", sk.Name)

	if err := writeTree(sk, dest, "0.2.0"); err != nil {
		t.Fatal(err)
	}

	// SKILL.md present and stamped with all three metadata keys.
	raw, err := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	fm, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if fm.ManagedBy != "huly-cli" || fm.Version != "0.2.0" || fm.ContentHash == "" {
		t.Errorf("SKILL.md not fully stamped: %+v", fm)
	}

	// The on-disk content hash equals the embedded hash (frontmatter excluded),
	// i.e. a fresh install is NOT misread as modified.
	emb, err := sk.contentHash()
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := ContentHash(os.DirFS(dest))
	if err != nil {
		t.Fatal(err)
	}
	if onDisk != emb {
		t.Errorf("fresh install hash %s != embedded %s", onDisk, emb)
	}
	if fm.ContentHash != emb {
		t.Errorf("stamped content_hash %s != embedded %s", fm.ContentHash, emb)
	}
}

func TestWriteTreeReplacesPopulatedDest(t *testing.T) {
	sk, _ := Get("huly-issue-tracking")
	dest := filepath.Join(t.TempDir(), "skills", sk.Name)

	// Pre-populate dest with junk so the swap must replace a non-empty dir
	// (regression for the os.Rename-over-non-empty-dir bug).
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "stale.txt"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := writeTree(sk, dest, "0.2.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "stale.txt")); !os.IsNotExist(err) {
		t.Error("stale file survived replacement")
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Error("SKILL.md missing after replacement")
	}
}

func TestSweepStale(t *testing.T) {
	parent := t.TempDir()
	orphan := filepath.Join(parent, ".huly-issue-tracking.new-XXXX")
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(parent, "real-skill")
	if err := os.MkdirAll(keep, 0o755); err != nil {
		t.Fatal(err)
	}
	sweepStale(parent)
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("orphan .new- dir not swept")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("real skill dir wrongly swept")
	}
	// Sanity: the orphan name matches the pattern we sweep.
	if !strings.Contains(filepath.Base(orphan), ".new-") {
		t.Fatal("test fixture wrong")
	}
}

// Integration: writeTree must sweep a prior crash's orphan .new- dir and still
// succeed (the crash-recovery path end to end, not sweepStale in isolation).
func TestWriteTreeSweepsOrphanBeforeWriting(t *testing.T) {
	sk, _ := Get("huly-issue-tracking")
	parent := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(parent, sk.Name)
	orphan := filepath.Join(parent, "."+sk.Name+".new-DEAD")
	if err := os.MkdirAll(orphan, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(orphan, "leftover.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeTree(sk, dest, "0.2.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Error("orphaned .new- dir from a prior crash was not swept")
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Error("write incomplete with a pre-existing orphan present")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./internal/skills/ -run 'TestWriteTree|TestSweepStale' -v`
Expected: FAIL — `writeTree`/`sweepStale` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `src/huly/internal/skills/install_fs.go`:

```go
package skills

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// writeTree copies the embedded subtree for sk to dest, stamping the root
// SKILL.md with managed_by/huly_cli_version=version/content_hash=<embedded>.
// It writes into a temp dir under dest's parent (same filesystem), then
// RemoveAll(dest)+Rename to swap it in. Crash-recoverable, not atomic:
// os.Rename cannot replace a non-empty directory, hence the RemoveAll.
func writeTree(sk Skill, dest, version string) error {
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", parent, err)
	}
	sweepStale(parent)

	embHash, err := sk.contentHash()
	if err != nil {
		return err
	}
	sub, err := fs.Sub(assetsFS, sk.fsPath)
	if err != nil {
		return err
	}

	tmp, err := os.MkdirTemp(parent, "."+filepath.Base(dest)+".new-*")
	if err != nil {
		return fmt.Errorf("mkdtemp: %w", err)
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(tmp)
		}
	}()

	err = fs.WalkDir(sub, ".", func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		target := filepath.Join(tmp, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, rerr := fs.ReadFile(sub, p)
		if rerr != nil {
			return rerr
		}
		if p == "SKILL.md" {
			raw, rerr = Stamp(raw, "huly-cli", version, embHash)
			if rerr != nil {
				return rerr
			}
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		return fmt.Errorf("copy tree: %w", err)
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("remove old %s: %w", dest, err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("rename into place: %w", err)
	}
	success = true
	return nil
}

// sweepStale removes orphaned ".<name>.new-*" temp dirs left by a crashed
// writeTree. Called before creating a fresh temp dir, so it never removes the
// in-flight one.
func sweepStale(parent string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, e := range entries {
		n := e.Name()
		if strings.HasPrefix(n, ".") && strings.Contains(n, ".new-") {
			_ = os.RemoveAll(filepath.Join(parent, n))
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/huly && go test ./internal/skills/ -run 'TestWriteTree|TestSweepStale' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/huly/internal/skills/install_fs.go src/huly/internal/skills/install_fs_test.go
git commit -m "feat(skills): writeTree copy+stamp with atomic-ish dir swap"
```

---

### Task 3: Install / Update state machine

**Files:**
- Create: `src/huly/internal/skills/install.go`
- Create: `src/huly/internal/skills/install_test.go`

**Interfaces:**
- Consumes: `Agent` (Task 1), `writeTree` (Task 2), `Skill`, `Parse`, `ContentHash`, `(s Skill) contentHash()`, `Stamp` (A1).
- Produces:
  - `type Status string` with constants `StatusInstalled`, `StatusUpdated`, `StatusRepaired`, `StatusUpToDate`, `StatusConflict`, `StatusRemoved`, `StatusSkipped` (values `"installed"`,`"updated"`,`"repaired"`,`"up-to-date"`,`"conflict"`,`"removed"`,`"skipped"`).
  - `type Result struct { Skill, Agent, Path string; Status Status; Reason string }`
  - `type InstallOpts struct { CurrentVersion string; Force, DryRun bool }`
  - `func Install(sk Skill, ag Agent, o InstallOpts) (Result, error)`
  - `func Update(sk Skill, ag Agent, o InstallOpts) (Result, error)`

- [ ] **Step 1: Write the failing test**

Create `src/huly/internal/skills/install_test.go`:

```go
package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// agentAt returns an Agent whose SkillsDir is a fresh temp dir.
func agentAt(t *testing.T) Agent {
	t.Helper()
	return Agent{ID: "test", Label: "Test", RootDir: t.TempDir(), SkillsDir: filepath.Join(t.TempDir(), "skills"), Present: true}
}

func seed(t *testing.T) Skill {
	t.Helper()
	sk, ok := Get("huly-issue-tracking")
	if !ok {
		t.Fatal("seed skill missing")
	}
	return sk
}

func TestInstallFreshThenIdempotent(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	o := InstallOpts{CurrentVersion: "0.2.0"}

	r, err := Install(sk, ag, o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusInstalled {
		t.Fatalf("first install status = %q", r.Status)
	}

	// Second install of the same shipped version is idempotent: up-to-date,
	// no re-copy.
	r2, err := Install(sk, ag, o)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Status != StatusUpToDate {
		t.Errorf("second install status = %q, want up-to-date", r2.Status)
	}
}

func TestUpdateAbsentIsSkipped(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	r, err := Update(sk, ag, InstallOpts{CurrentVersion: "0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusSkipped || r.Reason != "absent" {
		t.Errorf("update-absent = %q/%q, want skipped/absent", r.Status, r.Reason)
	}
}

func TestUserEditedIsConflictThenForce(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	o := InstallOpts{CurrentVersion: "0.2.0"}
	if _, err := Install(sk, ag, o); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(ag.SkillsDir, sk.Name)

	// Edit the installed body -> on-disk hash diverges from stored.
	md := filepath.Join(dest, "SKILL.md")
	raw, _ := os.ReadFile(md)
	if err := os.WriteFile(md, append(raw, []byte("\nuser edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Update(sk, ag, o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusConflict || r.Reason != "modified" {
		t.Fatalf("edited update = %q/%q, want conflict/modified", r.Status, r.Reason)
	}

	// --force backs up and overwrites.
	rf, err := Update(sk, ag, InstallOpts{CurrentVersion: "0.2.0", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if rf.Status != StatusUpdated {
		t.Errorf("forced update = %q, want updated", rf.Status)
	}
	baks, _ := filepath.Glob(dest + ".bak-*")
	if len(baks) == 0 {
		t.Error("force did not back up the user-edited dir")
	}
}

func TestForeignIsConflict(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	// A foreign skill: SKILL.md without our managed_by marker.
	if err := os.WriteFile(filepath.Join(dest, "SKILL.md"),
		[]byte("---\nname: huly-issue-tracking\n---\nsomeone else's\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Install(sk, ag, InstallOpts{CurrentVersion: "0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusConflict || r.Reason != "foreign" {
		t.Errorf("foreign = %q/%q, want conflict/foreign", r.Status, r.Reason)
	}
}

func TestUnreadableIsConflictNotClobbered(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	// dest exists but no SKILL.md -> cannot prove ownership -> conflict.
	if err := os.WriteFile(filepath.Join(dest, "other.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Install(sk, ag, InstallOpts{CurrentVersion: "0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusConflict || r.Reason != "unreadable" {
		t.Errorf("unreadable = %q/%q, want conflict/unreadable", r.Status, r.Reason)
	}
	// The foreign file must survive (not clobbered).
	if _, err := os.Stat(filepath.Join(dest, "other.txt")); err != nil {
		t.Error("unowned dir was clobbered without --force")
	}
}

func TestShippedContentChangedUpdates(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	o := InstallOpts{CurrentVersion: "0.2.0"}
	if _, err := Install(sk, ag, o); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	md := filepath.Join(dest, "SKILL.md")

	// Simulate an OLDER shipped hash by rewriting the stored content_hash to a
	// value that differs from the embedded one, WITHOUT touching the body
	// (so on-disk body hash still matches this stored value's basis is the
	// point of contention). To keep the body/stored consistent we instead
	// re-stamp a bogus stored hash and also make the on-disk body match it:
	// simplest reliable check — set stored hash to embedded is up-to-date;
	// to force "updated" we edit the body AND re-stamp so on-disk==stored but
	// embedded!=stored.
	raw, _ := os.ReadFile(md)
	// Append a body line, then re-stamp content_hash to the NEW on-disk hash
	// so the skill reads as "ours, unmodified, but shipped differs".
	edited := append(raw, []byte("\nshipped-delta\n")...)
	if err := os.WriteFile(md, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	newOnDisk, err := ContentHash(os.DirFS(dest))
	if err != nil {
		t.Fatal(err)
	}
	restamped, err := Stamp(edited, "huly-cli", "0.2.0", newOnDisk)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(md, restamped, 0o644); err != nil {
		t.Fatal(err)
	}

	// Now: ours, on-disk == stored (both newOnDisk), embedded != stored -> updated.
	r, err := Update(sk, ag, InstallOpts{CurrentVersion: "0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusUpdated {
		t.Errorf("shipped-changed update = %q, want updated", r.Status)
	}
	// After update the on-disk hash equals the embedded hash again.
	emb, _ := sk.contentHash()
	after, _ := ContentHash(os.DirFS(dest))
	if after != emb {
		t.Errorf("post-update hash %s != embedded %s", after, emb)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	r, err := Install(sk, ag, InstallOpts{CurrentVersion: "0.2.0", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusInstalled {
		t.Errorf("dry-run status = %q, want installed (predicted)", r.Status)
	}
	if _, err := os.Stat(filepath.Join(ag.SkillsDir, sk.Name)); !os.IsNotExist(err) {
		t.Error("dry-run wrote to disk")
	}
}

func TestUpToDateRestampsVersionHashNeutral(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	if _, err := Install(sk, ag, InstallOpts{CurrentVersion: "0.2.0"}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(ag.SkillsDir, sk.Name)

	// An unchanged skill under a newer binary: up-to-date, but provenance
	// version re-stamped to 0.3.0 WITHOUT moving the content hash.
	r, err := Update(sk, ag, InstallOpts{CurrentVersion: "0.3.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusUpToDate {
		t.Fatalf("version-bump update = %q, want up-to-date", r.Status)
	}
	raw, _ := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	fm, _ := Parse(raw)
	if fm.Version != "0.3.0" {
		t.Errorf("stamped version = %q, want 0.3.0", fm.Version)
	}
	// Hash-neutral proof: a third update at 0.3.0 is still up-to-date (would be
	// "modified" if the re-stamp had perturbed the hashed body).
	r3, err := Update(sk, ag, InstallOpts{CurrentVersion: "0.3.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r3.Status != StatusUpToDate {
		t.Fatalf("third update = %q, want up-to-date (no churn)", r3.Status)
	}
}

func TestAdoptedMissingContentHash(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	o := InstallOpts{CurrentVersion: "0.2.0"}
	if _, err := Install(sk, ag, o); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	md := filepath.Join(dest, "SKILL.md")

	// Simulate an older huly that stamped managed_by/version but no content_hash,
	// with the body still matching the shipped skill.
	raw, _ := os.ReadFile(md)
	stripped, err := Stamp(raw, "huly-cli", "0.2.0", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(md, stripped, 0o644); err != nil {
		t.Fatal(err)
	}
	if fm, _ := Parse(stripped); fm.ContentHash != "" {
		t.Fatalf("fixture: content_hash = %q, want empty", fm.ContentHash)
	}

	r, err := Update(sk, ag, o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusUpdated || r.Reason != "adopted" {
		t.Fatalf("adopt = %q/%q, want updated/adopted", r.Status, r.Reason)
	}
	emb, _ := sk.contentHash()
	after, _ := Parse(mustRead(t, md))
	if after.ContentHash != emb {
		t.Errorf("adopted content_hash = %q, want %q", after.ContentHash, emb)
	}
	onDisk, _ := ContentHash(os.DirFS(dest))
	if onDisk != emb {
		t.Errorf("adopted tree hash %s != embedded %s", onDisk, emb)
	}
}

// Pre-hash install whose body the user edited: adopt must NOT clobber it — the
// state is ambiguous (old shipped content vs a user edit), so it is a conflict.
func TestAdoptedEditedBodyIsConflict(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	o := InstallOpts{CurrentVersion: "0.2.0"}
	if _, err := Install(sk, ag, o); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	md := filepath.Join(dest, "SKILL.md")
	raw, _ := os.ReadFile(md)
	stripped, err := Stamp(raw, "huly-cli", "0.2.0", "") // pre-hash
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(md, append(stripped, []byte("\nuser edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Update(sk, ag, o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusConflict || r.Reason != "modified" {
		t.Fatalf("adopt-edited = %q/%q, want conflict/modified", r.Status, r.Reason)
	}
	if !strings.Contains(string(mustRead(t, md)), "user edit") {
		t.Error("adopt-edited clobbered the user's edit without --force")
	}
}

func TestUnreadableForceRepairs(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "other.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Install(sk, ag, InstallOpts{CurrentVersion: "0.2.0", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusRepaired || r.Reason != "unreadable" {
		t.Fatalf("forced unreadable = %q/%q, want repaired/unreadable", r.Status, r.Reason)
	}
	if baks, _ := filepath.Glob(dest + ".bak-*"); len(baks) == 0 {
		t.Error("force did not back up the unreadable dir")
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Error("SKILL.md missing after forced repair")
	}
}

func TestForeignForceRepairs(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "SKILL.md"),
		[]byte("---\nname: huly-issue-tracking\n---\nsomeone else's\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Install(sk, ag, InstallOpts{CurrentVersion: "0.2.0", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusRepaired || r.Reason != "foreign" {
		t.Fatalf("forced foreign = %q/%q, want repaired/foreign", r.Status, r.Reason)
	}
	baks, _ := filepath.Glob(dest + ".bak-*")
	if len(baks) == 0 {
		t.Fatal("force did not back up the foreign dir")
	}
	bakRaw, err := os.ReadFile(filepath.Join(baks[0], "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bakRaw), "someone else's") {
		t.Error("backup does not preserve the foreign content")
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./internal/skills/ -run 'TestInstall|TestUpdate|TestUser|TestForeign|TestUnreadable|TestShipped|TestDryRun|TestUpToDate|TestAdopted' -v`
Expected: FAIL — `Install`/`Update`/`Status*`/`Result`/`InstallOpts` undefined.

- [ ] **Step 3: Write minimal implementation**

Create `src/huly/internal/skills/install.go`:

```go
package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Status is the outcome of an install/update/uninstall on one (skill, agent).
type Status string

const (
	StatusInstalled Status = "installed"
	StatusUpdated   Status = "updated"
	StatusRepaired  Status = "repaired"
	StatusUpToDate  Status = "up-to-date"
	StatusConflict  Status = "conflict"
	StatusRemoved   Status = "removed"
	StatusSkipped   Status = "skipped"
)

// Result reports what happened for one (skill, agent) target.
type Result struct {
	Skill  string
	Agent  string
	Path   string
	Status Status
	Reason string // "foreign","unreadable","modified","adopted","absent", ""
}

// InstallOpts controls install/update/uninstall behavior.
type InstallOpts struct {
	CurrentVersion string // stamped as provenance; from version.Version at the CLI layer
	Force          bool   // override conflict guards (backs up first)
	DryRun         bool   // classify + report, write nothing
}

// Install ensures the shipped skill is present and current for the agent.
// A fresh dest is installed; an up-to-date one is left alone (idempotent).
// Force overrides the conflict guards (backing up first); on an already
// up-to-date tree it is a no-op, since up-to-date proves the body+siblings are
// byte-identical to the embedded tree.
func Install(sk Skill, ag Agent, o InstallOpts) (Result, error) {
	return apply(sk, ag, o, false)
}

// Update refreshes an already-installed, huly-owned skill that is behind.
// An absent skill is skipped (not installed).
func Update(sk Skill, ag Agent, o InstallOpts) (Result, error) {
	return apply(sk, ag, o, true)
}

func apply(sk Skill, ag Agent, o InstallOpts, updateOnly bool) (Result, error) {
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	res := Result{Skill: sk.Name, Agent: ag.ID, Path: dest}

	if _, statErr := os.Stat(dest); os.IsNotExist(statErr) {
		if updateOnly {
			res.Status, res.Reason = StatusSkipped, "absent"
			return res, nil
		}
		return finish(sk, dest, o, StatusInstalled, "", res)
	}

	md := filepath.Join(dest, "SKILL.md")
	raw, readErr := os.ReadFile(md)
	if readErr != nil {
		// dest exists but SKILL.md missing/unreadable -> can't prove ours.
		return conflictOrForce(sk, dest, o, "unreadable", res)
	}
	fm, perr := Parse(raw)
	if perr != nil {
		return conflictOrForce(sk, dest, o, "unreadable", res)
	}
	if fm.ManagedBy != "huly-cli" {
		return conflictOrForce(sk, dest, o, "foreign", res)
	}

	// Ours. Gate on content hash.
	emb, err := sk.contentHash()
	if err != nil {
		return res, err
	}
	onDisk, err := ContentHash(os.DirFS(dest))
	if err != nil {
		return res, err
	}

	if fm.ContentHash == "" {
		// Ours but pre-hash (an older huly stamped no content_hash). Adopt
		// WITHOUT clobbering: if the body already matches the embedded tree,
		// just stamp content_hash (hash-neutral, no copy). If it diverges, the
		// state is ambiguous (old shipped content vs a user edit) -> treat as
		// modified so --force backs up before overwriting.
		if onDisk == emb {
			if !o.DryRun {
				if err := restampVersion(md, raw, o.CurrentVersion, emb); err != nil {
					return res, err
				}
			}
			res.Status, res.Reason = StatusUpdated, "adopted"
			return res, nil
		}
		return conflictOrForce(sk, dest, o, "modified", res)
	}
	if onDisk == fm.ContentHash {
		// Unmodified.
		if emb == fm.ContentHash {
			// Up to date. Refresh provenance version only (hash-neutral).
			if fm.Version != o.CurrentVersion && !o.DryRun {
				if err := restampVersion(md, raw, o.CurrentVersion, fm.ContentHash); err != nil {
					return res, err
				}
			}
			res.Status = StatusUpToDate
			return res, nil
		}
		// Shipped content changed -> update.
		return finish(sk, dest, o, StatusUpdated, "", res)
	}
	// On-disk diverges from stored -> user-edited.
	return conflictOrForce(sk, dest, o, "modified", res)
}

// conflictOrForce reports a conflict, or (with Force) backs up + overwrites.
func conflictOrForce(sk Skill, dest string, o InstallOpts, reason string, res Result) (Result, error) {
	if !o.Force {
		res.Status, res.Reason = StatusConflict, reason
		return res, nil
	}
	if !o.DryRun {
		if err := backup(dest); err != nil {
			return res, err
		}
	}
	status := StatusUpdated
	if reason == "unreadable" || reason == "foreign" {
		status = StatusRepaired
	}
	return finish(sk, dest, o, status, reason, res)
}

// finish writes the tree (unless DryRun) and stamps the result.
func finish(sk Skill, dest string, o InstallOpts, status Status, reason string, res Result) (Result, error) {
	if !o.DryRun {
		if err := writeTree(sk, dest, o.CurrentVersion); err != nil {
			return res, err
		}
	}
	res.Status, res.Reason = status, reason
	return res, nil
}

// backup renames dest aside so it is recoverable after a forced overwrite.
// UnixNano avoids a same-instant collision between two backups of one dest.
func backup(dest string) error {
	bak := fmt.Sprintf("%s.bak-%d", dest, time.Now().UnixNano())
	return os.Rename(dest, bak)
}

// restampVersion rewrites only the SKILL.md to refresh the provenance version,
// keeping the same content_hash (hash-neutral, no tree copy).
func restampVersion(md string, raw []byte, version, hash string) error {
	out, err := Stamp(raw, "huly-cli", version, hash)
	if err != nil {
		return err
	}
	return os.WriteFile(md, out, 0o644)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/huly && go test ./internal/skills/ -run 'TestInstall|TestUpdate|TestUser|TestForeign|TestUnreadable|TestShipped|TestDryRun|TestUpToDate|TestAdopted' -v`
Expected: PASS (all 12 tests: the original 7 plus up-to-date-restamp, adopt×2, and unreadable/foreign force-repair).

- [ ] **Step 5: Run the whole package**

Run: `cd src/huly && go test ./internal/skills/ -v 2>&1 | tail -20`
Expected: all skills tests PASS (A1 + Tasks 1-3).

- [ ] **Step 6: Commit**

```bash
git add src/huly/internal/skills/install.go src/huly/internal/skills/install_test.go
git commit -m "feat(skills): install/update content-hash state machine"
```

---

### Task 4: Uninstall

**Files:**
- Modify: `src/huly/internal/skills/install.go` (append `Uninstall`)
- Modify: `src/huly/internal/skills/install_test.go` (append uninstall tests)

**Interfaces:**
- Consumes: `Agent`, `Result`, `Status`, `InstallOpts`, `Parse` (Task 3 / A1).
- Produces: `func Uninstall(sk Skill, ag Agent, o InstallOpts) (Result, error)` — removes `dest` only if it is huly-owned (→ `removed`); a foreign dir is `conflict`/`foreign` unless `Force`; an absent dir is `skipped`/`absent`.

- [ ] **Step 1: Write the failing test**

Append to `src/huly/internal/skills/install_test.go`:

```go
func TestUninstallOursRemoves(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	o := InstallOpts{CurrentVersion: "0.2.0"}
	if _, err := Install(sk, ag, o); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(ag.SkillsDir, sk.Name)

	r, err := Uninstall(sk, ag, o)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusRemoved {
		t.Fatalf("uninstall status = %q, want removed", r.Status)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("dir still present after uninstall")
	}
}

func TestUninstallForeignRefusedWithoutForce(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "SKILL.md"),
		[]byte("---\nname: huly-issue-tracking\n---\nforeign\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Uninstall(sk, ag, InstallOpts{CurrentVersion: "0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusConflict || r.Reason != "foreign" {
		t.Fatalf("foreign uninstall = %q/%q, want conflict/foreign", r.Status, r.Reason)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Error("foreign dir removed without --force")
	}
	// With --force it is removed FROM dest, but backed up (not destroyed).
	rf, err := Uninstall(sk, ag, InstallOpts{CurrentVersion: "0.2.0", Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if rf.Status != StatusRemoved {
		t.Errorf("forced foreign uninstall = %q, want removed", rf.Status)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Error("dest still present after forced uninstall")
	}
	baks, _ := filepath.Glob(dest + ".bak-*")
	if len(baks) == 0 {
		t.Fatal("forced foreign uninstall destroyed the dir without a backup")
	}
	if bakRaw, err := os.ReadFile(filepath.Join(baks[0], "SKILL.md")); err != nil || !strings.Contains(string(bakRaw), "foreign") {
		t.Error("backup does not preserve the foreign content")
	}
}

func TestUninstallAbsentSkipped(t *testing.T) {
	sk, ag := seed(t), agentAt(t)
	r, err := Uninstall(sk, ag, InstallOpts{CurrentVersion: "0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != StatusSkipped || r.Reason != "absent" {
		t.Errorf("absent uninstall = %q/%q, want skipped/absent", r.Status, r.Reason)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./internal/skills/ -run TestUninstall -v`
Expected: FAIL — `Uninstall` undefined.

- [ ] **Step 3: Write minimal implementation**

Append to `src/huly/internal/skills/install.go`:

```go
// Uninstall removes a skill from an agent, but only one huly owns unless Force.
// A foreign/unreadable dir removed under Force is backed up (never destroyed),
// mirroring install --force and the "never destroy unproven content" rule.
func Uninstall(sk Skill, ag Agent, o InstallOpts) (Result, error) {
	dest := filepath.Join(ag.SkillsDir, sk.Name)
	res := Result{Skill: sk.Name, Agent: ag.ID, Path: dest}

	if _, statErr := os.Stat(dest); os.IsNotExist(statErr) {
		res.Status, res.Reason = StatusSkipped, "absent"
		return res, nil
	}

	ours := false
	reason := "unreadable" // no/unparseable SKILL.md
	if raw, err := os.ReadFile(filepath.Join(dest, "SKILL.md")); err == nil {
		if fm, perr := Parse(raw); perr == nil {
			if fm.ManagedBy == "huly-cli" {
				ours = true
			} else {
				reason = "foreign"
			}
		}
	}
	if !ours && !o.Force {
		res.Status, res.Reason = StatusConflict, reason
		return res, nil
	}
	if !o.DryRun {
		if ours {
			if err := os.RemoveAll(dest); err != nil {
				return res, err
			}
		} else {
			// Force-removing a dir we cannot prove is ours: back it up rather
			// than destroy it.
			if err := backup(dest); err != nil {
				return res, err
			}
		}
	}
	res.Status = StatusRemoved
	if !ours {
		res.Reason = reason
	}
	return res, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd src/huly && go test ./internal/skills/ -run TestUninstall -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Run the whole module suite**

Run: `cd src/huly && go test ./... 2>&1 | tail -15`
Expected: all packages PASS.

- [ ] **Step 6: Commit**

```bash
git add src/huly/internal/skills/install.go src/huly/internal/skills/install_test.go
git commit -m "feat(skills): uninstall (owned-only unless --force)"
```

---

## Self-Review

**Spec coverage (Phase A2 scope):**
- §3 detection with injectable `Dirs`, opencode via `ConfigHome` (macOS fix), all five agents, `Present` flag → Task 1. ✓
- §4 tree copy: temp-dir + `RemoveAll` + `Rename` (no bare rename over non-empty dir), `chmod 0755`, `MkdirAll` parent before temp, orphan `.*.new-*` sweep, stamp on write → Task 2. ✓
- §4 state machine: absent→installed; update-absent→skipped; unreadable/foreign→conflict (unclobbered) / repaired under `--force` with backup (tested: `TestUnreadableForceRepairs`, `TestForeignForceRepairs`); ours+unmodified+same-hash→up-to-date + version-only hash-neutral re-stamp (tested: `TestUpToDateRestampsVersionHashNeutral`); ours+unmodified+diff-hash→updated; ours+edited→conflict/modified / updated+backup under `--force`; missing stored hash→adopt **without clobbering** (stamp-only when body matches, conflict/modified when it diverges — tested: `TestAdoptedMissingContentHash`, `TestAdoptedEditedBodyIsConflict`); dry-run writes nothing → Task 3. ✓
- §4 uninstall: owned→removed; foreign/unreadable→conflict / removed-under-force **with backup, never destroyed**; absent→skipped → Task 4. ✓
- `Result{Skill,Agent,Path,Status,Reason}` gives the CLI (Phase B) the reason to pick a token; `Status` values match the spec's ASCII token set. ✓

Not in this plan (correctly — Phase B/C): Cobra commands, flags, `--output json`, exit codes, completion, docs, TUI. The engine returns structured `Result`s; the CLI maps them to tokens/exit codes in Phase B.

**Placeholder scan:** No TBD/TODO; every step has complete code and an expected result.

**Type consistency:** `Dirs`, `Agent`, `Skill`, `Status`(+constants), `Result`, `InstallOpts`, `writeTree`, `sweepStale`, `Install`, `Update`, `Uninstall`, `apply`, `conflictOrForce`, `finish`, `backup`, `restampVersion` are named identically across tasks. `writeTree(sk, dest, version)` (Task 2) is called by `finish` (Task 3) with `(sk, dest, o.CurrentVersion)`. `Result`/`Status`/`InstallOpts` (Task 3) are reused by `Uninstall` (Task 4). All consume A1's real signatures (`Parse`, `ContentHash`, `(Skill).contentHash`, `Stamp`, `Get`, `assetsFS`, `Skill.fsPath`) as listed in Global Constraints.

**Note on `TestShippedContentChangedUpdates` (Task 3):** it constructs the "ours, unmodified on disk, but shipped differs" state by editing the body, recomputing the on-disk hash, and re-stamping that hash as the stored value — so `onDisk == storedHash != embeddedHash`. This is the only way to exercise the `updated` (non-adopt) branch without a second embedded skill version, and it faithfully reproduces the real "older shipped content" condition.
