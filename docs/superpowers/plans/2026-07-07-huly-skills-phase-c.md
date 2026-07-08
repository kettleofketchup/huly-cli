# huly skills — Phase C (TUI) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Charmbracelet `huh` TUI on top of the Phase B `huly skills` commands: an interactive agent picker when a mutating command is run without a selector at a terminal, and a bare-`huly skills` dashboard (browse skills → agents → action).

**Architecture:** New `src/huly/cmd/skills_tui.go` holds all `huh` code and the TTY gating; `skills.go` gains `--yes`/`--no-interactive` flags, a bare-command `RunE`, and a `huly skills tui` subcommand, and `runSkillsOp` falls back to the picker only when interactive and no `--agents`/`--all` was given. The engine (`internal/skills`) and the Phase B pure helpers are unchanged. huh forms need a real TTY, so all TUI paths are gated behind `isInteractive()`; batch/CI paths keep the Phase B behavior exactly. Spec: `docs/superpowers/specs/2026-07-07-huly-skills-distribution-design.md` (§5 bare-command + §6 TUI).

**Tech Stack:** Go 1.25, `spf13/cobra`, `github.com/charmbracelet/huh` (NEW dependency), `golang.org/x/term` (already present).

## Global Constraints

- Module path `github.com/kettleofketchup/huly-cli/src/huly`; module root `src/huly/`. CLI code in package `cmd`.
- **New dependency:** `github.com/charmbracelet/huh` (pulls bubbletea/lipgloss/bubbles). Add via `go get`; record with `go mod tidy`.
- **Interactivity gate:** interactive iff stdin **and** stderr are TTYs (huh renders to stderr) AND `--no-interactive` was not passed. Never launch a huh form when not interactive — fall back to the Phase B batch behavior (explicit `--agents`/`--all` required, error otherwise).
- **Additive, no Phase B regressions:** with `--agents`/`--all`/`--no-interactive`, or in a non-TTY, `install`/`update`/`uninstall` behave exactly as Phase B (same resolution, tokens, exit codes). The picker is used ONLY when interactive AND no selector flag was given.
- Bare `huly skills`: `RunE` launches the dashboard when interactive, else prints help (`cmd.Help()`); `Args: cobra.NoArgs` so `huly skills bogus` errors.
- Engine interfaces are unchanged from A2 (`Catalog`, `Get`, `Detect`, `Install/Update/Uninstall`, `InstallOpts`, `Result`, `Status`); Phase B helpers (`presentAgents`, `presentIDs`, `resolveAgents`, `renderResults`, `exitError`, `runSkillsOp`) are reused.
- huh forms are not unit-tested (they need a TTY); the pure gating helper (`wantInteractive`) and the batch fallbacks are. This matches the house rule used in A2/B ("engine tested, forms not").
- TDD where a pure seam exists; commit per task; commit style `feat(skills): …`; no Claude watermark/co-author lines.
- Test commands: `cd src/huly && go test ./cmd/ -run <Name> -v`; full `go test ./...`; lint `golangci-lint run ./cmd/...`.

## File Structure

- `src/huly/go.mod` / `go.sum` — MODIFY: add `charmbracelet/huh`.
- `src/huly/cmd/skills_tui.go` — CREATE: `wantInteractive`, `isInteractive`, `pickAgents`, `confirmApply`, `runDashboard`.
- `src/huly/cmd/skills.go` — MODIFY: `--yes`/`--no-interactive` flags; bare-command `RunE`; `skillsTUICmd`; `runSkillsOp` picker fallback.
- `src/huly/cmd/skills_test.go` — MODIFY: `wantInteractive` truth-table test; update `TestNoDuplicateSkillsSubcommands` to expect 5 subcommands.
- `docs/skills.md` — MODIFY: document the TUI, `huly skills tui`, `--yes`, `--no-interactive`.

---

### Task 1: huh dependency + interactivity gate + agent picker

**Files:**
- Modify: `src/huly/go.mod`, `src/huly/go.sum`
- Create: `src/huly/cmd/skills_tui.go`
- Modify: `src/huly/cmd/skills.go`
- Modify: `src/huly/cmd/skills_test.go`

**Interfaces:**
- Produces: `wantInteractive(noInteractive, stdinTTY, stderrTTY bool) bool`; `isInteractive(noInteractive bool) bool`; `pickAgents(present []skills.Agent) ([]skills.Agent, error)`; `confirmApply(action string, skillNames, agentIDs []string) (bool, error)`. Package flag vars `skillsYes`, `skillsNoInteractive`.

- [ ] **Step 1: Add the dependency**

Run: `cd src/huly && go get github.com/charmbracelet/huh@latest && go mod tidy`
Expected: `go.mod` now lists `github.com/charmbracelet/huh`; `go build ./...` still succeeds. If `go get` needs network and it's unavailable, STOP and report BLOCKED (this is a hard prerequisite).

- [ ] **Step 2: Write the failing test (pure gate)**

Add to `src/huly/cmd/skills_test.go`:

```go
func TestWantInteractive(t *testing.T) {
	cases := []struct {
		noInteractive, stdinTTY, stderrTTY, want bool
	}{
		{false, true, true, true},   // both TTY, not suppressed -> interactive
		{true, true, true, false},   // --no-interactive forces batch
		{false, false, true, false}, // stdin not a TTY (piped in)
		{false, true, false, false}, // stderr not a TTY (redirected)
		{false, false, false, false},
	}
	for _, c := range cases {
		if got := wantInteractive(c.noInteractive, c.stdinTTY, c.stderrTTY); got != c.want {
			t.Errorf("wantInteractive(no=%v,in=%v,err=%v) = %v, want %v",
				c.noInteractive, c.stdinTTY, c.stderrTTY, got, c.want)
		}
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd src/huly && go test ./cmd/ -run TestWantInteractive -v`
Expected: FAIL — `wantInteractive` undefined.

- [ ] **Step 4: Write `skills_tui.go`**

Create `src/huly/cmd/skills_tui.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"golang.org/x/term"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/skills"
)

// wantInteractive is the pure interactivity decision: a huh form may run only
// when both stdin and stderr are TTYs (huh renders to stderr) and the user did
// not pass --no-interactive.
func wantInteractive(noInteractive, stdinTTY, stderrTTY bool) bool {
	return !noInteractive && stdinTTY && stderrTTY
}

// isInteractive resolves wantInteractive against the real process TTYs.
func isInteractive(noInteractive bool) bool {
	return wantInteractive(noInteractive,
		term.IsTerminal(int(os.Stdin.Fd())),
		term.IsTerminal(int(os.Stderr.Fd())))
}

// pickAgents shows a pre-checked multi-select of the present agents and returns
// the chosen ones. An empty selection returns an error (nothing to do).
func pickAgents(present []skills.Agent) ([]skills.Agent, error) {
	opts := make([]huh.Option[string], 0, len(present))
	for _, a := range present {
		opts = append(opts, huh.NewOption(a.Label, a.ID).Selected(true))
	}
	var chosen []string
	form := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Install into which agents?").
			Options(opts...).
			Value(&chosen),
	))
	if err := form.Run(); err != nil {
		return nil, err
	}
	byID := map[string]skills.Agent{}
	for _, a := range present {
		byID[a.ID] = a
	}
	var out []skills.Agent
	for _, id := range chosen {
		out = append(out, byID[id])
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no agents selected")
	}
	return out, nil
}

// confirmApply asks for a yes/no before a mutating action; returns the choice.
func confirmApply(action string, skillNames, agentIDs []string) (bool, error) {
	ok := false
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("%s %d skill(s) into %d agent(s)?", action, len(skillNames), len(agentIDs))).
			Affirmative("Yes").
			Negative("No").
			Value(&ok),
	))
	if err := form.Run(); err != nil {
		return false, err
	}
	return ok, nil
}
```

Note: verify the `huh` API against the pinned version with `go doc github.com/charmbracelet/huh NewMultiSelect` / `NewConfirm` / `NewOption`; the shapes above match huh's stable API (`NewOption[string](label, value).Selected(bool)`, `NewMultiSelect[string]().Options(...).Value(&slice)`, `NewConfirm().Value(&bool)`, `NewForm(NewGroup(...)).Run()`). Adjust only if the pinned version differs.

- [ ] **Step 5: Run test to verify it passes**

Run: `cd src/huly && go build ./... && go test ./cmd/ -run TestWantInteractive -v`
Expected: build OK; test PASS.

- [ ] **Step 6: Wire the picker into `runSkillsOp` + add flags**

In `src/huly/cmd/skills.go`:

1. Add two flag vars alongside the existing ones:

```go
var (
	skillsYes           bool
	skillsNoInteractive bool
)
```

2. In `runSkillsOp`, replace the agent-resolution block (currently `detected, err := skills.Detect(); ... agents, err := resolveAgents(detected, skillsAgents, skillsAll)`) with a picker fallback:

```go
	detected, err := skills.Detect()
	if err != nil {
		return err
	}
	var agents []skills.Agent
	if skillsAgents == "" && !skillsAll && isInteractive(skillsNoInteractive) {
		// No explicit selector at a terminal: show the picker.
		present := presentAgents(detected)
		if len(present) == 0 {
			return fmt.Errorf("%s", noAgentsMessage(detected))
		}
		agents, err = pickAgents(present)
		if err != nil {
			return err
		}
		if !skillsYes {
			ok, cerr := confirmApply(op, skillNames(sks), presentIDs(agents))
			if cerr != nil {
				return cerr
			}
			if !ok {
				fmt.Fprintln(os.Stderr, "cancelled")
				return nil
			}
		}
	} else {
		agents, err = resolveAgents(detected, skillsAgents, skillsAll)
		if err != nil {
			return err
		}
	}
```

3. Add the small helper `skillNames` to `skills_run.go`:

```go
// skillNames returns the names of the given skills.
func skillNames(sks []skills.Skill) []string {
	out := make([]string, 0, len(sks))
	for _, s := range sks {
		out = append(out, s.Name)
	}
	return out
}
```

4. Register the two new flags on the three mutating commands (extend the existing flag loop in `init()`):

```go
		c.Flags().BoolVar(&skillsYes, "yes", false, "skip the interactive confirmation")
		c.Flags().BoolVar(&skillsNoInteractive, "no-interactive", false, "never prompt; require --all/--agents")
```

- [ ] **Step 7: Run tests + build + lint**

Run: `cd src/huly && go build ./... && go test ./cmd/ -v 2>&1 | tail -20 && golangci-lint run ./cmd/... 2>&1 | tail -3`
Expected: build OK; all `cmd` tests PASS (including the Phase B suite unchanged); lint 0 issues.

- [ ] **Step 8: Batch-path smoke (no TTY, must match Phase B)**

Run (piping makes stdin non-TTY, so the picker must NOT trigger):
```bash
cd src/huly
echo "" | env HOME=$(mktemp -d) XDG_CONFIG_HOME=/nonexistent go run . skills install
```
Expected: it does NOT hang on a form; it errors with the Phase B "select agents with --all or --agents" message (no agents present here → the no-agents message), exit non-zero. Confirms the non-TTY fallback.

- [ ] **Step 9: Commit**

```bash
git add src/huly/go.mod src/huly/go.sum src/huly/cmd/skills_tui.go src/huly/cmd/skills.go src/huly/cmd/skills_run.go src/huly/cmd/skills_test.go
git commit -m "feat(skills): huh agent picker + interactivity gate + --yes/--no-interactive"
```

---

### Task 2: bare-command dashboard + `huly skills tui`

**Files:**
- Modify: `src/huly/cmd/skills_tui.go` (add `runDashboard`)
- Modify: `src/huly/cmd/skills.go` (bare `RunE`, `skillsTUICmd`, register)
- Modify: `src/huly/cmd/skills_test.go` (update the subcommand-count guard)

**Interfaces:**
- Consumes: Task 1 helpers, engine ops, `renderResults`.
- Produces: `runDashboard() error`; `skillsTUICmd`; a `RunE` on `skillsCmd`.

- [ ] **Step 1: Update the subcommand-count guard (now 5)**

In `src/huly/cmd/skills_test.go`, change `TestNoDuplicateSkillsSubcommands` to expect **5** (list/install/update/uninstall/tui):

```go
func TestNoDuplicateSkillsSubcommands(t *testing.T) {
	if n := len(skillsCmd.Commands()); n != 5 {
		names := make([]string, 0, n)
		for _, c := range skillsCmd.Commands() {
			names = append(names, c.Name())
		}
		t.Errorf("skills has %d subcommands %v, want 5 (duplicate init()?)", n, names)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./cmd/ -run TestNoDuplicateSkillsSubcommands -v`
Expected: FAIL — currently 4 subcommands, want 5 (the `tui` command doesn't exist yet).

- [ ] **Step 3: Write `runDashboard`**

Add to `src/huly/cmd/skills_tui.go`:

```go
// runDashboard is the bare `huly skills` interactive flow: choose skills, then
// agents, then an action, then apply and print the result tokens.
func runDashboard() error {
	cat, err := skills.Catalog()
	if err != nil {
		return err
	}
	detected, err := skills.Detect()
	if err != nil {
		return err
	}
	present := presentAgents(detected)
	if len(present) == 0 {
		return fmt.Errorf("%s", noAgentsMessage(detected))
	}

	skillOpts := make([]huh.Option[string], 0, len(cat))
	for _, s := range cat {
		skillOpts = append(skillOpts, huh.NewOption(s.Name, s.Name).Selected(true))
	}
	agentOpts := make([]huh.Option[string], 0, len(present))
	for _, a := range present {
		agentOpts = append(agentOpts, huh.NewOption(a.Label, a.ID).Selected(true))
	}

	var chosenSkills, chosenAgents []string
	var action string
	form := huh.NewForm(
		huh.NewGroup(huh.NewMultiSelect[string]().Title("Skills").Options(skillOpts...).Value(&chosenSkills)),
		huh.NewGroup(huh.NewMultiSelect[string]().Title("Agents").Options(agentOpts...).Value(&chosenAgents)),
		huh.NewGroup(huh.NewSelect[string]().Title("Action").
			Options(huh.NewOption("Install", "install"), huh.NewOption("Update", "update"), huh.NewOption("Uninstall", "uninstall")).
			Value(&action)),
	)
	if err := form.Run(); err != nil {
		return err
	}
	if len(chosenSkills) == 0 || len(chosenAgents) == 0 {
		fmt.Fprintln(os.Stderr, "nothing selected")
		return nil
	}

	// Resolve selections back to engine types and apply.
	byAgent := map[string]skills.Agent{}
	for _, a := range present {
		byAgent[a.ID] = a
	}
	opts := skills.InstallOpts{CurrentVersion: version.Version, Force: skillsForce, DryRun: skillsDryRun}
	var results []skills.Result
	failed := false
	for _, name := range chosenSkills {
		sk, ok := skills.Get(name)
		if !ok {
			continue
		}
		for _, id := range chosenAgents {
			ag := byAgent[id]
			var r skills.Result
			var e error
			switch action {
			case "install":
				r, e = skills.Install(sk, ag, opts)
			case "update":
				r, e = skills.Update(sk, ag, opts)
			case "uninstall":
				r, e = skills.Uninstall(sk, ag, opts)
			}
			if e != nil {
				r = skills.Result{Skill: sk.Name, Agent: ag.ID, Status: "error", Reason: e.Error()}
				failed = true
			}
			results = append(results, r)
		}
	}
	if err := renderResults(os.Stdout, results, false); err != nil {
		return err
	}
	return exitError(results, failed, skillsFailOnConflict)
}
```

`skills_tui.go` must import `"github.com/kettleofketchup/huly-cli/src/huly/version"` for `version.Version` (used above), alongside `huh`, `golang.org/x/term`(Task 1), `fmt`, `os`, and the `internal/skills` + (for `renderResults`/`exitError`, which live in package `cmd`) no extra import — they're same-package.

- [ ] **Step 4: Add the bare `RunE` + `skills tui` command**

In `src/huly/cmd/skills.go`:

1. Give `skillsCmd` a `RunE` and `Args`:

```go
var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Install and manage huly's embedded agent skills",
	Long:  `... (keep the existing Long text) ...`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if isInteractive(false) {
			return runDashboard()
		}
		return cmd.Help()
	},
}
```

2. Add the explicit `tui` command:

```go
var skillsTUICmd = &cobra.Command{
	Use:   "tui",
	Short: "Open the interactive skills dashboard",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		if !isInteractive(false) {
			return fmt.Errorf("`huly skills tui` requires an interactive terminal")
		}
		return runDashboard()
	},
}
```

3. Register `skillsTUICmd` in `init()`'s `AddCommand` (making 5 subcommands total):

```go
	skillsCmd.AddCommand(skillsListCmd, skillsInstallCmd, skillsUpdateCmd, skillsUninstallCmd, skillsTUICmd)
```

- [ ] **Step 5: Run tests + build + lint**

Run: `cd src/huly && go build ./... && go test ./cmd/ -v 2>&1 | tail -20 && golangci-lint run ./cmd/... 2>&1 | tail -3`
Expected: build OK; `TestNoDuplicateSkillsSubcommands` now PASS (5); whole `cmd` suite PASS; lint 0.

- [ ] **Step 6: Non-TTY smoke (dashboard must not hang)**

Run:
```bash
cd src/huly
env HOME=$(mktemp -d) XDG_CONFIG_HOME=/nonexistent go run . skills < /dev/null
echo "bare-skills exit=$?"
env HOME=$(mktemp -d) XDG_CONFIG_HOME=/nonexistent go run . skills tui < /dev/null
echo "skills-tui exit=$?"
```
Expected: bare `huly skills` with non-TTY stdin prints the command help and exits 0 (not a hung form); `huly skills tui` errors "requires an interactive terminal" and exits non-zero. Neither hangs.

- [ ] **Step 7: Commit**

```bash
git add src/huly/cmd/skills_tui.go src/huly/cmd/skills.go src/huly/cmd/skills_test.go
git commit -m "feat(skills): bare-command dashboard + huly skills tui"
```

---

### Task 3: Docs update for the TUI

**Files:**
- Modify: `docs/skills.md`

**Interfaces:**
- Consumes: nothing.
- Produces: TUI documentation.

- [ ] **Step 1: Document the interactive surface**

In `docs/skills.md`, add a section after the "Selecting agents" section:

```markdown
## Interactive mode

At a terminal, `huly skills` with no subcommand opens an interactive dashboard
(pick skills → pick agents → pick an action). `huly skills tui` forces the
dashboard and errors if you're not on a terminal.

`install`/`update`/`uninstall` with no `--agents`/`--all` at a terminal open a
pre-checked agent picker, then a confirmation. Add `--yes` to skip the
confirmation, or `--no-interactive` to force the non-interactive behavior
(which then requires `--all` or `--agents`). In a pipe or script (no TTY) the
commands are always non-interactive and require an explicit selector.
```

Add `--yes` and `--no-interactive` rows to the flags table:

```markdown
| `--yes` | skip the interactive confirmation |
| `--no-interactive` | never prompt; require `--all`/`--agents` |
```

- [ ] **Step 2: Verify + commit**

Run: `cd src/huly && go build ./...` (sanity, no code changed) and confirm `docs/skills.md` renders (or `just docs::build` if available).

```bash
git add docs/skills.md
git commit -m "docs(skills): document the interactive TUI + --yes/--no-interactive"
```

---

## Self-Review

**Spec coverage (Phase C):**
- huh install picker (interactive, no selector) → Task 1. ✓
- `isInteractive` = stdin && stderr TTYs && !--no-interactive; batch fallback keeps Phase B behavior → Task 1. ✓
- `--yes`/`--no-interactive` flags → Task 1. ✓
- bare `huly skills` = dashboard when interactive, else help; `Args: NoArgs`; `huly skills tui` explicit entry (errors non-TTY) → Task 2. ✓
- Dashboard = skills → agents → action → apply → tokens → Task 2. ✓
- Docs → Task 3. ✓

Deferred / out of scope (unchanged): the exec-bit follow-up, `Catalog` is already memoized (A2 follow-up done in B), the JSON-schema doc note.

**Placeholder scan:** no TBD/TODO. The only "verify against the pinned version" note is the huh API shape in Task 1 Step 4 — the code given matches huh's stable API; the note tells the implementer to `go doc` if the pinned version differs (huh is a live external dep, so this is diligence, not a placeholder).

**Type consistency:** `wantInteractive`/`isInteractive`/`pickAgents`/`confirmApply`/`runDashboard`/`skillNames` names are consistent across `skills_tui.go`, `skills.go`, `skills_run.go`. `runSkillsOp` reuses `presentAgents`/`presentIDs`/`resolveAgents`/`renderResults`/`exitError` from Phase B and `skills.Install/Update/Uninstall`/`InstallOpts`/`Result` from A2, all with their real signatures. The dashboard uses `version.Version` for `CurrentVersion`, matching `runSkillsOp`.

**Testability note:** huh forms require a TTY and are validated by the non-TTY smoke tests (they must fall back, never hang) plus the pure `wantInteractive` truth-table. This matches the house rule that interactive forms aren't unit-tested while their gating logic is. A reviewer should confirm every huh `form.Run()` is only reachable when `isInteractive()` is true.
