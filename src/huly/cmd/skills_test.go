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
