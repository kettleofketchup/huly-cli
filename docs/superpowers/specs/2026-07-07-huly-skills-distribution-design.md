# Design: `huly skills` — embedded agent-skill distributor

**Date:** 2026-07-07
**Status:** Approved (revised after 5-agent review), ready for implementation plan

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
2. Author one seed skill, **`huly-issue-tracking`**, with `/skill-creator`,
   pinned by checkable acceptance criteria (§6).
3. Agent detection with an injectable directory root (testable, correct on
   Linux/macOS/Windows).
4. An install/update/uninstall **engine** in `internal/skills` that stamps
   provenance into frontmatter and gates upgrades on a content hash.
5. A `huly skills` Cobra command group: `list`, `install`, `update`,
   `uninstall`, plus a bare-command TUI dashboard.
6. Non-interactive flags (`--agents`, `--all`, `--yes`, `--force`,
   `--dry-run`, `--no-interactive`) with greppable/JSON output and defined
   exit codes.
7. A charmbracelet/huh TUI (agent picker + dashboard).
8. Docs page + nav entry and shell completion, per repo convention.

Delivered in **three phases** (see Phasing) so each is independently
shippable and testable.

Out of scope: authoring more than the one seed skill (the catalog grows by
dropping a dir in `assets/`); downloading skills from the network (they are
embedded); editing agents' own config files (`opencode.json`, `AGENTS.md`);
project-local install scope (global home dirs only for now — but the engine
takes an explicit target dir so a `--scope project` can be added later
without reshaping it).

## Background: the SKILL.md format is universal

Verified on this machine (skills installed by Cloudflare's `wrangler`):
`~/.claude/skills/wrangler/SKILL.md`, `~/.codex/skills/wrangler/SKILL.md`,
`~/.config/opencode/skills/wrangler/SKILL.md`, and
`~/.pi/agent/skills/wrangler/SKILL.md` are **byte-identical** (md5
confirmed), including their `references/`, `scripts/`, `templates/`,
`tests/` subdirs. A skill is a directory containing a `SKILL.md` (YAML
frontmatter with `name` + `description`, then Markdown body) plus optional
supporting files.

Canonical global skills paths and detection (all five confirmed against
`github.com/cloudflare/skills`):

| Agent       | Skills path                    | Detect by (dir exists)   |
|-------------|--------------------------------|--------------------------|
| Claude Code | `~/.claude/skills/`            | `~/.claude`              |
| Codex       | `~/.codex/skills/`             | `~/.codex`               |
| opencode    | `~/.config/opencode/skills/`  | `~/.config/opencode`     |
| Cursor      | `~/.cursor/skills/`           | `~/.cursor`              |
| Pi          | `~/.pi/agent/skills/`         | `~/.pi/agent`            |

**`metadata` tolerance:** `metadata` is an optional field in the open Agent
Skills spec (agentskills.io) — a *map from string keys to string values*.
Verified tolerant: **Cursor** (docs list `metadata` explicitly) and
**Claude Code** (on-disk skills carry nested-map frontmatter like `hooks:`,
`gbrain:` and load fine). **Codex** and **Pi** document only
`name`/`description` and are silent on unknown keys — residual **low** risk
they are stricter. Mitigation: install is best-effort per target; a target
whose parser ever rejects the frontmatter fails *that target only*, never
aborts the run. This mirrors what `wrangler` does (detect → prompt → copy
skill dirs; `--install-skills` for non-interactive).

## Current State

- No `//go:embed` anywhere in the repo (greenfield for embedding).
- `cmd/*.go`: each command self-registers via its own `init()` calling
  `rootCmd.AddCommand`. Leaf commands use `RunE`. Parent grouping commands
  are one-line `&cobra.Command{Use, Short}` **with no `RunE`**, so bare
  `huly issue` / `huly config` print help (Cobra default). `--output` is a
  global viper-bound flag (`cmd/root.go`).
- `cmd/update.go`: self-updates the binary from GitHub/GitLab releases.
  `compareVersions`/`parseVersion` (L364–412) are the semver helpers to
  reuse; note `runUpdate` **`strings.TrimPrefix(v, "v")` before comparing**
  (L80–81) — required because `version.Version` is `v`-prefixed.
- `version/version.go`: `Version` defaults to `"dev"`, set by ldflags to
  `git describe --tags --always` (`just/go.just:5`) → values like `v0.1.2`
  or `v0.1.2-3-gSHA`.
- `internal/creds/creds.go`: precedent for XDG resolution + `t.Setenv`
  override in tests.
- `completion_funcs.go`: house pattern for `ValidArgsFunction` /
  `RegisterFlagCompletionFunc`.
- `internal/output/output.go`: `output.Table()`/`Quiet`, the house non-TUI
  rendering; honors `--output json`.
- `charmbracelet/huh` is introduced by the concurrent `login --otp` TUI
  work (`docs/superpowers/specs/2026-07-07-login-otp-tui-design.md`). This
  design **shares** that dependency and its `isInteractive()` TTY helper;
  the TUI phase (C) depends on that work or adds the dep via `go mod tidy`.
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
├── frontmatter.go               # surgical parse/stamp of SKILL.md
├── detect.go                    # agent detection (injectable dirs)
├── install.go                   # copy embed→disk, hash gate, update logic
└── *_test.go
```

```go
//go:embed all:assets
var assetsFS embed.FS
```

`all:assets` embeds dotfiles too. **Caveat:** `go:embed` omits *empty*
directories and errors at compile time if the pattern matches no files — so
every shipped skill dir must contain at least one file (the seed skill
always does; a build-time catalog test guards it).

```go
type Skill struct {
    Name        string // dir name == frontmatter name
    Description string // from SKILL.md frontmatter
    fsPath      string // path within assetsFS, e.g. "assets/huly-issue-tracking"
}

func Catalog() ([]Skill, error)      // walk assets/*, parse each SKILL.md frontmatter
func Get(name string) (Skill, bool)
```

### 2. Frontmatter parse + surgical stamp (`frontmatter.go`)

Stamping must **not** round-trip the whole `SKILL.md` through
`yaml.Unmarshal`→`yaml.Marshal`: that reorders keys, drops comments,
reflows `>-` folded scalars (real skills use them, e.g. `calico`), and
drops any field we didn't model. Instead:

- **Read:** split the file textually on the first two `---` fences into
  `frontmatter` + `body`. Parse only what we need (`name`, `description`,
  and `metadata.managed_by` / `metadata.huly_cli_version` /
  `metadata.content_hash`) from the frontmatter.
- **Stamp:** parse the frontmatter into a `yaml.Node` (yaml.v3 Node
  preserves key order and comments), find-or-create the `metadata` mapping
  node, set the three keys, and re-emit `---\n<frontmatter>\n---\n` +
  **byte-for-byte-preserved body**. Never route the Markdown body through
  YAML.
- **Quoting:** all stamped metadata scalar values are emitted **quoted**
  (`huly_cli_version: "0.2.0"`, `managed_by: "huly-cli"`,
  `content_hash: "sha256:…"`) so a value like `1.0` can't be parsed as a
  YAML float and truncated.

### 3. Agent detection (`detect.go`)

Detection takes an **injectable directory root** — fixes the macOS bug
(`os.UserConfigDir()` returns `~/Library/Application Support` there, not
`~/.config`) and makes tests pure (no global-env mutation):

```go
type Dirs struct{ Home, ConfigHome string }

func ResolveDirs() Dirs        // Home=os.UserHomeDir(); ConfigHome=$XDG_CONFIG_HOME || Home/.config
func DetectAgents(d Dirs) []Agent
func Detect() []Agent          // = DetectAgents(ResolveDirs())

type Agent struct {
    ID        string // "claude","codex","opencode","cursor","pi"
    Label     string // "Claude Code"
    RootDir   string // detection marker dir
    SkillsDir string // <root>/skills (opencode: <ConfigHome>/opencode/skills)
    Present   bool
}
```

- Claude/Codex/Cursor/Pi resolve from `Home`; **opencode resolves from
  `ConfigHome` explicitly** (XDG with `$HOME/.config` fallback) on all OSes
  — never `os.UserConfigDir()`.
- `Present` = the agent's `RootDir` exists (not `skills/`, which we
  create). All five always returned; UI shows present ones selectable,
  absent ones skipped.
- Windows caveat noted: `os.UserHomeDir` uses `%USERPROFILE%` (ignores
  `$HOME`); tests inject `Dirs` directly rather than relying on `$HOME`.

### 4. Install / update engine (`install.go`)

Provenance lives in the installed `SKILL.md` frontmatter `metadata` map —
no sidecar file:

```yaml
metadata:
  managed_by: "huly-cli"
  huly_cli_version: "0.2.0"        # provenance: which huly wrote this
  content_hash: "sha256:…"          # the upgrade gate
```

`content_hash` is a stable hash over the embedded skill's file tree (sorted
relative paths + contents), **excluding** the `SKILL.md`'s own
`metadata.content_hash`/`huly_cli_version` values so the hash is a pure
function of authored content. The same function computes the embedded hash
and the on-disk hash, so they compare directly.

The current version is **injected**, not read from the global, so the state
machine is unit-testable:

```go
type InstallOpts struct {
    CurrentVersion string // defaulted from version.Version at the cobra layer
    Force, DryRun  bool
}
func Install(sk Skill, agent Agent, opts InstallOpts) (InstallResult, error)
```

Algorithm for one (skill, agent), `dest = <agent.SkillsDir>/<sk.Name>`,
`embHash` = embedded content hash:

1. **`dest` absent** → (dry-run: report `installed`) copy tree; stamp
   `managed_by`, `huly_cli_version=cur`, `content_hash=embHash` →
   `Installed`.
2. **`dest` present, `SKILL.md` absent or unparseable** → demonstrably
   broken, not provably foreign → treat as reinstallable, overwrite (no
   `--force` needed) → `Repaired`.
3. **`dest` present, parses, `managed_by != "huly-cli"`** → true foreign
   skill → `Conflict`; skip unless `--force`.
4. **`dest` present, ours** — compare hashes:
   - on-disk content hash **==** stored `content_hash` (unmodified):
     - `embHash == content_hash` → `UpToDate` (skip). Re-stamp
       `huly_cli_version=cur` only if it changed and content matches
       (cheap provenance refresh, no tree copy).
     - `embHash != content_hash` → shipped content changed → replace tree,
       re-stamp all three → `Updated`.
   - on-disk content hash **!=** stored `content_hash` (**user-edited**
     body): `Conflict` (report `modified`); **warn and skip** unless
     `--force`. Never silently clobber user edits.

Version comparison (for surfacing "update available" and provenance) uses a
lifted semver helper (§7) and **must `TrimPrefix(v, "v")`** on both operands
first — `version.Version` is `v0.1.2`; without the trim `parseVersion`
zeroes the major slot and major releases stop upgrading. The **upgrade gate
is the content hash, not the version** — this removes per-release churn
(bumping the binary with no skill change no longer rewrites every skill) and
the every-release overwrite of user edits.

**Dev builds:** `version.Version == "dev"` stamps `huly_cli_version: "dev"`
as provenance, but since upgrades are content-hash-gated, `update` still
works when skill *content* changes even on a dev binary. Documented in the
command help.

**Directory replacement is not a bare rename.** `os.Rename` cannot replace
a non-empty directory (`ENOTEMPTY`). Replace via: write the new tree to
`os.MkdirTemp(<agent.SkillsDir>, ".<name>.new-*")` (same filesystem),
`os.RemoveAll(dest)`, then `os.Rename(tmp, dest)`; on failure remove the
temp dir. The brief window where `dest` is absent is itself recoverable
(the absent→Installed path). The "atomic" claim from earlier is dropped in
favor of "crash-recoverable."

`install` is **idempotent by default**: `UpToDate` skips (does not re-copy).
`--force` re-asserts the shipped tree regardless (still refuses a *foreign*
dir unless combined intent is explicit). `update` only touches
present+ours skills whose content hash is behind. `uninstall` removes
`dest` only if its frontmatter is ours, unless `--force`.

### 5. Command surface (`cmd/skills.go`, `cmd/skills_tui.go`)

Self-registering `init()` pattern.

```
huly skills                       # TTY → TUI dashboard; non-TTY → print help
huly skills list                  # table: skill × agent status (+ "update available")
huly skills install [name...]
huly skills update  [name...]
huly skills uninstall [name...]
```

**Bare `huly skills`:** you asked for a TUI here. To stay pipe-safe and
match the house convention when scripted, bare `huly skills` launches the
dashboard **only when interactive** (stdin && stderr are TTYs and no
`--no-interactive`); otherwise it prints help (Cobra default). So it hangs
nothing in CI/pipes yet gives the interactive TUI you want at a terminal.

Flags on install/update/uninstall:

- `--agents claude,codex,opencode` — explicit targets (skips picker).
- `--all` — every *detected* agent.
- `--yes`/`-y` — no confirmation. `--force` — override conflict/modified
  guard. `--dry-run` — resolve and report, write nothing.
- `--no-interactive` — force batch path even on a TTY (symmetry with the
  login spec).

**Interactivity** = stdin && **stderr** are TTYs (huh renders to stderr;
matches the login spec) and `--no-interactive` not set and no explicit
`--agents`/`--all`. Factored into one shared `isInteractive()` helper used
by both this feature and login. Non-interactive with no agent selector →
error listing detected agents (never guess).

**Output.** All commands honor global `--output json`. Mutating commands
emit `[{skill, agent, status, path}]` in JSON and, in text mode, one line
per target prefixed with a **stable ASCII status token** (`installed`,
`updated`, `repaired`, `up-to-date`, `modified`, `conflict`, `skipped`,
`error`) so `… | grep conflict` works; a glyph is decoration only. **Exit
code:** non-zero if any target errored *or* ended `conflict`/`modified`
without `--force` (partial-failure detectable by scripts); `--dry-run`
exit is informational (zero).

`list` marks any installed skill whose embedded content hash differs from
its stored hash as **"update available."**

### 6. Charmbracelet TUI (Phase C)

Two entry points, `charmbracelet/huh`, in `cmd/skills_tui.go`:

- **install picker** — `huh.MultiSelect[string]` of detected agents,
  pre-checked, titled with the skill(s); returns chosen agent IDs.
- **dashboard** (bare `huly skills`) — `huh` sequence: MultiSelect skills →
  MultiSelect agents → Select action → results summary. A full
  `bubbletea.Model` only if the form proves limiting.

The engine is UI-agnostic; the TUI collects `(skills, agents, action)`,
calls it, and prints the same ASCII-token result lines.

### 7. Seed skill: `huly-issue-tracking` (with acceptance criteria)

Authored via `/skill-creator` into
`internal/skills/assets/huly-issue-tracking/`. Body teaches an agent to:

- Track bugs/issues for **one** huly project (resolve/confirm once, log
  against it).
- Create and **enable components** to group parts of the codebase, and file
  issues against the right component.
- Use the concrete commands: `huly project list`,
  `huly component list/create`, `huly issue create/list/update/view`.

**Checkable acceptance criteria (enforced by tests, not vibes):**

1. Frontmatter `name == "huly-issue-tracking"`, dir name matches, and
   `managed_by`/`description` present; description mentions issue/bug
   tracking (trigger correctness).
2. **Command-existence guard:** a test greps every `` `huly <verb …>` ``
   token in `SKILL.md` and asserts each maps to a registered command in
   `rootCmd` — so a renamed/removed command breaks CI instead of shipping a
   lying skill.
3. A required-command set (`issue create`, `component create`,
   `project list`) must appear, so the skill can't regress to a stub.

### 8. Version helper lift (`internal/semver` or `version`)

`compareVersions`/`parseVersion` move from `cmd/` into a **leaf package**
(`version`, which imports nothing, or a new `internal/semver`) so
`internal/skills` can import them without the `cmd → internal/skills → cmd`
import cycle. `cmd/update.go` and `update_test.go` are updated to call the
lifted helpers in the same change. This is an explicit task, not a
footnote.

## Phasing

Each phase is independently shippable and testable.

- **Phase A — engine, no UI.** `internal/skills`: embed+catalog,
  `frontmatter` surgical stamp, `Detect` (injectable `Dirs`),
  `Install/Update/Uninstall` (injected version, content-hash gate,
  recovery branch), the semver lift (§8). All engine unit tests land here.
  No cobra, no huh.
- **Phase B — non-interactive CLI.** `skills` group + `list`/`install`/
  `update`/`uninstall` with all flags, `output.Table` + `--output json` +
  ASCII tokens + exit codes + `--dry-run`, completion (`completeSkills`,
  `completeAgents`), docs page (`docs/skills.md` + `zensical.toml` nav),
  post-`update` staleness hint. Fully testable without a TTY.
- **Phase C — TUI.** huh install picker, then the bare-command dashboard
  (the YAGNI cut line). Depends on the login work's `huh` dep +
  `isInteractive()`.

## Data Flow

```
huly skills install
      │  Catalog() (embed)          Detect(ResolveDirs())
      ▼                                   ▼
  pick skills ─────────────▶  agents: --agents/--all/--yes  or  huh picker (TTY)
      │                                   │
      ▼                                   ▼
  for each (skill, agent):  Install(skill, agent, {cur, force, dryRun})
      │   dest missing → install ; broken → repair ; foreign → conflict
      │   ours: embHash==stored → up-to-date ; embHash≠stored & unmodified → update
      │         on-disk≠stored (user-edited) → modified → warn/skip unless --force
      ▼
  write tree (tmp on same FS → RemoveAll(dest) → rename) + surgical stamp
      ▼
  per-target result: ASCII token (+ JSON with --output json); exit code reflects conflicts

Refresh path:  huly update (new binary) → prints "skills may be behind; run
               huly skills update" → huly skills update (content hash differs → replace)
```

## Error Handling

- No agents detected: message naming supported agents + paths; `list` exits
  zero, `install` exits non-zero.
- Non-interactive without `--agents`/`--all`: error listing detected agents.
- Unknown skill name arg: error listing catalog names.
- Foreign skill (`managed_by` != huly-cli): `conflict`, skip, suggest
  `--force`; never overwrite silently.
- User-modified managed skill (on-disk hash ≠ stored): `modified`, warn +
  skip unless `--force`.
- Broken managed skill (unparseable/absent `SKILL.md`): `repaired` by
  reinstall, no `--force` needed.
- Per-target copy/rename/permission failure or a target rejecting the
  frontmatter: wrapped `%w`, that target fails, others proceed; command
  exits non-zero.
- Embedded catalog empty/corrupt: caught by the build-time catalog test.
- TUI cancelled (Ctrl-C/Esc): clean "cancelled", nothing written.

## Testing

- **Catalog integrity + empty-dir guard:** every embedded skill parses; dir
  name == frontmatter `name`; description non-empty; catalog non-empty.
- **Seed-skill acceptance:** the three §7 criteria, incl. the
  command-existence grep against `rootCmd`.
- **Frontmatter surgical stamp:** stamp a SKILL.md with a `>-` folded
  description + comments + an unmodeled field; assert body is byte-identical,
  comments/order/unmodeled field preserved, metadata values quoted.
- **Detection:** `DetectAgents(Dirs{Home: tmpA, ConfigHome: tmpA+"/.config"})`
  with subsets of agent dirs present; assert `Present` and resolved
  `SkillsDir` (incl. opencode under `ConfigHome`, proving the macOS fix).
- **Install/update state machine (version injected):** fresh install
  stamps all three fields; unmodified + same hash → `UpToDate`; unmodified +
  changed embedded hash → `Updated`; user-edited body → `modified`/skip,
  replaced with `--force`; foreign dir → `Conflict`; broken `SKILL.md` →
  `Repaired`; second `update` is a no-op (no churn).
- **Semver lift:** `v1.9.0` vs `v2.0.0` (proves the `TrimPrefix`/major fix);
  `dev` handling; garbage → `[0,0,0]`.
- **Directory replace:** replacing a populated `dest` succeeds (regression
  test for the `os.Rename`-over-non-empty-dir bug).
- **Flags/output:** `--agents`/`--all`/`--yes`/`--dry-run` resolve targets
  without a TTY; `--output json` shape for `list` and mutating commands;
  exit code non-zero on unforced conflict; ASCII tokens greppable.
- huh forms aren't unit-tested (interactive TTY); the engine and
  selector-resolution around them are.

## YAGNI / cut lines

All five agents are in scope. If scope needs trimming, in order: drop
**Phase C**'s dashboard (keep the install picker only); then drop
`uninstall` (users can `rm`). The Phase A engine + Phase B non-interactive
CLI + one seed skill is the irreducible core. Deferred by design (engine
already shaped for them): `--scope project`, `metadata.content_hash`-based
partial merges, network-fetched catalogs.
