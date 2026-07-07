# Design: `huly skills` — embedded agent-skill distributor

**Date:** 2026-07-07
**Status:** Approved, ready for implementation plan

## Goal

Ship AI-agent **skills** inside the `huly` binary and install them into
whatever coding agents the user has on their machine (Claude Code, Codex,
opencode, Cursor, Pi). huly becomes a self-contained skill distributor:
`huly skills` opens a Charmbracelet TUI to browse the embedded catalog and
install/update/uninstall skills per detected agent. Because every one of
these agents consumes the **same native `SKILL.md` directory format**,
"cross-agent compatibility" is just copying the same skill dir into each
agent's skills path — no per-agent conversion.

The seed skill teaches an agent to use huly-cli itself for issue tracking.

## Scope

In scope:

1. Embed skill directories in the binary via `//go:embed`, under
   `src/huly/internal/skills/assets/<name>/`.
2. Author one seed skill, **`huly-issue-tracking`**, with `/skill-creator`.
3. Agent detection: probe known home-dir skills paths for the five agents.
4. A `huly skills` Cobra command group:
   - bare `huly skills` → Charmbracelet TUI dashboard.
   - `install [name...]`, `update [name...]`, `uninstall [name...]`,
     `list`.
5. Non-interactive flags (`--agents`, `--all`, `--yes`) with automatic
   fallback when stdout/stdin isn't a TTY.
6. A per-install marker (`.huly-skill.json`) so `update` is idempotent and
   won't clobber user-edited skills without `--force`.

Out of scope: authoring more than the one seed skill (the catalog is
designed to grow — dropping a dir in `assets/` ships it next build);
downloading skills from the network (they are embedded); editing agents'
own config files (`opencode.json`, `AGENTS.md`, etc.) — we only write into
each agent's `skills/` directory.

## Background: the SKILL.md format is universal

Verified on this machine (skills installed by Cloudflare's `wrangler`):
`~/.claude/skills/wrangler/SKILL.md`, `~/.codex/skills/wrangler/SKILL.md`,
and `~/.config/opencode/skills/wrangler/SKILL.md` are **byte-identical**,
including their `references/`, `scripts/`, `templates/`, `tests/` subdirs.
A skill is a directory containing a `SKILL.md` (YAML frontmatter with
`name` + `description`, then Markdown body) plus optional supporting files.

Canonical global skills paths and detection:

| Agent       | Skills path                    | Detect by (dir exists)   |
|-------------|--------------------------------|--------------------------|
| Claude Code | `~/.claude/skills/`            | `~/.claude`              |
| Codex       | `~/.codex/skills/`             | `~/.codex`               |
| opencode    | `~/.config/opencode/skills/`  | `~/.config/opencode`     |
| Cursor      | `~/.cursor/skills/`           | `~/.cursor`              |
| Pi          | `~/.pi/agent/skills/`         | `~/.pi/agent`            |

This mirrors what `wrangler` does (detect agents → prompt → copy skill
dirs; `--install-skills` for non-interactive).

## Current State

- No `//go:embed` anywhere in the repo (greenfield for embedding).
- `cmd/*.go`: each command self-registers via its own `init()` calling
  `rootCmd.AddCommand`. Leaf commands use `RunE`. Parent grouping commands
  are one-line `&cobra.Command{Use, Short}`. This is the pattern to follow.
- `cmd/update.go`: self-updates the binary from GitHub/GitLab releases. Its
  atomic download-then-rename and symlink-resolution helpers are a model
  for safe filesystem writes but are not reused directly.
- `internal/` holds `huly/` (API client), `creds/`, `cache/`, `output/`.
  `output.Table()`/`Quiet` is the house rendering style for non-TUI output.
- Config dirs are XDG-aware but skills are **not** written under huly's own
  config — they go into each agent's directory as listed above.
- `charmbracelet/huh` is being introduced by the concurrent
  `login --otp` TUI work (`docs/superpowers/specs/2026-07-07-login-otp-tui-design.md`).
  This design **shares** that dependency; if this lands first it adds it.
- Go 1.25, module root `src/huly/`, module path
  `github.com/kettleofketchup/huly-cli/src/huly`.

## Design

### 1. Embedded catalog (`internal/skills`)

New package `src/huly/internal/skills/`.

```
internal/skills/
├── assets/                      # embedded skill sources (real dirs)
│   └── huly-issue-tracking/
│       ├── SKILL.md
│       └── references/ ...
├── catalog.go                   # //go:embed assets  + catalog parsing
├── detect.go                    # agent detection
├── install.go                   # copy embed→disk, marker, update logic
└── *_test.go
```

```go
//go:embed all:assets
var assetsFS embed.FS
```

Use `all:assets` so dotfiles inside a skill (if any) are embedded too.

```go
type Skill struct {
    Name        string // dir name == frontmatter name
    Description string // from SKILL.md frontmatter
    fsPath      string // path within assetsFS, e.g. "assets/huly-issue-tracking"
}

func Catalog() ([]Skill, error) // walk assets/*, parse each SKILL.md frontmatter
func (s Skill) Get(name string) (Skill, bool)
```

Frontmatter parsing: read `SKILL.md`, split on the first two `---`
fences, `yaml.Unmarshal` the `name`/`description` (yaml.v3 is already a
dep). A catalog-integrity test asserts every embedded skill's dir name
matches its frontmatter `name` and description is non-empty.

### 2. Agent detection (`detect.go`)

```go
type Agent struct {
    ID         string // "claude", "codex", "opencode", "cursor", "pi"
    Label      string // "Claude Code"
    SkillsDir  string // absolute, e.g. /home/u/.claude/skills
    Present    bool   // marker dir for the agent exists
}

func DetectAgents() []Agent // resolves ~ via os.UserHomeDir / UserConfigDir
```

- Claude `~/.claude`, Codex `~/.codex`, Cursor `~/.cursor`,
  Pi `~/.pi/agent` are under `$HOME`.
- opencode uses `os.UserConfigDir()/opencode` (XDG:
  `~/.config/opencode`), matching the verified path.
- `Present` = the agent's root marker dir exists (not the `skills/`
  subdir, which we create on install). All five are always returned; the
  UI shows present ones selectable and absent ones greyed/​skippable.

### 3. Install / update / marker (`install.go`)

**Marker file** written at `<skillsDir>/<name>/.huly-skill.json`:

```json
{ "skill": "huly-issue-tracking", "huly_version": "0.1.3",
  "content_hash": "sha256:…", "installed_at": "2026-07-07T18:04:05Z" }
```

`content_hash` is a stable hash over the embedded skill's file tree
(sorted relative paths + contents). It is what huly *wrote*, used to tell
huly-managed installs from user-edited ones. The hash **excludes**
`.huly-skill.json` itself, so the same function computes both the embedded
hash (no marker present) and the on-disk hash (marker skipped) and they
compare directly.

```go
func Install(sk Skill, agent Agent, opts InstallOpts) (InstallResult, error)
```

Algorithm for one (skill, agent):

1. `dest = <agent.SkillsDir>/<sk.Name>`; compute embedded `hash`.
2. If `dest` absent → copy tree, write marker → `Installed`.
3. If `dest` present with a valid marker:
   - marker hash == embedded hash → `UpToDate` (skip).
   - marker hash != embedded hash but on-disk tree still matches the
     marker hash (huly-managed, just an older version) → replace tree,
     rewrite marker → `Updated`.
   - on-disk tree hash != marker hash → user-modified: skip with
     `Conflict` unless `opts.Force`, then replace.
4. If `dest` present with **no** marker (installed by another tool / hand)
   → `Conflict`, skip unless `--force`.

Copy is write-to-temp-dir-then-rename within the same `skills/` parent to
stay atomic-ish and on one filesystem (same lesson as `update.go`).
`uninstall` removes `<dest>` only if it carries a huly marker (never
deletes foreign skills) unless `--force`.

### 4. Command surface (`cmd/skills.go`)

Follows the repo's self-registering `init()` pattern.

```
huly skills                       # → TUI dashboard (interactive)
huly skills list                  # plain table: skill × agent install status
huly skills install [name...]     # select agents (TUI) → install
huly skills update  [name...]     # refresh managed installs
huly skills uninstall [name...]   # remove managed installs
```

Flags (on install/update/uninstall):

- `--agents claude,codex,opencode` — explicit target list (skips the
  picker).
- `--all` — every *detected* agent.
- `--yes` / `-y` — no confirmation.
- `--force` — override conflict/user-edited guard.

Interactivity rule (same test as the login TUI): interactive iff stdin
**and** stdout are TTYs (`term.IsTerminal`) and neither `--agents`,
`--all`, nor a non-TTY forces batch mode. Non-interactive with no agent
selector specified → error listing detected agents (never silently guess).

`skills list` uses `output.Table()` for pipe-safe output and honors
`--output json`.

### 5. Charmbracelet TUI

Two entry points, both `charmbracelet/huh` (shared dep), kept in
`cmd/skills_tui.go` so `skills.go` stays declarative:

- **`huly skills install` picker** — a `huh.MultiSelect[string]` of
  detected agents, pre-checked, titled with the skill(s) being installed;
  returns the chosen agent IDs. Absent agents are omitted.
- **`huly skills` dashboard** — a `huh` form sequence: (1) MultiSelect the
  skills from the catalog (with descriptions as option help), (2)
  MultiSelect the target agents, (3) choose action
  (install/update/uninstall) via a Select, then a results summary printed
  after the form. A full `bubbletea.Model` is only introduced if the form
  sequence proves too limiting; start with `huh`.

The install engine (`internal/skills`) is UI-agnostic; the TUI only
collects `(skills, agents, action)` and calls it, printing a per-target
result line (`✓ installed`, `= up to date`, `! conflict (use --force)`).

### 6. Seed skill: `huly-issue-tracking`

Authored via `/skill-creator` into
`internal/skills/assets/huly-issue-tracking/`. Frontmatter description
triggers on issue/bug tracking in a repo. Body teaches an agent to:

- Track bugs and issues for **one** huly project (resolve/confirm the
  project once, then log against it).
- Create and **enable components** to group parts of the codebase, and
  file issues against the right component.
- Use the concrete `huly` commands: `huly project list`,
  `huly component list/create`, `huly issue create/list/update/view`,
  referencing real flags from `cmd/`.

Content authored during implementation (skill-creator step), not in this
spec.

## Data Flow

```
huly skills install
      │  Catalog() (embed)         DetectAgents()
      ▼                                  ▼
  pick skills ─────────────▶  huh MultiSelect(detected agents)
      │                                  │  (or --agents/--all/--yes)
      ▼                                  ▼
  for each (skill, agent):  Install(skill, agent, opts)
      │   embed tree ─hash→ compare marker ─▶ copy/skip/conflict
      ▼
  write <skillsDir>/<name>/ + .huly-skill.json
      ▼
  per-target result summary

Refresh path:  huly update (new binary w/ newer embedded skills)
               → huly skills update  (marker hash differs → replace)
```

## Error Handling

- No agents detected: friendly message naming the supported agents and
  their paths; exit non-error for `list`, error for `install`.
- Non-interactive without `--agents`/`--all`: error listing detected
  agents.
- Unknown skill name arg: error listing catalog names.
- Conflict (foreign or user-edited skill dir): skip that target, report
  `! conflict`, suggest `--force`; never overwrite silently.
- Copy/rename/permission failure: wrapped `%w`, that target fails but
  others still proceed; command exits non-zero if any target errored.
- Embedded catalog empty/corrupt: caught by the integrity test at build/CI
  time, not at runtime.
- TUI cancelled (Ctrl-C/Esc): clean "cancelled" message, nothing written.

## Testing

- **Catalog integrity:** every embedded skill parses; dir name ==
  frontmatter `name`; description non-empty. Guards the embed at CI time.
- **Detection:** point `HOME`/`XDG_CONFIG_HOME` at a `t.TempDir()`, create
  subsets of agent dirs, assert `DetectAgents()` `Present` flags and
  resolved `SkillsDir` paths.
- **Install/update/marker:** into `t.TempDir()` skills dirs —
  fresh install writes tree + marker; second install → `UpToDate`;
  bump embedded hash → `Updated`; hand-edit a file → `Conflict` without
  `--force`, replaced with `--force`; foreign dir (no marker) → `Conflict`.
- **Uninstall:** removes managed dir; refuses foreign dir without
  `--force`.
- **Non-interactive flag paths:** `--agents`, `--all`, `--yes` resolve the
  right target set without a TTY; `--output json` for `list`.
- huh forms themselves aren't unit-tested (interactive TTY); the engine
  and selector-resolution logic around them are.

## YAGNI / cut lines

If scope needs trimming, in order: drop the full `huly skills` dashboard
and keep only `install`/`list`/`update` + the agent picker (the dashboard
is the most UI for the least logic); drop `uninstall` (users can `rm`);
drop Cursor/Pi from the detection table (keep Claude/Codex/opencode, the
three the user named). The embed + install engine + one seed skill is the
irreducible core.
