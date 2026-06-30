package cmd

import (
	"testing"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/cache"
)

func seedCache(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_ = cache.Save(cache.Cache{
		Projects:   []cache.ProjectRec{{Identifier: "PROJ"}, {Identifier: "OPS"}},
		Issues:     []cache.IssueRec{{Identifier: "PROJ-1"}, {Identifier: "PROJ-2"}},
		Components: []cache.ComponentRec{{Project: "PROJ", Label: "api"}},
	})
}

func TestCompleteProjects(t *testing.T) {
	seedCache(t)
	got, _ := completeProjects(nil, nil, "")
	if len(got) != 2 {
		t.Fatalf("projects = %v", got)
	}
}

func TestCompleteIssuesPrefix(t *testing.T) {
	seedCache(t)
	got, _ := completeIssues(nil, nil, "PROJ-1")
	if len(got) != 1 || got[0] != "PROJ-1" {
		t.Fatalf("issues = %v", got)
	}
}

func TestCompletePriorities(t *testing.T) {
	got, _ := completePriorities(nil, nil, "")
	if len(got) != 5 {
		t.Fatalf("priorities = %v", got)
	}
}
