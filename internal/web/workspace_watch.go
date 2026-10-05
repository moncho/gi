package web

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// One bounded watcher per server, shared only by active SSE clients. Events
// contain relative paths, never file contents. No idle directory polling.
type workspaceWatch struct {
	root    string
	watcher *fsnotify.Watcher
	mu      sync.Mutex
	subs    map[chan []string]struct{}
	done    chan struct{}
	watched map[string]struct{} // owned by setup, then the event-loop goroutine
}

func (s *Server) subscribeWorkspaceChanges() (<-chan []string, func()) {
	s.workspaceWatchMu.Lock()
	defer s.workspaceWatchMu.Unlock()
	if s.workspaceWatch == nil {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, func() {}
		}
		root, err := filepath.EvalSymlinks(s.workspaceRootPath())
		if err != nil {
			watcher.Close()
			return nil, func() {}
		}
		root, err = filepath.Abs(root)
		if err != nil {
			watcher.Close()
			return nil, func() {}
		}
		ww := &workspaceWatch{root: root, watcher: watcher, subs: map[chan []string]struct{}{}, done: make(chan struct{}), watched: map[string]struct{}{root: {}}}
		if err := watcher.Add(root); err != nil {
			watcher.Close()
			return nil, func() {}
		}
		ww.addDirectories(root)
		s.workspaceWatch = ww
		go ww.run()
	}
	ww := s.workspaceWatch
	ch := make(chan []string, 1)
	ww.mu.Lock()
	ww.subs[ch] = struct{}{}
	ww.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			s.workspaceWatchMu.Lock()
			defer s.workspaceWatchMu.Unlock()
			ww.mu.Lock()
			delete(ww.subs, ch)
			close(ch)
			empty := len(ww.subs) == 0
			ww.mu.Unlock()
			if empty && s.workspaceWatch == ww {
				s.workspaceWatch = nil
				ww.watcher.Close()
				<-ww.done
			}
		})
	}
}

func workspaceWatchExcluded(name string) bool {
	switch name {
	case ".git", "node_modules", ".cache", ".venv", ".gi-run", ".gi-test", ".gi-ux-parity", "generated", "dist", "build", "output", "coverage":
		return true
	}
	return false
}

func (ww *workspaceWatch) addDirectories(start string) {
	root, err := os.OpenRoot(ww.root)
	if err != nil {
		return
	}
	defer root.Close()
	remaining := workspaceTreeNodeLimit
	var walk func(string)
	walk = func(path string) {
		rel, err := filepath.Rel(ww.root, path)
		if err != nil || !filepath.IsLocal(rel) || remaining <= 0 {
			return
		}
		if rel != "." && (workspaceWatchExcluded(filepath.Base(path)) || strings.Count(rel, string(filepath.Separator)) >= 4) {
			return
		}
		info, err := root.Lstat(rel)
		if err != nil || !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
			return
		}
		if _, exists := ww.watched[path]; !exists {
			if len(ww.watched) >= 1024 {
				return
			}
			if err := ww.watcher.Add(path); err != nil {
				return
			}
			ww.watched[path] = struct{}{}
		}
		dir, err := root.Open(rel)
		if err != nil {
			return
		}
		entries, _ := dir.ReadDir(remaining + 1)
		dir.Close()
		if len(entries) > remaining {
			return
		}
		remaining -= len(entries)
		for _, entry := range entries {
			if entry.IsDir() && entry.Type()&fs.ModeSymlink == 0 {
				walk(filepath.Join(path, entry.Name()))
			}
		}
	}
	walk(start)
}

func (ww *workspaceWatch) run() {
	defer close(ww.done)
	pending := map[string]struct{}{}
	var timer *time.Timer
	var tick <-chan time.Time
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()
	queue := func(path string) {
		if len(pending) >= 256 {
			clear(pending)
			pending["."] = struct{}{}
		}
		if _, all := pending["."]; !all {
			pending[path] = struct{}{}
		}
		if timer == nil {
			timer = time.NewTimer(100 * time.Millisecond)
			tick = timer.C
		}
	}
	for {
		select {
		case event, ok := <-ww.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
				continue
			}
			rel, err := filepath.Rel(ww.root, event.Name)
			if err != nil || !filepath.IsLocal(rel) {
				continue
			}
			excluded := false
			for _, part := range strings.Split(rel, string(filepath.Separator)) {
				if workspaceWatchExcluded(part) {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
			if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				for path := range ww.watched {
					if path == event.Name || strings.HasPrefix(path, event.Name+string(filepath.Separator)) {
						_ = ww.watcher.Remove(path)
						delete(ww.watched, path)
					}
				}
			}
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Lstat(event.Name); err == nil && info.IsDir() {
					ww.addDirectories(event.Name)
					queue(".") // Files may have arrived before directory watches.
				}
			}
			queue(filepath.ToSlash(rel))
		case _, ok := <-ww.watcher.Errors:
			if !ok {
				return
			}
			queue(".") // A lost event asks clients to re-read.
		case <-tick:
			paths := make([]string, 0, len(pending))
			for p := range pending {
				paths = append(paths, p)
			}
			sort.Strings(paths)
			ww.mu.Lock()
			for ch := range ww.subs {
				select {
				case ch <- paths:
				default:
					select {
					case <-ch:
					default:
					}
					select {
					case ch <- []string{"."}:
					default:
					}
				}
			}
			ww.mu.Unlock()
			clear(pending)
			timer = nil
			tick = nil
		}
	}
}
