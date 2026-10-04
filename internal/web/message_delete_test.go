package web

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/store"
)

func TestMessageDeleteAuthSessionMethodsAndCascade(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "delete.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, id := range []string{"a", "b"} {
		if _, err := db.CreateSession(t.Context(), id, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.AddMessage(t.Context(), "message", "a", "assistant", "keep until deleted", nil); err != nil {
		t.Fatal(err)
	}
	srv := New(db, nil, config.RuntimeConfig{WorkspaceRoot: t.TempDir()})
	call := func(method, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, httptest.NewRequest(method, path, nil))
		return w
	}
	for _, tc := range []struct {
		method, path string
		code         int
	}{{"GET", "/api/sessions/a/messages/message", 405}, {"DELETE", "/api/sessions/a/messages/message?cascade=invalid", 400}, {"DELETE", "/api/sessions/b/messages/message", 404}, {"DELETE", "/api/sessions/a/messages/message/extra", 404}, {"DELETE", "/api/sessions/a/messages/message", 200}, {"DELETE", "/api/sessions/a/messages/message", 404}} {
		if r := call(tc.method, tc.path); r.Code != tc.code {
			t.Fatal(tc, r.Code, r.Body.String())
		}
	}
	pending, err := srv.auth.StartEnrollment("rui")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = srv.auth.VerifyEnrollment("rui", webTestTOTPCode(pending.Secret)); err != nil {
		t.Fatal(err)
	}
	if r := call("DELETE", "/api/sessions/a/messages/message"); r.Code != 401 {
		t.Fatal(r.Code)
	}
}

func TestMessageDeleteCascadeReturnsAllIDs(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "cascade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.CreateSession(t.Context(), "s", "s", nil)
	for _, m := range []struct{ id, role string }{{"prompt", "user"}, {"reply", "assistant"}} {
		if err = db.AddMessage(t.Context(), m.id, "s", m.role, m.id, map[string]any{"turn_id": "t"}); err != nil {
			t.Fatal(err)
		}
	}
	srv := New(db, nil, config.RuntimeConfig{WorkspaceRoot: t.TempDir()})
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest("DELETE", "/api/sessions/s/messages/prompt?cascade=true", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		IDs     []string `json:"ids"`
		Deleted []string `json:"deleted"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.IDs) != 2 || result.IDs[0] != "prompt" || result.IDs[1] != "reply" || len(result.Deleted) != 2 {
		t.Fatalf("result: %+v", result)
	}
	rows, err := db.ListMessages(t.Context(), "s")
	if err != nil || len(rows) != 0 {
		t.Fatal(rows, err)
	}
}
