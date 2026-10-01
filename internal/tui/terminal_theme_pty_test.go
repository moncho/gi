package tui

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/store"
	"github.com/rcarmo/gi/internal/turn"
)

// This entrypoint is only in a test binary, not a shipped diagnostic command.
func TestTerminalThemePTYFixture(t *testing.T) {
	dir := os.Getenv("GI_THEME_PTY_DIR")
	if dir == "" {
		t.Skip("make test-tui-theme-pty")
	}
	started := time.Now()
	name := initPiTheme(os.Getenv("GI_THEME_SETTING"))
	fmt.Printf("THEME-RESULT:%s elapsed=%d\n", name, time.Since(started).Milliseconds())
	if os.Getenv("GI_THEME_PROBE_ONLY") != "" {
		return
	}
	cfg := config.Load(dir)
	cfg.DefaultModel = "test-model"
	cfg.Theme = os.Getenv("GI_THEME_SETTING")
	s, err := store.Open(dir + "/state.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	engine := turn.NewWithRuntimeConfig(s, cfg, cfg.SystemPrompt)
	defer engine.Close()
	if err := runWithEngineMode(s, engine, cfg, false); err != nil {
		t.Fatal(err)
	}
}
