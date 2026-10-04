// Package shellenv prepares agent and user shell commands like Piclaw 3.2.5:
// the runtime environment with Settings → Environment overrides, keychain
// secrets only for the variables the command text names, keychain
// placeholders resolved, and the configured or detected shell.
package shellenv

import (
	"context"
	"database/sql"
	"strings"

	"github.com/rcarmo/gi/internal/environment"
	"github.com/rcarmo/gi/internal/keychain"
	"github.com/rcarmo/gi/internal/tools"
)

// Environment is the Settings → Environment store, with keychain variable
// names excluded.
func Environment(db *sql.DB) environment.Store {
	kc := keychain.New(db)
	return environment.Store{DB: db, KeychainNames: kc.EnvNames}
}

// Prepare is command as it runs.
func Prepare(ctx context.Context, db *sql.DB, shellPath, command string) (tools.PreparedShell, error) {
	prepared := tools.PreparedShell{Command: command, ShellPath: shellPath}
	if db == nil {
		return prepared, nil
	}
	env, err := Environment(db).Environ(ctx)
	if err != nil {
		return prepared, err
	}
	set := map[string]bool{}
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		set[name] = true
	}
	kc := keychain.New(db)
	extra, err := kc.EnvironmentFor(ctx, command, func(name string) bool { return set[name] })
	if err != nil {
		return prepared, err
	}
	resolved, err := kc.ResolvePlaceholders(ctx, command)
	if err != nil {
		return prepared, err
	}
	prepared.Command = resolved
	prepared.Env = append(env, extra...)
	return prepared, nil
}

// Preparer is Prepare as a tools.ShellPreparer.
func Preparer(db *sql.DB, shellPath string) tools.ShellPreparer {
	return func(ctx context.Context, command string) (tools.PreparedShell, error) {
		return Prepare(ctx, db, shellPath, command)
	}
}
