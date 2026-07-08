package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	// --agents csv, present — assert WHICH agents, not just the count (2 would
	// also match a broken impl that ignores the selector and returns all).
	sel, err := resolveAgents(detected, "claude,pi", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sel) != 2 || sel[0].ID != "claude" || sel[1].ID != "pi" {
		t.Errorf("csv resolved = %+v, want [claude pi]", sel)
	}
	// --agents naming an ABSENT agent -> error
	if _, err := resolveAgents(detected, "codex", false); err == nil {
		t.Error("selecting an absent agent should error")
	}
	// --agents naming a wholly UNKNOWN id -> error
	if _, err := resolveAgents(detected, "bogus", false); err == nil {
		t.Error("selecting an unknown agent id should error")
	}
	// whitespace in the csv is trimmed
	trimmed, err := resolveAgents(detected, " claude , pi ", false)
	if err != nil || len(trimmed) != 2 || trimmed[0].ID != "claude" || trimmed[1].ID != "pi" {
		t.Errorf("whitespace csv resolved = %+v (err %v), want [claude pi]", trimmed, err)
	}
	// a csv with no real ids errors (doesn't silently resolve to nothing)
	if _, err := resolveAgents(detected, " , ,", false); err == nil {
		t.Error("comma-only csv should error")
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

func TestResolveAgentsAllAndAgentsConflict(t *testing.T) {
	detected := []skills.Agent{{ID: "claude", Present: true}, {ID: "pi", Present: true}}
	if _, err := resolveAgents(detected, "claude", true); err == nil {
		t.Error("passing both --all and --agents should error")
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

func TestRenderResultsJSONFields(t *testing.T) {
	// A result with no reason must OMIT the reason key; path must be present.
	results := []skills.Result{
		{Skill: "s", Agent: "claude", Path: "/home/x/.claude/skills/s", Status: skills.StatusInstalled},
	}
	var buf bytes.Buffer
	if err := renderResults(&buf, results, true); err != nil {
		t.Fatal(err)
	}
	var raw []map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("json invalid: %v\n%s", err, buf.String())
	}
	if _, present := raw[0]["reason"]; present {
		t.Errorf("empty reason should be omitted:\n%s", buf.String())
	}
	if _, present := raw[0]["path"]; !present {
		t.Errorf("path field missing:\n%s", buf.String())
	}
	var typed []map[string]any
	_ = json.Unmarshal(buf.Bytes(), &typed)
	if typed[0]["path"] != "/home/x/.claude/skills/s" {
		t.Errorf("path = %v", typed[0]["path"])
	}
}

func TestExitError(t *testing.T) {
	clean := []skills.Result{{Status: skills.StatusInstalled}}
	conflict := []skills.Result{{Status: skills.StatusConflict, Reason: "modified"}}

	if err := exitError(clean, false, false); err != nil {
		t.Errorf("clean run should exit 0, got %v", err)
	}
	if err := exitError(clean, false, true); err != nil {
		t.Errorf("--fail-on-conflict with no conflict should exit 0, got %v", err)
	}
	if err := exitError(conflict, false, false); err != nil {
		t.Errorf("conflict without --fail-on-conflict is a policy skip (exit 0), got %v", err)
	}
	if err := exitError(conflict, false, true); err == nil {
		t.Error("conflict with --fail-on-conflict should be non-zero")
	}
	if err := exitError(clean, true, false); err == nil {
		t.Error("a failed target must be non-zero regardless of --fail-on-conflict")
	}
}

func TestListLabelDefaultBranch(t *testing.T) {
	for _, s := range []skills.Status{skills.StatusRepaired, skills.StatusRemoved, skills.StatusSkipped} {
		if got := listLabel(skills.Result{Status: s}); got != string(s) {
			t.Errorf("listLabel(%s) = %q, want %q (default passthrough)", s, got, string(s))
		}
	}
}

func TestListLabelAdoptedIsInstalled(t *testing.T) {
	got := listLabel(skills.Result{Status: skills.StatusUpdated, Reason: "adopted"})
	if got != "installed" {
		t.Errorf("adopted should read as installed, got %q", got)
	}
}

// End-to-end wiring: drive the engine through a t.TempDir()-based agent (using
// the pure DetectAgents(Dirs)) and render the results — catches pairing/format
// bugs the isolated unit tests can't.
func TestSkillsInstallUpdateUninstallEndToEnd(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	present := presentAgents(skills.DetectAgents(skills.Dirs{Home: tmp, ConfigHome: tmp}))
	if len(present) != 1 || present[0].ID != "claude" {
		t.Fatalf("expected only claude present, got %+v", present)
	}
	cat, err := skills.Catalog()
	if err != nil || len(cat) == 0 {
		t.Fatalf("catalog: %v", err)
	}
	sk, opts := cat[0], skills.InstallOpts{CurrentVersion: "test"}

	r, err := skills.Install(sk, present[0], opts)
	if err != nil || r.Status != skills.StatusInstalled {
		t.Fatalf("install: status=%s err=%v", r.Status, err)
	}
	r2, err := skills.Update(sk, present[0], opts)
	if err != nil || r2.Status != skills.StatusUpToDate {
		t.Fatalf("update: status=%s err=%v", r2.Status, err)
	}
	var buf bytes.Buffer
	if err := renderResults(&buf, []skills.Result{r, r2}, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "installed") || !strings.Contains(buf.String(), "up-to-date") {
		t.Errorf("render missing tokens:\n%s", buf.String())
	}
	r3, err := skills.Uninstall(sk, present[0], opts)
	if err != nil || r3.Status != skills.StatusRemoved {
		t.Fatalf("uninstall: status=%s err=%v", r3.Status, err)
	}
}

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
	if got, _ := completeSkills(nil, nil, "zzz"); len(got) != 0 {
		t.Errorf("prefix zzz should match nothing, got %v", got)
	}

	ag, _ := completeAgents(nil, nil, "co")
	if len(ag) != 1 || ag[0] != "codex" {
		t.Errorf("completeAgents(co) = %v, want [codex]", ag)
	}
}

// Guards the init()-append bug: skills has exactly 4 subcommands
// (list, install, update, uninstall). A duplicate init() would double them.
func TestNoDuplicateSkillsSubcommands(t *testing.T) {
	if n := len(skillsCmd.Commands()); n != 4 {
		names := make([]string, 0, n)
		for _, c := range skillsCmd.Commands() {
			names = append(names, c.Name())
		}
		t.Errorf("skills has %d subcommands %v, want 4 (duplicate init()?)", n, names)
	}
}

// Spec §7 criterion 2: every `huly …` command shown in the seed skill's code
// spans must resolve to a real leaf command, so a renamed/removed command
// breaks CI instead of shipping a lying skill.
func TestSeedSkillCommandsExist(t *testing.T) {
	raw, err := os.ReadFile("../internal/skills/assets/huly-issue-tracking/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	spans := codeSpanText(string(raw))
	// huly <verb...> up to the first flag/placeholder (regex stops at non
	// [a-z-] tokens, i.e. --flags, <ID>, [x], quotes, uppercase).
	re := regexp.MustCompile(`huly ([a-z][a-z-]*(?: [a-z][a-z-]*)*)`)
	seen := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(spans, -1) {
		path := strings.Fields(m[1])
		c, rest, err := rootCmd.Find(path)
		if err != nil || len(rest) != 0 || c.HasSubCommands() {
			t.Errorf("seed references `huly %s` which is not a real leaf command (err=%v rest=%v)",
				strings.Join(path, " "), err, rest)
		}
		seen[strings.Join(path, " ")] = true
	}
	for _, required := range []string{"project list", "component create", "issue create"} {
		if !seen[required] {
			t.Errorf("seed skill must document `huly %s` in a code span", required)
		}
	}
}

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

// codeSpanText returns only the text inside fenced ``` blocks and inline
// `code` spans, so prose (e.g. "run huly to …") never yields false commands.
func codeSpanText(md string) string {
	var b strings.Builder
	inFence := false
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	for _, m := range regexp.MustCompile("`([^`]*)`").FindAllStringSubmatch(md, -1) {
		b.WriteString(m[1])
		b.WriteByte('\n')
	}
	return b.String()
}
