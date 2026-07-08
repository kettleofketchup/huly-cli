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
