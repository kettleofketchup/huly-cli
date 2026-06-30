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

func TestCreateMilestone(t *testing.T) {
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
	id, err := createMilestone(context.Background(), rc, "p1", "acc-1", "v1.0", 0)
	if err != nil || id == "" {
		t.Fatalf("create = %q err=%v", id, err)
	}
	if gotTx["objectClass"] != huly.ClassMilestone {
		t.Fatalf("objectClass = %v", gotTx["objectClass"])
	}
}
