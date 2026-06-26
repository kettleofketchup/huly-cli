package huly

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRestFindAllAndTx(t *testing.T) {
	var gotTx map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok-ws" {
			t.Errorf("missing bearer: %q", r.Header.Get("Authorization"))
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/find-all/"):
			if r.URL.Query().Get("class") != ClassProject {
				t.Errorf("class param = %q", r.URL.Query().Get("class"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"value": []map[string]any{{"_id": "p1", "identifier": "PROJ", "name": "Proj"}},
				"total": 1,
			})
		case strings.HasPrefix(r.URL.Path, "/api/v1/tx/"):
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &gotTx)
			_ = json.NewEncoder(w).Encode(map[string]any{})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewRestClient(srv.URL, "ws-uuid", "tok-ws")
	var projs []Project
	total, err := c.FindAll(context.Background(), ClassProject, nil, nil, &projs)
	if err != nil || total != 1 || len(projs) != 1 || projs[0].Identifier != "PROJ" {
		t.Fatalf("findAll = %v total=%d err=%v", projs, total, err)
	}
	tx := NewCreateDocTx(ClassMilestone, "p1", map[string]any{"label": "v1"}, "acc-1", 1)
	if err := c.Tx(context.Background(), tx); err != nil {
		t.Fatalf("tx err: %v", err)
	}
	if gotTx["objectClass"] != ClassMilestone {
		t.Fatalf("server saw objectClass=%v", gotTx["objectClass"])
	}
}

func TestRestUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := NewRestClient(srv.URL, "ws", "bad")
	var out []Project
	_, err := c.FindAll(context.Background(), ClassProject, nil, nil, &out)
	if err != ErrUnauthorized {
		t.Fatalf("err = %v want ErrUnauthorized", err)
	}
}
