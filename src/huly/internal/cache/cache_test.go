package cache

import (
	"testing"
)

func TestSaveLoadAndUpdate(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	c, err := Load() // missing file -> empty
	if err != nil {
		t.Fatalf("load empty: %v", err)
	}
	if len(c.Projects) != 0 {
		t.Fatalf("expected empty cache")
	}

	err = Update(func(c *Cache) {
		c.Projects = append(c.Projects, ProjectRec{ID: "p1", Identifier: "PROJ", Name: "Proj"})
		c.Issues = append(c.Issues, IssueRec{ID: "i1", Project: "PROJ", Identifier: "PROJ-1", Title: "x"})
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := Load()
	if len(got.Projects) != 1 || got.Projects[0].Identifier != "PROJ" {
		t.Fatalf("projects = %+v", got.Projects)
	}
	if len(got.Issues) != 1 || got.Issues[0].Identifier != "PROJ-1" {
		t.Fatalf("issues = %+v", got.Issues)
	}
}
