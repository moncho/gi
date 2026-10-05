package web

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/store"
	"github.com/rcarmo/gi/internal/turn"
)

func TestWorkspaceEditorWatcherSharesResourcesConfinesPathsAndStops(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	srv := New(nil, nil, config.RuntimeConfig{WorkspaceRoot: root})
	first, stop1 := srv.subscribeWorkspaceChanges()
	second, stop2 := srv.subscribeWorkspaceChanges()
	defer stop1()
	defer stop2()
	if first == nil || second == nil {
		t.Fatal("watcher unavailable")
	}
	srv.workspaceWatchMu.Lock()
	ww := srv.workspaceWatch
	srv.workspaceWatchMu.Unlock()
	for _, p := range ww.watcher.WatchList() {
		if p == outside {
			t.Fatal("watcher escaped root")
		}
	}
	if err := os.WriteFile(filepath.Join(nested, "change.md"), []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, ch := range []<-chan []string{first, second} {
		select {
		case paths := <-ch:
			if !reflect.DeepEqual(paths, []string{"nested/change.md"}) {
				t.Fatal(paths)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("no change")
		}
	}
	stop1()
	srv.workspaceWatchMu.Lock()
	same := srv.workspaceWatch == ww
	srv.workspaceWatchMu.Unlock()
	if !same {
		t.Fatal("closed shared watcher")
	}
	stop2()
	stop2()
	srv.workspaceWatchMu.Lock()
	remaining := srv.workspaceWatch
	srv.workspaceWatchMu.Unlock()
	if remaining != nil {
		t.Fatal("leaked watcher")
	}
	select {
	case <-ww.done:
	default:
		t.Fatal("watch goroutine did not stop")
	}
	if _, ok := <-first; ok {
		t.Fatal("subscription not closed")
	}
}

func TestWorkspaceEditorSSEReportsExternalChangeWithBoundedSnapshot(t *testing.T) {
	root := t.TempDir()
	db, err := store.Open(filepath.Join(t.TempDir(), "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	engine := turn.New(db)
	defer engine.Close()
	srv := New(db, engine, config.RuntimeConfig{WorkspaceRoot: root})
	server := httptest.NewServer(srv.Handler())
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/sse/stream?chat_jid=gi:test", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		if scanner.Text() == "" {
			break
		}
	}
	if err := os.WriteFile(filepath.Join(root, "external.md"), []byte("secret content not in SSE"), 0o600); err != nil {
		t.Fatal(err)
	}
	for scanner.Scan() {
		const prefix = "data: "
		line := scanner.Text()
		if len(line) < len(prefix) || line[:len(prefix)] != prefix {
			continue
		}
		var p map[string]any
		if json.Unmarshal([]byte(line[len(prefix):]), &p) != nil {
			continue
		}
		updates, ok := p["updates"].([]any)
		if !ok {
			continue
		}
		if len(updates) != 1 {
			t.Fatal(p)
		}
		update := updates[0].(map[string]any)
		tree, ok := update["root"].(map[string]any)
		if !ok || tree["type"] != "dir" || update["truncated"] != false || !reflect.DeepEqual(update["changed_paths"], []any{"external.md"}) {
			t.Fatal(p)
		}
		if strings.Contains(line, "secret content not in SSE") {
			t.Fatal("file contents leaked")
		}
		return
	}
	t.Fatal("missing SSE change", scanner.Err())
}
