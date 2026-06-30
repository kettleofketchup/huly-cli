package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kettleofketchup/huly-cli/src/huly/internal/huly"
)

func TestListProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{
				{"_id": "p1", "identifier": "PROJ", "name": "Proj"},
				{"_id": "p2", "identifier": "OPS", "name": "Ops"},
			},
			"total": 2,
		})
	}))
	defer srv.Close()
	rc := huly.NewRestClient(srv.URL, "ws", "tok")
	ps, err := listProjects(context.Background(), rc)
	if err != nil || len(ps) != 2 {
		t.Fatalf("projects = %v err=%v", ps, err)
	}
}
