package web

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebQueueHoldResumeRouteRemoved(t *testing.T) {
	ctx := context.Background()
	server, s := newTestWebServer(t, t.TempDir())
	defer s.Close()
	h := server.Handler()
	if _, err := s.CreateSession(ctx, "A", "test", nil); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "POST"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, "/api/sessions/A/resume-queue", strings.NewReader(`{"stop_turn_id":"stopped"}`)))
		if w.Code != 404 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/sessions/A/activity", nil))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var activity map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &activity); err != nil {
		t.Fatal(err)
	}
	if _, exists := activity["queue_hold_turn_id"]; exists {
		t.Fatal("removed UI contract still exposed", activity)
	}
}
