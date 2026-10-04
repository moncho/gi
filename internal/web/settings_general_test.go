package web

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/store"
	"github.com/rcarmo/gi/internal/turn"
)

func TestSettingsGeneralUploadLimit(t *testing.T) {
	root := t.TempDir()
	cfg := config.Load(root)
	st, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := New(st, turn.NewWithRuntimeConfig(st, cfg, "test-model"), cfg)
	call := func(method, body string) (int, int) {
		req := httptest.NewRequest(method, "/api/settings/general", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr, req.Host = "127.0.0.1:1234", "127.0.0.1:8090"
		res := httptest.NewRecorder()
		srv.Handler().ServeHTTP(res, req)
		var out struct {
			Settings generalSettings `json:"settings"`
		}
		json.Unmarshal(res.Body.Bytes(), &out)
		return res.Code, out.Settings.WorkspaceUploadLimitMB
	}
	if code, limit := call("GET", ""); code != 200 || limit != defaultUploadLimitMB {
		t.Fatalf("default %d %d", code, limit)
	}
	if code, limit := call("POST", `{"workspaceUploadLimitMb":64}`); code != 200 || limit != 64 {
		t.Fatalf("save %d %d", code, limit)
	}
	for _, body := range []string{`{"workspaceUploadLimitMb":0}`, `{"workspaceUploadLimitMb":1025}`} {
		if code, _ := call("POST", body); code != 400 {
			t.Fatalf("%s accepted: %d", body, code)
		}
	}
	if code, limit := call("GET", ""); code != 200 || limit != 64 {
		t.Fatalf("persisted %d %d", code, limit)
	}
	if code, _ := call("PUT", "{}"); code != 405 {
		t.Fatalf("PUT %d", code)
	}
}
