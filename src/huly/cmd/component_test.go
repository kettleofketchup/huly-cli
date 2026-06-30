package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/cache"
	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
)

func TestCreateComponentWritesThroughCache(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var gotTx map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/tx/") {
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &gotTx)
			_ = json.NewEncoder(w).Encode(map[string]any{})
		}
	}))
	defer srv.Close()

	rc := huly.NewRestClient(srv.URL, "ws", "tok")
	id, err := createComponent(context.Background(), rc, "p1", "PROJ", "acc-1", "api", "API layer", "")
	if err != nil || id == "" {
		t.Fatalf("create = %q err=%v", id, err)
	}
	if gotTx["objectClass"] != huly.ClassComponent {
		t.Fatalf("objectClass = %v", gotTx["objectClass"])
	}
	c, _ := cache.Load()
	found := false
	for _, x := range c.Components {
		if x.Label == "api" && x.ID == id {
			found = true
			if x.Project != "PROJ" {
				t.Fatalf("expected Project=PROJ, got %q", x.Project)
			}
		}
	}
	if !found {
		t.Fatalf("component not written through to cache: %+v", c.Components)
	}
}
