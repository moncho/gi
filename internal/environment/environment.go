// Package environment holds Settings → Environment overrides, as Piclaw's
// environment-overrides.ts: values that replace or add to the runtime's
// inherited environment for every later shell command. Overrides persist in
// the store; keychain variable names can neither be listed nor overridden.
package environment

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	kvNamespace = "environment"
	kvKey       = "overrides"
)

var nameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var (
	ErrInvalidName  = errors.New("Invalid environment variable name.")
	ErrKeychainName = errors.New("Keychain-injected environment variables cannot be overridden here.")
)

// ValidName reports whether name is a shell identifier.
func ValidName(name string) bool { return nameRE.MatchString(name) }

// Row is one listed variable.
type Row struct {
	Name       string `json:"name"`
	Value      string `json:"value"`
	Overridden bool   `json:"overridden"`
	Source     string `json:"source"` // "process" or "override"
}

// Data is the Environment section's view.
type Data struct {
	Variables     []Row             `json:"variables"`
	Overrides     map[string]string `json:"overrides"`
	Count         int               `json:"count"`
	OverrideCount int               `json:"overrideCount"`
}

// Store reads and writes overrides. KeychainNames returns the variables the
// keychain injects (excluded everywhere).
type Store struct {
	DB            *sql.DB
	KeychainNames func(context.Context) map[string]bool
}

func (s Store) keychain(ctx context.Context) map[string]bool {
	if s.KeychainNames == nil {
		return map[string]bool{}
	}
	if names := s.KeychainNames(ctx); names != nil {
		return names
	}
	return map[string]bool{}
}

// Overrides are the stored overrides minus invalid and keychain names.
func (s Store) Overrides(ctx context.Context) (map[string]string, error) {
	return s.load(ctx, s.keychain(ctx))
}

func (s Store) load(ctx context.Context, keychain map[string]bool) (map[string]string, error) {
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `select value from kv_store where namespace=? and key=?`, kvNamespace, kvKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	stored := map[string]string{}
	if err := json.Unmarshal(raw, &stored); err != nil {
		return map[string]string{}, nil
	}
	out := map[string]string{}
	for k, v := range stored {
		if k = strings.TrimSpace(k); ValidName(k) && !keychain[k] {
			out[k] = v
		}
	}
	return out, nil
}

func (s Store) persist(ctx context.Context, overrides map[string]string) error {
	if len(overrides) == 0 {
		_, err := s.DB.ExecContext(ctx, `delete from kv_store where namespace=? and key=?`, kvNamespace, kvKey)
		return err
	}
	raw, err := json.Marshal(overrides)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.DB.ExecContext(ctx, `insert into kv_store(namespace,key,value,created_at,updated_at) values(?,?,?,?,?) on conflict(namespace,key) do update set value=excluded.value, updated_at=excluded.updated_at`, kvNamespace, kvKey, raw, now, now)
	return err
}

// Set stores an override for name.
func (s Store) Set(ctx context.Context, name, value string) (Data, error) {
	name = strings.TrimSpace(name)
	if !ValidName(name) {
		return Data{}, ErrInvalidName
	}
	keychain := s.keychain(ctx)
	if keychain[name] {
		return Data{}, ErrKeychainName
	}
	current, err := s.load(ctx, keychain)
	if err != nil {
		return Data{}, err
	}
	current[name] = value
	if err := s.persist(ctx, current); err != nil {
		return Data{}, err
	}
	return s.Data(ctx)
}

// Clear removes the override for name: the inherited value (if any) applies again.
func (s Store) Clear(ctx context.Context, name string) (Data, error) {
	name = strings.TrimSpace(name)
	if !ValidName(name) {
		return Data{}, ErrInvalidName
	}
	keychain := s.keychain(ctx)
	current, err := s.load(ctx, keychain)
	if err != nil {
		return Data{}, err
	}
	if _, ok := current[name]; ok {
		delete(current, name)
		if err := s.persist(ctx, current); err != nil {
			return Data{}, err
		}
	}
	return s.Data(ctx)
}

// Data lists the inherited variables and overrides, keychain names excluded.
func (s Store) Data(ctx context.Context) (Data, error) {
	keychain := s.keychain(ctx)
	overrides, err := s.load(ctx, keychain)
	if err != nil {
		return Data{}, err
	}
	values := map[string]string{}
	for _, kv := range os.Environ() {
		name, value, _ := strings.Cut(kv, "=")
		if ValidName(name) && !keychain[name] {
			values[name] = value
		}
	}
	for name, value := range overrides {
		values[name] = value
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	rows := make([]Row, 0, len(names))
	for _, name := range names {
		_, overridden := overrides[name]
		source := "process"
		if overridden {
			source = "override"
		}
		rows = append(rows, Row{Name: name, Value: values[name], Overridden: overridden, Source: source})
	}
	return Data{Variables: rows, Overrides: overrides, Count: len(rows), OverrideCount: len(overrides)}, nil
}

// Environ is the process environment with the overrides applied, as
// NAME=value pairs for a command.
func (s Store) Environ(ctx context.Context) ([]string, error) {
	overrides, err := s.Overrides(ctx)
	if err != nil {
		return nil, err
	}
	return Apply(os.Environ(), overrides), nil
}

// Apply replaces or adds overrides in env.
func Apply(env []string, overrides map[string]string) []string {
	out := make([]string, 0, len(env)+len(overrides))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if _, ok := overrides[name]; ok {
			continue
		}
		out = append(out, kv)
	}
	names := make([]string, 0, len(overrides))
	for name := range overrides {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out = append(out, name+"="+overrides[name])
	}
	return out
}
