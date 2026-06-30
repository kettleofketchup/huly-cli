package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
)

func TestResolveProjectRefLive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/find-all/") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": []map[string]any{{"_id": "p1", "identifier": "PROJ", "name": "Proj"}},
				"total": 1,
			})
		}
	}))
	defer srv.Close()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	rc := huly.NewRestClient(srv.URL, "ws", "tok")
	p, err := resolveProjectRef(context.Background(), rc, "PROJ")
	if err != nil || p.ID != "p1" {
		t.Fatalf("resolve = %+v err=%v", p, err)
	}
}

func TestResolveProjectRefMiss(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"value": []map[string]any{}, "total": 0})
	}))
	defer srv.Close()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	rc := huly.NewRestClient(srv.URL, "ws", "tok")
	_, err := resolveProjectRef(context.Background(), rc, "NOPE")
	if err == nil {
		t.Fatal("expected miss error")
	}
}
