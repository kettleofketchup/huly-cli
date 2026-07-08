package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

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

// skillNames returns the names of the given skills.
func skillNames(sks []skills.Skill) []string {
	out := make([]string, 0, len(sks))
	for _, s := range sks {
		out = append(out, s.Name)
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
		// A pre-hash "adopted" skill already has current content — only its
		// provenance stamp is refreshed — so it is installed, not behind.
		if r.Reason == "adopted" {
			return "installed"
		}
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
			return nil, fmt.Errorf("unknown skill %q; available: %s", name, strings.Join(catalogNames(), ", "))
		}
		out = append(out, sk)
	}
	return out, nil
}

// catalogNames returns the embedded skill names for error messages.
func catalogNames() []string {
	cat, _ := skills.Catalog()
	out := make([]string, 0, len(cat))
	for _, s := range cat {
		out = append(out, s.Name)
	}
	return out
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
	if all && agentsCSV != "" {
		return nil, fmt.Errorf("pass either --all or --agents, not both")
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
		_, _ = fmt.Fprintf(w, "%-12s %s → %s%s\n", r.Status, r.Skill, r.Agent, reason)
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

// exitError decides the CLI exit outcome from the per-target results: a
// genuine engine failure always fails the run; otherwise a conflict fails it
// only when --fail-on-conflict was requested. Everything else (policy skips)
// exits 0. Extracted so the exit logic is unit-testable without a TTY/$HOME.
func exitError(results []skills.Result, failed, failOnConflict bool) error {
	if failed {
		return fmt.Errorf("one or more targets failed")
	}
	if failOnConflict && anyConflict(results) {
		return fmt.Errorf("conflicts detected (use --force to override, or resolve manually)")
	}
	return nil
}

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
