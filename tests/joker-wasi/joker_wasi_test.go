package jokerwasi

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// This is a feasibility test, not the production codemode security boundary.
func TestJokerWASIGuest(t *testing.T) {
	path := os.Getenv("GI_JOKER_WASI_GUEST")
	if path == "" {
		t.Skip("run make test-joker-wasi to build the guest")
	}
	wasm, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, backend := range []struct {
		name   string
		config wazero.RuntimeConfig
	}{
		{"interpreter", wazero.NewRuntimeConfigInterpreter()},
		{"compiler", wazero.NewRuntimeConfigCompiler()},
	} {
		t.Run(backend.name, func(t *testing.T) {
			runtime := wazero.NewRuntimeWithConfig(ctx, backend.config.WithCloseOnContextDone(true).WithMemoryLimitPages(4096))
			defer runtime.Close(ctx)
			if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
				t.Fatal(err)
			}
			compiled, err := runtime.CompileModule(ctx, wasm)
			if err != nil {
				t.Fatal(err)
			}
			defer compiled.Close(ctx)
			for _, tc := range []struct {
				name, source, want string
				timeout            time.Duration
				cancelled          bool
			}{
				{"arithmetic", "(println (+ 40 2))", "42", 10 * time.Second, false},
				{"transformation", "(println (mapv inc [1 2 3]))", "[2 3 4]", 10 * time.Second, false},
				{"infinite-loop", "(loop [] (recur))", "", time.Second, true},
				{"after-cancellation", "(println (+ 40 2))", "42", 10 * time.Second, false},
			} {
				t.Run(tc.name, func(t *testing.T) {
					callCtx, cancel := context.WithTimeout(ctx, tc.timeout)
					defer cancel()
					var stdout, stderr bytes.Buffer
					// No filesystem mounts, inherited environment, or network imports.
					config := wazero.NewModuleConfig().WithName("").WithArgs("joker", tc.source).WithStdout(&stdout).WithStderr(&stderr)
					module, err := runtime.InstantiateModule(callCtx, compiled, config)
					if module != nil {
						defer module.Close(ctx)
					}
					if tc.cancelled {
						if err == nil || callCtx.Err() != context.DeadlineExceeded {
							t.Fatalf("expected deadline termination, got %v (context %v)", err, callCtx.Err())
						}
						return
					}
					if err != nil {
						t.Fatalf("guest: %v; stderr: %s", err, stderr.String())
					}
					if got := strings.TrimSpace(stdout.String()); got != tc.want {
						t.Fatalf("want %q, got %q; stderr: %s", tc.want, got, stderr.String())
					}
				})
			}
		})
	}
}
