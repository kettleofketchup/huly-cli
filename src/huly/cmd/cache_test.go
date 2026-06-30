package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
)

func TestSyncCachePopulatesProjectsAndIssues(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		class := r.URL.Query().Get("class")
		var value []map[string]any
		switch class {
		case huly.ClassProject:
			value = []map[string]any{{"_id": "p1", "identifier": "PROJ", "name": "Proj"}}
		case huly.ClassIssue:
			value = []map[string]any{{"_id": "i1", "identifier": "PROJ-1", "title": "x", "space": "p1"}}
		case huly.ClassComponent:
			value = []map[string]any{{"_id": "c1", "label": "api", "space": "p1"}}
		case huly.ClassMilestone:
			value = []map[string]any{{"_id": "m1", "label": "v1", "space": "p1"}}
		case huly.ClassIssueStatus:
			value = []map[string]any{{"_id": "s1", "name": "Todo", "space": "p1"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"value": value, "total": len(value)})
	}))
	defer srv.Close()

	rc := huly.NewRestClient(srv.URL, "ws", "tok")
	c, err := syncCache(context.Background(), rc, "")
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(c.Projects) != 1 || len(c.Issues) != 1 || len(c.Components) != 1 ||
		len(c.Milestones) != 1 || len(c.Statuses) != 1 {
		t.Fatalf("incomplete cache: %+v", c)
	}
	if c.Issues[0].Project != "PROJ" {
		t.Fatalf("issue project not mapped to identifier: %+v", c.Issues[0])
	}
}
