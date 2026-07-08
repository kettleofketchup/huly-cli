package skills

import (
	"os"
	"strings"
	"testing"
)

func TestSeedSkillAsset(t *testing.T) {
	raw, err := os.ReadFile("assets/huly-issue-tracking/SKILL.md")
	if err != nil {
		t.Fatalf("read seed SKILL.md: %v", err)
	}
	s := string(raw)

	// Criterion 1: frontmatter identity.
	if !strings.Contains(s, "name: huly-issue-tracking") {
		t.Error("frontmatter must declare name: huly-issue-tracking")
	}
	if !strings.Contains(s, "managed_by: huly-cli") {
		t.Error("frontmatter metadata must declare managed_by: huly-cli")
	}
	lower := strings.ToLower(s)
	if !strings.Contains(lower, "issue") || !strings.Contains(lower, "bug") {
		t.Error("description/body should mention issue and bug tracking")
	}

	// Criterion 3: required command set present (string level).
	for _, cmd := range []string{"huly project list", "huly component create", "huly issue create"} {
		if !strings.Contains(s, cmd) {
			t.Errorf("seed skill must reference %q", cmd)
		}
	}
}
