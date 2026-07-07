# Design: `huly skills` — embedded agent-skill distributor

**Date:** 2026-07-07
**Status:** Approved (revised after two 5-agent review rounds), ready for implementation plan

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
   pinned by checkable acceptance criteria (§7).
3. Agent detection with an injectable directory root (testable, correct on
   Linux/macOS/Windows).
4. An install/update/uninstall **engine** in `internal/skills` that stamps
   provenance into frontmatter and gates upgrades on a content hash of the
   authored body + sibling files.
5. A `huly skills` Cobra command group: `list`, `install`, `update`,
   `uninstall`, `tui`, plus a bare-command dashboard.
6. Non-interactive flags (`--agents`, `--all`, `--yes`, `--force`,
   `--dry-run`, `--no-interactive`, `--fail-on-conflict`) with
   greppable/JSON output and defined exit codes.
7. A charmbracelet/huh TUI (agent picker + dashboard).
8. Docs page + nav entry and shell completion, per repo convention.

Delivered in **four plan-sized phases** (A1, A2, B, C — see Phasing) so each
is independently shippable and testable.

Out of scope: authoring more than the one seed skill (the catalog grows by
dropping a dir in `assets/`); downloading skills from the network (they are
embedded); editing agents' own config files (`opencode.json`, `AGENTS.md`);
project-local install scope (global home dirs only for now — but the engine
takes an explicit target dir so `--scope project` can be added later);
3-way merge of user edits (huly keeps only a hash, never the base tree, so a
real merge is impossible; `--force` with backup is the v1 story).

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
Skills spec (agentskills.io) — a *map from string keys to string values*,
explicitly for "additional properties not defined by the spec," with **no
size limit and no value-schema validation** (only `name`/`description`/
`compatibility` are length-bounded). Only `name` + `description` load into
the model's discovery context; `metadata` is not surfaced at discovery. A
`sha256:` string (~72 bytes) is negligible. Verified tolerant: **Cursor**
(docs) and **Claude Code** (on-disk skills carry far heavier frontmatter —
nested `hooks:`/`gbrain:` maps — and load fine). **Codex** and **Pi**
document only `name`/`description`; residual **low** risk they are stricter.
Mitigation: install is best-effort per target — a target whose parser
rejects the frontmatter fails *that target only*, never aborts the run.
Namespacing everything under `metadata:` (vs top-level keys) is the
spec-sanctioned, safest choice. This mirrors `wrangler` (detect → prompt →
copy; `--install-skills` for non-interactive).

## Current State

- No `//go:embed` anywhere in the repo (greenfield for embedding).
- `cmd/*.go`: each command self-registers via its own `init()` calling
  `rootCmd.AddCommand`. Leaf commands use `RunE`. Parent grouping commands
  are one-line `&cobra.Command{Use, Short}` **with no `RunE`**, so bare
  `huly issue`/`huly config` print help (Cobra default) — the pattern the
  bare `skills` command follows in Phase B. `--output` is a global
  viper-bound flag (`cmd/root.go`).
- `cmd/update.go`: self-updates the binary. `compareVersions`/`parseVersion`
  (L364–412) are the semver helpers to lift; note `runUpdate`
  `strings.TrimPrefix(v,"v")` **at the call site** (L80–81) — `parseVersion`
  does not trim, so a `v`-prefixed value silently zeroes the major slot.
- `version/version.go`: `Version` defaults to `"dev"`; ldflags set it to
  `git describe --tags --always` (`just/go.just:5`) → `v0.1.2` or
  `v0.1.2-3-gSHA`. The package imports nothing (safe lift target).
- `internal/creds/creds.go`: XDG-resolution + `t.Setenv` precedent.
- `completion_funcs.go`: house `ValidArgsFunction`/`RegisterFlagCompletionFunc`.
- `internal/output/output.go`: `output.Table()`/`Quiet`, honors `--output json`.
- `charmbracelet/huh` is introduced by the concurrent `login --otp` work
  (`docs/superpowers/specs/2026-07-07-login-otp-tui-design.md`), which
  inlines an `otpInteractive()` TTY check in `login.go` (no shared helper
  exists yet). Phase C adds the huh dep if login hasn't merged and factors
  the shared `isInteractive()` (§5).
- No `README`/`CHANGELOG` in the repo; the zensical docs site is the doc
  surface. Tests run via `just cicd::test` (`go test ./...` in `src/huly`),
  so any `_test.go` under `internal/skills`/`cmd` runs in CI automatically —
  no new/skipped just recipe needed.
- Go 1.25, module root `src/huly/`, module path
  `github.com/kettleofketchup/huly-cli/src/huly`.

## Design

### 1. Embedded catalog (`internal/skills`)

```
internal/skills/
├── assets/huly-issue-tracking/{SKILL.md, references/…}   # embedded (authored)
├── catalog.go        # //go:embed assets + catalog parsing
├── frontmatter.go    # split/parse + surgical stamp of SKILL.md
├── hash.go           # content hash (body + sibling files)
├── detect.go         # agent detection (injectable dirs)
├── install.go        # install/update/uninstall engine
└── *_test.go
```

```go
//go:embed all:assets
var assetsFS embed.FS
```

`all:assets` embeds dotfiles too. **Caveats:** `go:embed` omits *empty*
directories and is a **compile error** on a zero-file match — so the seed
skill asset must exist (with ≥1 file per shipped dir) before this compiles;
authoring it is therefore the **first** task (A1). A build-time catalog test
guards non-emptiness.

```go
type Skill struct { Name, Description, fsPath string }
func Catalog() ([]Skill, error)
func Get(name string) (Skill, bool)
```

### 2. Frontmatter: authored vs injected, surgical stamp (`frontmatter.go`)

**Field ownership is fixed:**

- **Authored** into the embedded seed `SKILL.md`: `name`, `description`,
  and `metadata.managed_by: "huly-cli"` (so it's recognizably ours even
  before install).
- **Injected** at install time: `metadata.huly_cli_version`,
  `metadata.content_hash`.

**Stamping** must not round-trip the whole file through
`yaml.Unmarshal`→`Marshal`. Instead: split textually on the first two `---`
fences into `frontmatter` + `body`; parse the frontmatter into a `yaml.Node`
(preserves key order/comments); find-or-create the `metadata` mapping node;
set/replace the two injected keys **quoted** (`huly_cli_version: "0.2.0"`,
`content_hash: "sha256:…"`); re-emit `---\n<frontmatter>\n---\n` with
`enc.SetIndent(2)` + the **body written back byte-for-byte**.

**Explicitly:** only the **body** is guaranteed byte-preserved; the
frontmatter is *semantically* preserved but may be re-emitted (indent/wrap),
which is exactly why the content hash (§3) must not depend on frontmatter
bytes.

### 3. Content hash — the upgrade gate (`hash.go`)

The gate is a hash that is **invariant across the authored→installed
transform**. The only guaranteed-invariant content is the SKILL.md **body**
and the **sibling files**; the frontmatter is not (it carries injected keys
and may be re-emitted). Therefore:

```
contentHash(tree) =
  sha256( join_sorted_by_relpath(
     for SKILL.md:      "SKILL.md\x00"  + body_after_frontmatter   // frontmatter EXCLUDED
     for every other f: relpath + "\x00" + raw_bytes
  ) )
```

The **same function** runs on the embedded tree (→ `embHash`, stored as
`content_hash` at install) and on the on-disk tree. Because the frontmatter
is excluded entirely, stamping/​re-stamping any metadata key never moves the
hash, and a pristine install hashes identically to the embedded source — so
it is **not** misread as user-modified. Trade-off accepted: a future skill
revision that changes *only* the frontmatter `description` won't trigger
`update` (body/sibling changes do); such a change can ride a one-character
body edit or `--force`.

### 4. Install / update / uninstall engine (`install.go`)

Provenance in frontmatter `metadata` (no sidecar):

```yaml
metadata: { managed_by: "huly-cli", huly_cli_version: "0.2.0", content_hash: "sha256:…" }
```

Version is **injected** for testability; results carry a **reason** so the
CLI can pick a token:

```go
type InstallOpts struct { CurrentVersion string; Force, DryRun bool }
type Status  string // installed|updated|repaired|up-to-date|modified|conflict|removed|skipped
type Result  struct { Skill, Agent, Path string; Status Status; Reason string }
func Install(sk Skill, ag Agent, o InstallOpts) (Result, error)
func Uninstall(sk Skill, ag Agent, o InstallOpts) (Result, error)
```

Before any copy: `os.MkdirAll(ag.SkillsDir, 0o755)`; sweep stale
`.*.new-*` temp dirs in `SkillsDir`. Enumeration (catalog/detect/list)
ignores dot-prefixed dirs.

`Install` for one (skill, agent), `dest = ag.SkillsDir/sk.Name`,
`embHash` = embedded content hash:

1. **`dest` absent** → copy tree; stamp `managed_by` + `huly_cli_version=cur`
   + `content_hash=embHash` → `installed`.
2. **`dest` present, `SKILL.md` absent/unparseable** → cannot prove
   ownership → `conflict` (reason `unreadable`); skip unless `--force`
   (with `--force`: back up `dest`→`dest.bak-<ts>` then reinstall →
   `repaired`). *Safe default: never delete what we can't prove is ours.*
3. **`dest` present, parses, `managed_by != "huly-cli"`** → `conflict`
   (reason `foreign`); skip unless `--force`.
4. **`dest` present, ours** — compare on-disk content hash to stored
   `content_hash`:
   - **stored hash missing** (ours but pre-hash) → adopt: re-stamp
     `content_hash=embHash` (+ copy tree if `embHash` differs) → `updated`
     (reason `adopted`).
   - **on-disk == stored** (unmodified):
     - `embHash == stored` → `up-to-date`. Provenance-refresh only:
       re-stamp `huly_cli_version=cur` if changed (hash-neutral, no copy).
     - `embHash != stored` → shipped content changed → replace tree,
       re-stamp all → `updated`.
   - **on-disk != stored** (**user-edited body/files**) → `conflict`
     (reason `modified`); warn + skip unless `--force` (with `--force`:
     back up `dest`→`dest.bak-<ts>`, then replace → `updated`).

`Install` is **idempotent by default** (`up-to-date` does not re-copy).
`--force` re-asserts the shipped tree (still backs up a `modified` dir
first; still refuses a `foreign` dir's semantics by backing up, not
merging). `update` only touches present+ours skills whose content hash is
behind. `Uninstall` removes `dest` only if its frontmatter is ours
(→ `removed`), else `conflict` unless `--force`; a not-present target →
`skipped`.

**Directory replacement** (never a bare `os.Rename` over a non-empty dir,
which fails `ENOTEMPTY`): write the new tree to
`os.MkdirTemp(ag.SkillsDir, ".<name>.new-*")` (same filesystem),
`chmod 0755`, `os.RemoveAll(dest)`, `os.Rename(tmp,dest)`; on error remove
tmp. Crash-recoverable (a missing `dest` re-installs cleanly), not atomic.

**Versions never gate upgrades.** `content_hash` alone drives `update` and
`list`'s "update available." `huly_cli_version` is **provenance/display
only**. The lifted semver helper (§8) — used only for display ordering —
**normalizes internally** (`TrimPrefix(v,"v")` inside `parseVersion`) so no
caller can reintroduce the major-zeroing bug; `"dev"` and `v…-N-gSHA`
pseudo-versions only affect the provenance string shown in `list`.

### 5. Command surface (`cmd/skills.go`, `cmd/skills_tui.go`)

```
huly skills                       # (Phase B) prints help; (Phase C) TTY→dashboard, else help
huly skills tui                   # (Phase C) launch dashboard; error "requires a terminal" if non-TTY
huly skills list                  # table: skill × agent status (+ "update available")
huly skills install [name...]
huly skills update  [name...]
huly skills uninstall [name...]
```

**Bare `huly skills` across phases (additive, no forward refs):** Phase B
ships it as a **no-`RunE` grouping command** (help at TTY and non-TTY,
matching `issue`/`config`). Phase C **adds** a `RunE` (with
`Args: cobra.NoArgs`) that launches the dashboard when interactive and
prints help otherwise, plus the explicit `huly skills tui` for a scripted,
completion-listed entry point. The group `Long` documents the split.

Flags (install/update/uninstall): `--agents claude,codex,opencode`,
`--all`, `--yes`/`-y`, `--force`, `--dry-run`, `--no-interactive`,
`--fail-on-conflict`.

**Interactivity** = stdin && **stderr** are TTYs (huh renders to stderr;
matches login) and `--no-interactive` unset and no explicit `--agents`/
`--all`. Factored into a shared `isInteractive()` helper in package `cmd`
(Phase C extracts it and rewires login's `otpInteractive()` to it). In
**Phase B** there is no picker, so `install`/`update` without a selector
require an explicit `--agents`/`--all` (or default to `--all` detected) —
Phase B is effectively always-non-interactive for selection.

**Output & exit codes.** All commands honor `--output json`; mutating
commands emit `[{skill,agent,status,path,reason}]`. Text mode prints one
line per target prefixed with a **stable ASCII status token** (the `Status`
values above) so `… | grep conflict` works. **Exit code:** `0` when the run
completed — including targets skipped by policy (`modified`/`conflict`/
`up-to-date`/`skipped`), which are reported via tokens, not failures;
**non-zero only for genuine errors** (copy/rename/permission/parse-of-ours).
`--fail-on-conflict` opts into non-zero when any target ends
`conflict`/`modified` (for CI drift-gating). `--dry-run` runs the full
resolution, writes nothing, and applies the **same** exit-code logic
(so `--dry-run --fail-on-conflict` is a real CI preview gate).

`list` on zero installed skills (with agents present) prints a one-line
nudge to `install`; the no-agents-detected case is handled in Error
Handling.

### 6. Charmbracelet TUI (Phase C)

`cmd/skills_tui.go`, `charmbracelet/huh`: an **install picker**
(`huh.MultiSelect` of detected agents, pre-checked) and a **dashboard**
(MultiSelect skills → MultiSelect agents → Select action → summary). The
engine is UI-agnostic; the TUI collects `(skills, agents, action)`, calls
it, and prints the same ASCII-token result lines. `bubbletea.Model` only if
`huh` proves limiting.

### 7. Seed skill: `huly-issue-tracking`

Authored via `/skill-creator` into `internal/skills/assets/huly-issue-tracking/`
(first A1 task). Body teaches an agent to: track bugs/issues for **one** huly
project; create and **enable components** to group parts of the codebase and
file issues against the right component; use `huly project list`,
`huly component list/create`, `huly issue create/list/update/view`.

**Checkable acceptance criteria (tests, not vibes):**

1. *(A1)* Frontmatter `name == "huly-issue-tracking"`, dir name matches,
   `metadata.managed_by == "huly-cli"`, description non-empty and mentions
   issue/bug tracking.
2. *(B — needs `rootCmd`, so lives in package `cmd`)* **Command-existence
   guard:** extract `huly …` command paths **only from code spans** (inline
   backticks + fenced blocks) in `SKILL.md`, tokenize until the first
   flag/placeholder, and assert each resolves via `rootCmd.Find(path)` to a
   real **leaf** command (`err==nil && !c.HasSubCommands()`). Reuses Cobra's
   resolver — no hand-matching, no prose false positives.
3. *(A1)* A required-command set (`issue create`, `component create`,
   `project list`) appears in code spans (resolved via `rootCmd.Find` in the
   Phase-B test), so the skill can't regress to a stub.

### 8. Version helper lift (`internal/semver`)

Move `compareVersions`/`parseVersion` from `cmd/` into a **leaf package**
(new `internal/semver`, or `version`) so `internal/skills`/`cmd` import it
without the `cmd → internal/skills → cmd` cycle. **Bake `TrimPrefix(v,"v")`
into `parseVersion`** so the major-zeroing bug can't recur. Rewire
`cmd/update.go` + `update_test.go` to the lifted helper in the same task.

## Phasing

Four forward-only, plan-sized phases:

- **A1 — primitives.** Author the seed skill asset (first); semver lift
  (§8); embed+catalog + integrity test; frontmatter split/parse/surgical
  stamp + test; content hash (§3) + exclusion/re-stamp-invariance test;
  seed criteria 1 & 3. No cobra, no huh.
- **A2 — engine.** `detect.go` (`Dirs` injection, opencode via
  `ConfigHome`); install/update/uninstall state machine (reasons, backups,
  dir-replace recovery, temp-dir sweep) with injected version + all state
  tests. Depends on A1. No cobra, no huh.
- **B — non-interactive CLI.** `skills` group (no-`RunE` bare) + `list`/
  `install`/`update`/`uninstall` with all flags, `output.Table` +
  `--output json` + ASCII tokens + exit codes + `--dry-run` +
  `--fail-on-conflict`; completion (`completeSkills`/`completeAgents`); docs
  page (`docs/skills.md` + `zensical.toml` nav) documenting only shipped
  behavior; post-`update` staleness hint; zero-state `list` nudge; seed
  criterion 2 test. Fully testable without a TTY.
- **C — TUI.** huh install picker; bare-command `RunE` dashboard +
  `huly skills tui`; shared `isInteractive()` extraction (rewire login);
  huh dep if not already present. The YAGNI cut line.

## Data Flow

```
huly skills install
   Catalog() (embed)                 Detect(ResolveDirs())
        │                                    │
   pick skills ─────▶ agents: --agents/--all  or  huh picker (Phase C, TTY)
        │                                    │
   for each (skill,agent): Install(skill, agent, {cur, force, dryRun})
        │  MkdirAll(SkillsDir); sweep .*.new-*
        │  dest absent→installed ; unreadable→conflict(unreadable) ; foreign→conflict(foreign)
        │  ours: on-disk==stored & embHash==stored → up-to-date (refresh provenance)
        │        on-disk==stored & embHash≠stored  → updated (replace tree)
        │        on-disk≠stored (user-edited)      → conflict(modified); --force→backup+updated
        ▼
   write tree (tmp same FS → chmod → RemoveAll(dest) → rename) + surgical stamp
        ▼
   per-target: ASCII token(+reason) / JSON; exit 0 unless error (or --fail-on-conflict)

Refresh:  huly update → "skills may be behind; run huly skills update"
          → huly skills update (embHash≠stored → replace)
```

## Error Handling

- No agents detected: message naming agents + paths; `list` exits 0,
  `install` exits non-zero.
- Non-interactive without `--agents`/`--all`: error listing detected agents.
- Unknown skill name arg: error listing catalog names.
- Foreign skill: `conflict(foreign)`, skip, suggest `--force`.
- User-modified managed skill: `conflict(modified)`, warn + skip unless
  `--force` (which backs up first).
- Unreadable/absent managed `SKILL.md`: `conflict(unreadable)`; `--force`
  backs up + repairs.
- Per-target copy/rename/permission failure, or a target rejecting the
  frontmatter: wrapped `%w`; that target errors, others proceed; command
  exits non-zero.
- Embedded catalog empty/corrupt: compile error (`go:embed`) / build-time
  catalog test.
- TUI cancelled (Ctrl-C/Esc): clean "cancelled", nothing written.

## Testing

- **Catalog integrity + empty-dir guard** (A1): every skill parses; dir name
  == `name`; description non-empty; catalog non-empty.
- **Content hash invariance** (A1): mutating only `metadata.*` leaves the
  hash unchanged; a fresh install's on-disk hash == `embHash` (no false
  `modified`); a body/sibling edit *does* change it (positive control).
- **Frontmatter surgical stamp** (A1): stamp a SKILL.md with a `>-` folded
  description, comments, and an unmodeled field; assert body byte-identical
  and metadata values quoted.
- **Semver lift** (A1): `v1.9.0` vs `v2.0.0` (proves internal `TrimPrefix`);
  `dev` handling; garbage → `[0,0,0]`.
- **Detection** (A2): `DetectAgents(Dirs{Home,ConfigHome})` over subsets;
  assert `Present` + resolved `SkillsDir`, incl. opencode under
  `ConfigHome` (the macOS fix).
- **Install state machine** (A2, version injected): installed / up-to-date /
  updated / modified→skip-then-`--force`-with-backup / foreign→conflict /
  unreadable→conflict / adopt-missing-hash / removed / not-present→skipped;
  second `update` is a no-op (no churn); dir-replace over a populated dest
  succeeds (regression for the `Rename`-over-non-empty bug); orphan
  `.*.new-*` swept and dot-dirs ignored.
- **Command-existence** (B, in `cmd`): code-span extraction + `rootCmd.Find`
  for the seed skill; required-command set present.
- **Flags/output/exit** (B): selectors resolve without a TTY; JSON shape for
  `list` + mutating commands; exit 0 on policy-skips, non-zero on error and
  under `--fail-on-conflict`; `--dry-run` writes nothing but honors exit
  logic; tokens greppable.
- huh forms aren't unit-tested; the engine + selector resolution around them
  are.

## YAGNI / cut lines

All five agents in scope. Trim order if needed: drop **Phase C**'s dashboard
(keep the install picker); then `uninstall` (users can `rm`). A1+A2 engine +
Phase B non-interactive CLI + one seed skill is the irreducible core.
Deferred by design (engine already shaped for them): `--scope project`,
frontmatter-only-change detection, 3-way merge, network-fetched catalogs.
