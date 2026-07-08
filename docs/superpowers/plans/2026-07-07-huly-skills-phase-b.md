# huly skills — Phase B (non-interactive CLI) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the A2 skills engine into a `huly skills` Cobra command group (`list`/`install`/`update`/`uninstall`) with greppable + JSON output, defined exit codes, shell completion, a post-`huly update` staleness hint, and a docs page.

**Architecture:** New `src/huly/cmd/skills.go` (+ helpers/tests) drives the `internal/skills` engine built in A1/A2. The engine stays UI-free; the CLI resolves target skills/agents, calls `Install`/`Update`/`Uninstall`, and renders `[]skills.Result`. Pure resolution/label/render helpers are split out so they're unit-testable without touching the real `$HOME`. Bare `huly skills` prints help (no `RunE`), matching `huly issue`/`huly config`; the interactive TUI arrives in Phase C. Spec: `docs/superpowers/specs/2026-07-07-huly-skills-distribution-design.md` (§5).

**Tech Stack:** Go 1.25, `spf13/cobra`, `spf13/viper` (both already used across `cmd/`), the `internal/skills` + `internal/output` + `version` packages.

## Global Constraints

- Module path `github.com/kettleofketchup/huly-cli/src/huly`; module root `src/huly/`. CLI code in package `cmd` at `src/huly/cmd/`.
- Follow house command conventions (verified in `cmd/issue.go`, `cmd/config.go`, `cmd/root.go`): each command self-registers in `init()` via `rootCmd.AddCommand`; leaf commands use `RunE`; a parent grouping command is a one-line `&cobra.Command{Use, Short}` **with no `RunE`** (prints help). `--output` is a global viper-bound flag read via `viper.GetString("output") == "json"`.
- **Engine interfaces consumed** (package `skills`): `Catalog() ([]Skill, error)`, `Get(name string) (Skill, bool)`, `Skill{Name, Description}` (exported fields only), `Detect() ([]Agent, error)`, `Agent{ID, Label, RootDir, SkillsDir string; Present bool}`, `Install/Update/Uninstall(sk Skill, ag Agent, o InstallOpts) (Result, error)`, `InstallOpts{CurrentVersion string; Force, DryRun bool}`, `Result{Skill, Agent, Path string; Status Status; Reason string}`, and the `Status` constants `StatusInstalled/StatusUpdated/StatusRepaired/StatusUpToDate/StatusConflict/StatusRemoved/StatusSkipped`.
- **Output helpers** (package `output`): `Table(w io.Writer, headers []string, rows [][]string)`, `JSON(w io.Writer, v any) error`. Current version comes from `version.Version`.
- **Status token set** = the seven engine Statuses, plus a CLI-only `error` token for a target whose engine call returned a Go error. Text output prefixes each result line with the stable ASCII token so `… | grep conflict` works.
- **Exit codes:** `0` when the run completed, including targets skipped by policy (`up-to-date`/`conflict`/`removed`/`skipped`), which are reported via tokens. Non-zero only when a target's engine call errored, OR `--fail-on-conflict` was set and any target ended `conflict`. `--dry-run` uses the same exit logic (so `--dry-run --fail-on-conflict` is a real preview gate).
- **Agent selection (Phase B has no picker):** `install`/`update`/`uninstall` require `--all` or `--agents <csv>`; with neither, error listing the detected present agents (never guess). `list` needs no selector (it shows all present agents).
- Bare `huly skills` = help (no `RunE`); the TUI `RunE` and `--yes`/`--no-interactive` flags are Phase C, not here.
- TDD; commit per task; commit style `feat(skills): …`; no Claude watermark/co-author lines.
- Test commands: `just go::test huly` or `cd src/huly && go test ./...`; targeted `cd src/huly && go test ./cmd/ -run <Name> -v` (raw `go test` allow-listed).

## File Structure

- `src/huly/internal/skills/catalog.go` — MODIFY: memoize `Catalog()` (A2 review follow-up; Phase B loops over it).
- `src/huly/cmd/skills.go` — the `skills` group + `list`/`install`/`update`/`uninstall` commands, flag vars, `init()`.
- `src/huly/cmd/skills_run.go` — pure helpers: `resolveTargetSkills`, `resolveAgents`, `presentAgents`, `presentIDs`, `listLabel`, `renderResults`, `anyConflict`, `noAgentsMessage`, `completeSkills`, `completeAgents`.
- `src/huly/cmd/skills_test.go` — tests for the pure helpers.
- `src/huly/cmd/update.go` — MODIFY: one-line post-update staleness hint.
- `docs/skills.md` + `zensical.toml` — docs page + nav entry.

---

### Task 1: Memoize `Catalog()` + `skills` group + `list` command

**Files:**
- Modify: `src/huly/internal/skills/catalog.go`
- Create: `src/huly/cmd/skills.go`
- Create: `src/huly/cmd/skills_run.go`
- Create: `src/huly/cmd/skills_test.go`

**Interfaces:**
- Consumes: engine `Catalog`, `Detect`, `Install` (DryRun), `Agent`, `Result`, `Status` constants; `output.Table`/`JSON`; `version.Version`.
- Produces: `skillsCmd`, `skillsListCmd`; pure helpers `presentAgents([]skills.Agent) []skills.Agent`, `presentIDs([]skills.Agent) []string`, `noAgentsMessage([]skills.Agent) string`, `listLabel(skills.Result) string`.

- [ ] **Step 1: Write the failing test**

Create `src/huly/cmd/skills_test.go`:

```go
package cmd

import (
	"testing"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/skills"
)

func TestListLabel(t *testing.T) {
	cases := []struct {
		status skills.Status
		reason string
		want   string
	}{
		{skills.StatusInstalled, "", "not installed"},
		{skills.StatusUpToDate, "", "installed"},
		{skills.StatusUpdated, "", "update available"},
		{skills.StatusConflict, "modified", "modified"},
		{skills.StatusConflict, "foreign", "conflict (foreign)"},
		{skills.StatusConflict, "unreadable", "conflict (unreadable)"},
	}
	for _, c := range cases {
		got := listLabel(skills.Result{Status: c.status, Reason: c.reason})
		if got != c.want {
			t.Errorf("listLabel(%s/%s) = %q, want %q", c.status, c.reason, got, c.want)
		}
	}
}

func TestPresentAgents(t *testing.T) {
	agents := []skills.Agent{
		{ID: "claude", Present: true},
		{ID: "codex", Present: false},
		{ID: "pi", Present: true},
	}
	present := presentAgents(agents)
	if len(present) != 2 {
		t.Fatalf("got %d present, want 2", len(present))
	}
	if ids := presentIDs(present); ids[0] != "claude" || ids[1] != "pi" {
		t.Errorf("presentIDs = %v", ids)
	}
}

func TestNoAgentsMessageListsLabels(t *testing.T) {
	msg := noAgentsMessage([]skills.Agent{{Label: "Claude Code"}, {Label: "Codex"}})
	if !contains(msg, "Claude Code") || !contains(msg, "Codex") {
		t.Errorf("message should name agents: %q", msg)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./cmd/ -run 'TestListLabel|TestPresentAgents|TestNoAgents' -v`
Expected: FAIL — `listLabel`/`presentAgents`/`presentIDs`/`noAgentsMessage` undefined.

- [ ] **Step 3: Memoize `Catalog()`**

In `src/huly/internal/skills/catalog.go`: add `"sync"` to the imports, rename the existing `Catalog` function body to an unexported `loadCatalog`, and add a memoized `Catalog`:

```go
var (
	catalogOnce sync.Once
	catalogVal  []Skill
	catalogErr  error
)

// Catalog returns the embedded skills, parsing the embedded FS once.
func Catalog() ([]Skill, error) {
	catalogOnce.Do(func() { catalogVal, catalogErr = loadCatalog() })
	return catalogVal, catalogErr
}

// loadCatalog walks assets/ and parses each SKILL.md frontmatter.
func loadCatalog() ([]Skill, error) {
	// ... the EXACT original body of the old Catalog() ...
}
```

Do not change `loadCatalog`'s logic — only move the old body into it verbatim and add the `sync.Once` wrapper above.

- [ ] **Step 4: Write `skills_run.go` helpers**

Create `src/huly/cmd/skills_run.go`:

```go
package cmd

import (
	"strings"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/skills"
)

// presentAgents returns only the agents whose root dir exists.
func presentAgents(agents []skills.Agent) []skills.Agent {
	var out []skills.Agent
	for _, a := range agents {
		if a.Present {
			out = append(out, a)
		}
	}
	return out
}

// presentIDs returns the ids of the given agents.
func presentIDs(agents []skills.Agent) []string {
	out := make([]string, 0, len(agents))
	for _, a := range agents {
		out = append(out, a.ID)
	}
	return out
}

// noAgentsMessage explains that no supported agent was detected.
func noAgentsMessage(agents []skills.Agent) string {
	labels := make([]string, 0, len(agents))
	for _, a := range agents {
		labels = append(labels, a.Label)
	}
	return "No supported coding agents detected (looked for " +
		strings.Join(labels, ", ") + "). Install one (or create its config dir) and retry."
}

// listLabel maps a DryRun Install result to a current-state label for `list`.
// A DryRun install of an absent skill reports StatusInstalled ("would
// install") — for a status view that means the skill is not there yet.
func listLabel(r skills.Result) string {
	switch r.Status {
	case skills.StatusInstalled:
		return "not installed"
	case skills.StatusUpToDate:
		return "installed"
	case skills.StatusUpdated:
		return "update available"
	case skills.StatusConflict:
		if r.Reason == "modified" {
			return "modified"
		}
		return "conflict (" + r.Reason + ")"
	default:
		return string(r.Status)
	}
}
```

- [ ] **Step 5: Write `skills.go` (group + list)**

Create `src/huly/cmd/skills.go`:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/output"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/skills"
	"github.com/kettleofketchup/huly-cli/src/huly/version"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Install and manage huly's embedded agent skills",
	Long: `Install huly's embedded agent skills into the coding agents on your
machine (Claude Code, Codex, opencode, Cursor, Pi).

Run 'huly skills list' to see status, and 'huly skills install --all' to add
them to every detected agent.`,
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show embedded skills and their install status per agent",
	RunE:  runSkillsList,
}

func runSkillsList(_ *cobra.Command, _ []string) error {
	cat, err := skills.Catalog()
	if err != nil {
		return err
	}
	agents, err := skills.Detect()
	if err != nil {
		return err
	}
	present := presentAgents(agents)
	if len(present) == 0 {
		fmt.Fprintln(os.Stderr, noAgentsMessage(agents))
		return nil
	}

	type listRow struct {
		Skill  string `json:"skill"`
		Agent  string `json:"agent"`
		Status string `json:"status"`
	}
	var rows []listRow
	anyInstalled := false
	for _, sk := range cat {
		for _, ag := range present {
			r, err := skills.Install(sk, ag, skills.InstallOpts{CurrentVersion: version.Version, DryRun: true})
			if err != nil {
				return err
			}
			label := listLabel(r)
			if label != "not installed" {
				anyInstalled = true
			}
			rows = append(rows, listRow{sk.Name, ag.ID, label})
		}
	}

	if viper.GetString("output") == "json" {
		return output.JSON(os.Stdout, rows)
	}
	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		table = append(table, []string{r.Skill, r.Agent, r.Status})
	}
	output.Table(os.Stdout, []string{"SKILL", "AGENT", "STATUS"}, table)
	if !anyInstalled {
		fmt.Fprintln(os.Stderr, "\nNo skills installed yet. Run `huly skills install --all` to add them.")
	}
	return nil
}

func init() {
	skillsCmd.AddCommand(skillsListCmd)
	rootCmd.AddCommand(skillsCmd)
}
```

- [ ] **Step 6: Run tests + build**

Run: `cd src/huly && go build ./... && go test ./cmd/ -run 'TestListLabel|TestPresentAgents|TestNoAgents' -v`
Expected: build OK; the three tests PASS.

- [ ] **Step 7: Smoke-test `list` manually**

Run: `cd src/huly && go run . skills list`
Expected: a `SKILL  AGENT  STATUS` table listing `huly-issue-tracking` against each detected agent (statuses like `not installed`/`installed`/`update available`), or the "No supported coding agents detected" line if none. No panic, exit 0.

- [ ] **Step 8: Commit**

```bash
git add src/huly/internal/skills/catalog.go src/huly/cmd/skills.go src/huly/cmd/skills_run.go src/huly/cmd/skills_test.go
git commit -m "feat(skills): skills group + list command; memoize catalog"
```

---

### Task 2: `install` / `update` / `uninstall` commands

**Files:**
- Modify: `src/huly/cmd/skills.go` (add the three commands + flag vars + register)
- Modify: `src/huly/cmd/skills_run.go` (add `resolveTargetSkills`, `resolveAgents`, `renderResults`, `anyConflict`, `runSkillsOp`)
- Modify: `src/huly/cmd/skills_test.go` (add resolution + render + exit tests)

**Interfaces:**
- Consumes: Task 1 helpers; engine `Install/Update/Uninstall`, `InstallOpts`, `Get`, `Catalog`, `Detect`.
- Produces: `resolveTargetSkills([]string) ([]skills.Skill, error)`, `resolveAgents(detected []skills.Agent, agentsCSV string, all bool) ([]skills.Agent, error)`, `renderResults(w io.Writer, results []skills.Result, jsonOut bool) error`, `anyConflict([]skills.Result) bool`, `runSkillsOp(op string, args []string) error`.

- [ ] **Step 1: Write the failing test**

Add to `src/huly/cmd/skills_test.go` (add imports `bytes`, `strings`, `encoding/json`):

```go
func TestResolveTargetSkills(t *testing.T) {
	all, err := resolveTargetSkills(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) == 0 {
		t.Fatal("no skills in catalog")
	}
	one, err := resolveTargetSkills([]string{"huly-issue-tracking"})
	if err != nil {
		t.Fatal(err)
	}
	if len(one) != 1 || one[0].Name != "huly-issue-tracking" {
		t.Errorf("resolved = %+v", one)
	}
	if _, err := resolveTargetSkills([]string{"nope"}); err == nil {
		t.Error("unknown skill should error")
	}
}

func TestResolveAgents(t *testing.T) {
	detected := []skills.Agent{
		{ID: "claude", Label: "Claude Code", Present: true},
		{ID: "codex", Label: "Codex", Present: false},
		{ID: "pi", Label: "Pi", Present: true},
	}
	// --all -> present only
	all, err := resolveAgents(detected, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("--all resolved %d, want 2 present", len(all))
	}
	// --agents csv, present
	sel, err := resolveAgents(detected, "claude,pi", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sel) != 2 {
		t.Errorf("csv resolved %d", len(sel))
	}
	// --agents naming an ABSENT agent -> error
	if _, err := resolveAgents(detected, "codex", false); err == nil {
		t.Error("selecting an absent agent should error")
	}
	// neither --all nor --agents -> error
	if _, err := resolveAgents(detected, "", false); err == nil {
		t.Error("no selector should error")
	}
	// no present agents -> error
	if _, err := resolveAgents([]skills.Agent{{ID: "claude", Present: false}}, "", true); err == nil {
		t.Error("no present agents should error")
	}
}

func TestRenderResultsAndConflict(t *testing.T) {
	results := []skills.Result{
		{Skill: "s", Agent: "claude", Path: "/p", Status: skills.StatusInstalled},
		{Skill: "s", Agent: "codex", Path: "/q", Status: skills.StatusConflict, Reason: "modified"},
	}

	// text: greppable ASCII tokens
	var text bytes.Buffer
	if err := renderResults(&text, results, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text.String(), "installed") || !strings.Contains(text.String(), "conflict") {
		t.Errorf("text render missing tokens:\n%s", text.String())
	}
	if !strings.Contains(text.String(), "(modified)") {
		t.Errorf("reason not shown:\n%s", text.String())
	}

	// json: array of {skill,agent,status,path,reason}
	var jsonBuf bytes.Buffer
	if err := renderResults(&jsonBuf, results, true); err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(jsonBuf.Bytes(), &got); err != nil {
		t.Fatalf("json invalid: %v\n%s", err, jsonBuf.String())
	}
	if len(got) != 2 || got[1]["status"] != "conflict" || got[1]["reason"] != "modified" {
		t.Errorf("json shape wrong: %+v", got)
	}

	if !anyConflict(results) {
		t.Error("anyConflict should be true")
	}
	if anyConflict(results[:1]) {
		t.Error("anyConflict should be false without a conflict")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd src/huly && go test ./cmd/ -run 'TestResolve|TestRenderResults' -v`
Expected: FAIL — `resolveTargetSkills`/`resolveAgents`/`renderResults`/`anyConflict` undefined.

- [ ] **Step 3: Add helpers to `skills_run.go`**

Append to `src/huly/cmd/skills_run.go` (add imports `encoding/json`, `fmt`, `io`; keep `strings`):

```go
// resolveTargetSkills returns the catalog skills named by args, or ALL catalog
// skills when args is empty. An unknown name is an error.
func resolveTargetSkills(args []string) ([]skills.Skill, error) {
	if len(args) == 0 {
		return skills.Catalog()
	}
	out := make([]skills.Skill, 0, len(args))
	for _, name := range args {
		sk, ok := skills.Get(name)
		if !ok {
			return nil, fmt.Errorf("unknown skill %q (run `huly skills list`)", name)
		}
		out = append(out, sk)
	}
	return out, nil
}

// resolveAgents picks the target agents from a detected set. It is pure so it
// can be tested without touching $HOME: the caller passes skills.Detect()'s
// result. With all=true it returns every PRESENT agent; with agentsCSV it
// returns the named present agents (erroring on an unknown or absent id);
// with neither it errors listing the present agents.
func resolveAgents(detected []skills.Agent, agentsCSV string, all bool) ([]skills.Agent, error) {
	present := presentAgents(detected)
	if len(present) == 0 {
		return nil, fmt.Errorf("%s", noAgentsMessage(detected))
	}
	if all {
		return present, nil
	}
	if agentsCSV != "" {
		byID := make(map[string]skills.Agent, len(present))
		for _, a := range present {
			byID[a.ID] = a
		}
		var out []skills.Agent
		for _, raw := range strings.Split(agentsCSV, ",") {
			id := strings.TrimSpace(raw)
			if id == "" {
				continue
			}
			a, ok := byID[id]
			if !ok {
				return nil, fmt.Errorf("agent %q is not detected/installed; detected: %s",
					id, strings.Join(presentIDs(present), ", "))
			}
			out = append(out, a)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("no agents named in --agents")
		}
		return out, nil
	}
	return nil, fmt.Errorf("select agents with --all or --agents <%s>",
		strings.Join(presentIDs(present), ","))
}

// renderResults writes the per-target outcomes as greppable ASCII token lines
// or, when jsonOut, as an array of {skill,agent,status,path,reason}.
func renderResults(w io.Writer, results []skills.Result, jsonOut bool) error {
	if jsonOut {
		type jr struct {
			Skill  string `json:"skill"`
			Agent  string `json:"agent"`
			Status string `json:"status"`
			Path   string `json:"path"`
			Reason string `json:"reason,omitempty"`
		}
		out := make([]jr, 0, len(results))
		for _, r := range results {
			out = append(out, jr{r.Skill, r.Agent, string(r.Status), r.Path, r.Reason})
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	for _, r := range results {
		reason := ""
		if r.Reason != "" {
			reason = " (" + r.Reason + ")"
		}
		fmt.Fprintf(w, "%-12s %s → %s%s\n", r.Status, r.Skill, r.Agent, reason)
	}
	return nil
}

// anyConflict reports whether any target ended in a conflict.
func anyConflict(results []skills.Result) bool {
	for _, r := range results {
		if r.Status == skills.StatusConflict {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Add the commands + `runSkillsOp` to `skills.go`**

In `src/huly/cmd/skills.go`, add the flag vars (package level) and commands, and register them. Add `"io"` is NOT needed here (renderResults lives in skills_run.go). Add near the top after the imports:

```go
var (
	skillsAgents         string
	skillsAll            bool
	skillsForce          bool
	skillsDryRun         bool
	skillsFailOnConflict bool
)

var skillsInstallCmd = &cobra.Command{
	Use:               "install [skill...]",
	Short:             "Install embedded skills into agents",
	ValidArgsFunction: completeSkills,
	RunE:              func(_ *cobra.Command, args []string) error { return runSkillsOp("install", args) },
}

var skillsUpdateCmd = &cobra.Command{
	Use:               "update [skill...]",
	Short:             "Update huly-owned skills that are behind",
	ValidArgsFunction: completeSkills,
	RunE:              func(_ *cobra.Command, args []string) error { return runSkillsOp("update", args) },
}

var skillsUninstallCmd = &cobra.Command{
	Use:               "uninstall [skill...]",
	Short:             "Remove huly-owned skills from agents",
	ValidArgsFunction: completeSkills,
	RunE:              func(_ *cobra.Command, args []string) error { return runSkillsOp("uninstall", args) },
}

// runSkillsOp resolves the target skills and agents, runs the engine op for
// every (skill, agent) pair, renders the results, and returns an error (=>
// non-zero exit) only on a genuine failure or an unforced conflict under
// --fail-on-conflict.
func runSkillsOp(op string, args []string) error {
	sks, err := resolveTargetSkills(args)
	if err != nil {
		return err
	}
	detected, err := skills.Detect()
	if err != nil {
		return err
	}
	agents, err := resolveAgents(detected, skillsAgents, skillsAll)
	if err != nil {
		return err
	}
	opts := skills.InstallOpts{CurrentVersion: version.Version, Force: skillsForce, DryRun: skillsDryRun}

	var results []skills.Result
	failed := false
	for _, sk := range sks {
		for _, ag := range agents {
			var r skills.Result
			var e error
			switch op {
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

	if err := renderResults(os.Stdout, results, viper.GetString("output") == "json"); err != nil {
		return err
	}
	if failed {
		return fmt.Errorf("one or more targets failed")
	}
	if skillsFailOnConflict && anyConflict(results) {
		return fmt.Errorf("conflicts detected (use --force to override, or resolve manually)")
	}
	return nil
}
```

Then extend the existing `init()` in `skills.go` to register the three commands and their flags:

```go
func init() {
	for _, c := range []*cobra.Command{skillsInstallCmd, skillsUpdateCmd, skillsUninstallCmd} {
		c.Flags().StringVar(&skillsAgents, "agents", "", "comma-separated agent ids: claude,codex,opencode,cursor,pi")
		c.Flags().BoolVar(&skillsAll, "all", false, "target every detected agent")
		c.Flags().BoolVar(&skillsForce, "force", false, "override conflicts (backs the old dir up first)")
		c.Flags().BoolVar(&skillsDryRun, "dry-run", false, "show what would change; write nothing")
		c.Flags().BoolVar(&skillsFailOnConflict, "fail-on-conflict", false, "exit non-zero if any target conflicts")
		_ = c.RegisterFlagCompletionFunc("agents", completeAgents)
	}
	skillsCmd.AddCommand(skillsListCmd, skillsInstallCmd, skillsUpdateCmd, skillsUninstallCmd)
	rootCmd.AddCommand(skillsCmd)
}
```

Note: the old `init()` from Task 1 registered only `skillsListCmd`; replace it with this fuller `init()` (there must be exactly ONE `init()` in `skills.go`). `completeSkills`/`completeAgents` are added in Task 3 — until then this won't compile, so Step 5 stubs them; Task 3 fills them in. To keep Task 2 self-contained, add minimal real implementations of `completeSkills`/`completeAgents` now (they're small) in `skills_run.go`:

```go
// completeSkills completes skill-name args from the catalog.
func completeSkills(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	cat, _ := skills.Catalog()
	out := make([]string, 0, len(cat))
	for _, s := range cat {
		out = append(out, s.Name)
	}
	return filterPrefix(out, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// completeAgents completes --agents values.
func completeAgents(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return filterPrefix([]string{"claude", "codex", "opencode", "cursor", "pi"}, toComplete),
		cobra.ShellCompDirectiveNoFileComp
}
```

(Add `"github.com/spf13/cobra"` to `skills_run.go`'s imports for these. `filterPrefix` already exists in `cmd/completion_funcs.go`.)

- [ ] **Step 5: Run tests + build**

Run: `cd src/huly && go build ./... && go test ./cmd/ -run 'TestResolve|TestRenderResults' -v`
Expected: build OK; tests PASS.

- [ ] **Step 6: Smoke-test end to end into a temp agent**

Run:
```bash
cd src/huly
TMP=$(mktemp -d)
HOME=$TMP mkdir -p "$TMP/.claude"
HOME=$TMP go run . skills install --all
HOME=$TMP go run . skills list
HOME=$TMP go run . skills update --all
HOME=$TMP go run . skills uninstall --all
```
Expected: install prints `installed  huly-issue-tracking → claude`; the skill dir appears at `$TMP/.claude/skills/huly-issue-tracking/SKILL.md`; list shows `installed`; second update shows `up-to-date`; uninstall shows `removed`. Exit 0 throughout.

- [ ] **Step 7: Commit**

```bash
git add src/huly/cmd/skills.go src/huly/cmd/skills_run.go src/huly/cmd/skills_test.go
git commit -m "feat(skills): install/update/uninstall commands with flags, tokens, json, exit codes"
```

---

### Task 3: Shell completion registration + post-update staleness hint

**Files:**
- Modify: `src/huly/cmd/skills.go` (register `completeSkills` as ValidArgsFunction — already set on the commands in Task 2; this task adds an explicit test-backed check and the update hint)
- Modify: `src/huly/cmd/update.go`
- Modify: `src/huly/cmd/skills_test.go`

**Interfaces:**
- Consumes: `completeSkills`/`completeAgents` (Task 2).
- Produces: a completion smoke test; a post-update hint line in `runUpdate`.

- [ ] **Step 1: Write the failing test**

Add to `src/huly/cmd/skills_test.go`:

```go
func TestCompleteSkillsAndAgents(t *testing.T) {
	sk, _ := completeSkills(nil, nil, "")
	found := false
	for _, s := range sk {
		if s == "huly-issue-tracking" {
			found = true
		}
	}
	if !found {
		t.Errorf("completeSkills missing seed skill: %v", sk)
	}
	// prefix filtering works
	if got, _ := completeSkills(nil, nil, "zzz"); len(got) != 0 {
		t.Errorf("prefix zzz should match nothing, got %v", got)
	}

	ag, _ := completeAgents(nil, nil, "co")
	// "co" matches codex only
	if len(ag) != 1 || ag[0] != "codex" {
		t.Errorf("completeAgents(co) = %v, want [codex]", ag)
	}
}
```

- [ ] **Step 2: Run test to verify it fails or passes**

Run: `cd src/huly && go test ./cmd/ -run TestCompleteSkillsAndAgents -v`
Expected: PASS if Task 2 already added `completeSkills`/`completeAgents` (this test just pins their behavior). If it fails to compile, those funcs are missing — add them per Task 2 Step 4.

- [ ] **Step 3: Add the post-update staleness hint**

In `src/huly/cmd/update.go`, at the very end of `runUpdate()` (after the existing `"You may need to restart your terminal..."` line, before `return nil`), add:

```go
	fmt.Println("Installed agent skills may now be behind. Run `huly skills update` to refresh them.")
```

- [ ] **Step 4: Run tests + build**

Run: `cd src/huly && go build ./... && go test ./cmd/ -run TestCompleteSkillsAndAgents -v`
Expected: build OK; test PASS.

- [ ] **Step 5: Verify completion is wired**

Run: `cd src/huly && go run . __complete skills install ""`
Expected: output includes `huly-issue-tracking` (Cobra's completion for the skill-name arg). And `go run . __complete skills install --agents ""` includes `claude`.

- [ ] **Step 6: Commit**

```bash
git add src/huly/cmd/update.go src/huly/cmd/skills_test.go
git commit -m "feat(skills): completion checks + post-update staleness hint"
```

---

### Task 4: Docs page + nav entry

**Files:**
- Create: `docs/skills.md`
- Modify: `zensical.toml` (add a nav entry)

**Interfaces:**
- Consumes: nothing (docs).
- Produces: a user-facing docs page for `huly skills`.

- [ ] **Step 1: Inspect the existing nav format**

Run: `grep -n 'nav' zensical.toml` and read the surrounding array so the new entry matches the exact `{"Title" = "file.md"}` shape used by the other pages (e.g. auth.md, cache.md).

- [ ] **Step 2: Write the docs page**

Create `docs/skills.md`:

```markdown
# Agent skills

`huly skills` installs huly's embedded agent skills into the AI coding agents
on your machine, so they know how to use the huly CLI. The same native
`SKILL.md` format works across Claude Code, Codex, opencode, Cursor, and Pi.

## Commands

```sh
huly skills list                 # status of each skill per detected agent
huly skills install --all        # install into every detected agent
huly skills install --agents claude,codex
huly skills update --all         # refresh skills that are behind
huly skills uninstall --all      # remove huly-owned skills
```

## Selecting agents

`install`/`update`/`uninstall` require an explicit target:

- `--all` — every agent detected on your machine.
- `--agents <ids>` — a comma-separated subset (`claude,codex,opencode,cursor,pi`).

`huly skills list` shows every detected agent with no selector.

## Flags

| Flag | Meaning |
|------|---------|
| `--all` | target all detected agents |
| `--agents <csv>` | target the named agents |
| `--force` | overwrite a conflicting/edited/foreign skill (the old copy is backed up to `<dir>.bak-<n>` first) |
| `--dry-run` | print what would change; write nothing |
| `--fail-on-conflict` | exit non-zero if any target conflicts (for CI) |
| `--output json` | machine-readable results |

## How updates work

Each installed skill records the shipping huly version and a content hash in
its `SKILL.md` frontmatter. `huly skills update` re-installs a skill only when
the embedded content actually differs — it never clobbers a skill you edited
(that reports `conflict`/`modified`; use `--force`, which backs up first). After
`huly update` upgrades the binary, run `huly skills update` to refresh.

## Result tokens

`installed`, `updated`, `repaired`, `up-to-date`, `conflict`, `removed`,
`skipped` (and `error`). Text output prefixes each line with the token, so
`huly skills install --all | grep conflict` works.
```

- [ ] **Step 3: Add the nav entry**

In `zensical.toml`, add `{"Skills" = "skills.md"}` to the `nav` array, placed alongside the other command pages (match the exact quoting/spacing of the neighbours from Step 1).

- [ ] **Step 4: Verify the docs build**

Run: `just docs::build` (or `uvx zensical build` if that's the recipe). 
Expected: build succeeds, `public/skills/` (or the configured output) is produced, no nav errors. If the docs toolchain isn't available in the environment, instead verify the TOML parses: `cd . && python -c "import tomllib; tomllib.load(open('zensical.toml','rb'))"` and confirm `skills.md` is present in the nav array.

- [ ] **Step 5: Commit**

```bash
git add docs/skills.md zensical.toml
git commit -m "docs(skills): add huly skills page + nav entry"
```

---

## Self-Review

**Spec coverage (Phase B):**
- Bare `huly skills` = help (no `RunE`), matching house convention → Task 1. ✓
- `list` with per-agent status incl. "update available" + zero-state nudge + no-agents message + `--output json` → Task 1. ✓
- `install`/`update`/`uninstall` with `--agents`/`--all`/`--force`/`--dry-run`/`--fail-on-conflict`; explicit-selector-required (no picker in B) → Task 2. ✓
- ASCII status tokens + `--output json` for mutating commands + exit codes (0 on policy-skip, non-zero on error / `--fail-on-conflict`) → Task 2. ✓
- Completion (`completeSkills`/`completeAgents`) + post-`huly update` staleness hint → Tasks 2/3. ✓
- Docs page + nav → Task 4. ✓
- `Catalog()` memoization (A2 follow-up, needed because Phase B loops) → Task 1. ✓

Deferred to Phase C (correctly not here): the huh TUI, bare-command `RunE`, the interactive agent picker, `--yes`/`--no-interactive`.

**Placeholder scan:** The only intentional "copy the original body" reference is Task 1 Step 3's `loadCatalog` (the existing `Catalog` body is moved verbatim — the engineer has that file open). No TBD/TODO; every other step shows complete code.

**Type consistency:** `presentAgents`/`presentIDs`/`noAgentsMessage`/`listLabel` (Task 1) are reused by `resolveAgents`/`renderResults` (Task 2). `resolveTargetSkills`/`resolveAgents`/`renderResults`/`anyConflict`/`runSkillsOp` names match between `skills.go` and `skills_run.go`. `completeSkills`/`completeAgents` are defined once (Task 2, `skills_run.go`) and referenced by the command `ValidArgsFunction`/flag completion in `skills.go` and tested in Task 3. All engine calls use the real signatures from Global Constraints (`Install/Update/Uninstall(sk, ag, opts)`, `InstallOpts{CurrentVersion,Force,DryRun}`, `Result{Skill,Agent,Path,Status,Reason}`).

**Note on `list` classification:** it calls `Install(..., DryRun:true)` and remaps via `listLabel` because the embedded-hash computation (`fs.Sub(assetsFS,…)` + `(Skill).contentHash()`) is unexported — the CLI cannot classify current state without the engine. `DryRun` guarantees no writes. `StatusInstalled` from a DryRun means "would install" i.e. "not installed", which `listLabel` maps accordingly.
