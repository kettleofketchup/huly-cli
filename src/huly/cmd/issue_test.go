package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
)

func TestCreateIssueSendsAttachedDocTx(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var gotTx map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/tx/") {
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &gotTx)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer srv.Close()
	rc := huly.NewRestClient(srv.URL, "ws", "tok")

	id, err := createIssue(context.Background(), rc, "p1", "acc-1", "Fix bug",
		issueOpts{Priority: "High", Description: "details"})
	if err != nil || id == "" {
		t.Fatalf("create = %q err=%v", id, err)
	}
	if gotTx["objectClass"] != huly.ClassIssue {
		t.Fatalf("objectClass = %v", gotTx["objectClass"])
	}
	attrs := gotTx["attributes"].(map[string]any)
	if attrs["attachedTo"] != huly.IDNoParent {
		t.Fatalf("attachedTo = %v", attrs["attachedTo"])
	}
	if attrs["collection"] != huly.CollectionSubIssues {
		t.Fatalf("collection = %v", attrs["collection"])
	}
	if int(attrs["priority"].(float64)) != int(huly.High) {
		t.Fatalf("priority = %v", attrs["priority"])
	}
}

func TestCreateIssueBadPriority(t *testing.T) {
	rc := huly.NewRestClient("http://unused", "ws", "tok")
	_, err := createIssue(context.Background(), rc, "p1", "acc", "t", issueOpts{Priority: "Bogus"})
	if err == nil {
		t.Fatal("expected bad priority error")
	}
}
