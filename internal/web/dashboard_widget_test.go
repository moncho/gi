package web

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/store"
)

func TestDashboardWidgetHTTPPersistenceIsolationAndAuth(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "widgets.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.CreateSession(t.Context(), "s", "s", nil)
	db.CreateSession(t.Context(), "other", "other", nil)
	block := map[string]any{"type": "generated_widget", "widget_id": "widget-a", "title": "Hi", "artifact": map[string]any{"kind": "html", "html": "<p>Hello</p>"}}
	message, err := db.PostDashboardWidget(t.Context(), "s", "t", "fallback", block)
	if err != nil {
		t.Fatal(err)
	}
	server := New(db, nil, config.RuntimeConfig{WorkspaceRoot: t.TempDir()})
	call := func(method, path string) *httptest.ResponseRecorder {
		r := httptest.NewRecorder()
		server.Handler().ServeHTTP(r, httptest.NewRequest(method, path, nil))
		return r
	}
	r := call("GET", "/api/sessions/s/widgets/widget-a")
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	var artifact store.DashboardWidget
	if err = json.Unmarshal(r.Body.Bytes(), &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.PostID != message.ID || !strings.Contains(r.Body.String(), "Hello") {
		t.Fatal(artifact)
	}
	for _, tc := range []struct {
		method, path string
		code         int
	}{
		{"GET", "/api/sessions/other/widgets/widget-a", 404},
		{"GET", "/api/sessions/s/widgets/missing", 404},
		{"GET", "/api/sessions/s/widgets/widget-a/extra", 404},
		{"POST", "/api/sessions/s/widgets/widget-a", 405},
	} {
		if r = call(tc.method, tc.path); r.Code != tc.code {
			t.Fatal(tc, r.Code, r.Body.String())
		}
	}
	pending, err := server.auth.StartEnrollment("rui")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = server.auth.VerifyEnrollment("rui", webTestTOTPCode(pending.Secret)); err != nil {
		t.Fatal(err)
	}
	if r = call("GET", "/api/sessions/s/widgets/widget-a"); r.Code != 401 {
		t.Fatal("unprotected artifact", r.Code)
	}
	rows, _ := db.ListMessages(t.Context(), "s")
	if len(rows) != 1 {
		t.Fatal("lookup mutated messages")
	}
}
