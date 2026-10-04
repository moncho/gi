// Package keychain is Piclaw's keychain for gi: named credentials encrypted
// at rest in the session database, managed from Settings and made available
// to shell commands that name them (docs/internal/keychain.md).
//
// Entries use Piclaw's format: AES-256-GCM with a key derived by
// PBKDF2-SHA256 (150,000 iterations, a 16-byte salt per entry) from the
// master key, a 12-byte nonce, and the entry name as additional data. The
// sealed payload is {"secret": …, "username": …}.
package keychain

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	kdfAlgo       = "pbkdf2-sha256"
	kdfIterations = 150_000
	saltBytes     = 16
	nonceBytes    = 12
	prefix        = "keychain:"
)

// Types are the entry types, as Piclaw's.
var Types = []string{"secret", "token", "password", "basic"}

// ErrDisabled is returned when no master key is configured.
var ErrDisabled = errors.New("Keychain is disabled. Set GI_KEYCHAIN_KEY or GI_KEYCHAIN_KEY_FILE.")

// Entry is a decrypted entry.
type Entry struct {
	Name      string
	Type      string
	Secret    string
	Username  string
	UserNote  *string // nil keeps the stored note on update
	AgentNote *string
}

// Meta is an entry without its secret, as Settings lists it.
type Meta struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	UserNote  string `json:"userNote"`
	AgentNote string `json:"agentNote"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	EnvVar    string `json:"envVar,omitempty"`
}

// Keychain reads and writes entries in a gi store's database.
type Keychain struct {
	db  *sql.DB
	key func() (string, error)
}

// New is the keychain in db (the store's keychain_entries table), keyed by
// MasterKey.
func New(db *sql.DB) *Keychain { return &Keychain{db: db, key: MasterKey} }

// WithKey is the keychain with a fixed master key (tests).
func WithKey(db *sql.DB, key string) *Keychain {
	return &Keychain{db: db, key: func() (string, error) { return key, nil }}
}

// MasterKey is the configured master key: GI_KEYCHAIN_KEY, or the trimmed
// contents of GI_KEYCHAIN_KEY_FILE, or Piclaw's PICLAW_KEYCHAIN_KEY and
// PICLAW_KEYCHAIN_KEY_FILE.
func MasterKey() (string, error) {
	for _, prefix := range []string{"GI", "PICLAW"} {
		if key := os.Getenv(prefix + "_KEYCHAIN_KEY"); key != "" {
			return key, nil
		}
		if file := os.Getenv(prefix + "_KEYCHAIN_KEY_FILE"); file != "" {
			data, err := os.ReadFile(file)
			if err != nil {
				return "", fmt.Errorf("read keychain key file: %w", err)
			}
			return strings.TrimSpace(string(data)), nil
		}
	}
	return "", ErrDisabled
}

// Enabled reports whether a master key is configured.
func (k *Keychain) Enabled() bool {
	key, err := k.key()
	return err == nil && key != ""
}

// MasterKeyMatches reports whether password is the master key (Settings
// asks for it before revealing a secret).
func (k *Keychain) MasterKeyMatches(password string) bool {
	key, err := k.key()
	return err == nil && key != "" && subtle.ConstantTimeCompare([]byte(password), []byte(key)) == 1
}

func (k *Keychain) aead(salt []byte, iterations int) (cipher.AEAD, error) {
	key, err := k.key()
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, ErrDisabled
	}
	derived, err := pbkdf2.Key(sha256.New, key, salt, iterations, 32)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(derived)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func validType(t string) bool {
	for _, v := range Types {
		if v == t {
			return true
		}
	}
	return false
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// Set encrypts and stores an entry, replacing one of the same name.
func (k *Keychain) Set(ctx context.Context, e Entry) error {
	if e.Name == "" {
		return errors.New("Keychain entry name is required.")
	}
	if e.Secret == "" {
		return errors.New("Keychain entry secret is required.")
	}
	if !validType(e.Type) {
		e.Type = "secret"
	}
	salt, nonce := make([]byte, saltBytes), make([]byte, nonceBytes)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	aead, err := k.aead(salt, kdfIterations)
	if err != nil {
		return err
	}
	payload := map[string]any{"secret": e.Secret, "username": nil}
	if e.Username != "" {
		payload["username"] = e.Username
	}
	plain, _ := json.Marshal(payload)
	sealed := aead.Seal(nil, nonce, plain, []byte(e.Name))
	note := func(v *string) string {
		if v == nil {
			return ""
		}
		return *v
	}
	stamp := now()
	_, err = k.db.ExecContext(ctx, `insert into keychain_entries (name, type, ciphertext, nonce, salt, kdf, kdf_iterations, user_note, agent_note, created_at, updated_at)
		values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		on conflict(name) do update set type = excluded.type, ciphertext = excluded.ciphertext, nonce = excluded.nonce, salt = excluded.salt,
			kdf = excluded.kdf, kdf_iterations = excluded.kdf_iterations,
			user_note = case when ? then excluded.user_note else keychain_entries.user_note end,
			agent_note = case when ? then excluded.agent_note else keychain_entries.agent_note end,
			updated_at = excluded.updated_at`,
		e.Name, e.Type, sealed, nonce, salt, kdfAlgo, kdfIterations, note(e.UserNote), note(e.AgentNote), stamp, stamp,
		e.UserNote != nil, e.AgentNote != nil)
	return err
}

// Get decrypts an entry.
func (k *Keychain) Get(ctx context.Context, name string) (Entry, error) {
	var e Entry
	var sealed, nonce, salt []byte
	var kdf string
	var iterations int
	err := k.db.QueryRowContext(ctx, `select name, type, ciphertext, nonce, salt, kdf, kdf_iterations from keychain_entries where name = ?`, name).
		Scan(&e.Name, &e.Type, &sealed, &nonce, &salt, &kdf, &iterations)
	if errors.Is(err, sql.ErrNoRows) {
		return Entry{}, fmt.Errorf("Keychain entry not found: %s", name)
	}
	if err != nil {
		return Entry{}, err
	}
	if kdf != kdfAlgo {
		return Entry{}, fmt.Errorf("Unsupported keychain KDF: %s", kdf)
	}
	aead, err := k.aead(salt, iterations)
	if err != nil {
		return Entry{}, err
	}
	plain, err := aead.Open(nil, nonce, sealed, []byte(e.Name))
	if err != nil {
		return Entry{}, fmt.Errorf("Cannot decrypt keychain entry %s: %w", name, err)
	}
	var payload struct {
		Secret   string  `json:"secret"`
		Username *string `json:"username"`
	}
	if err := json.Unmarshal(plain, &payload); err != nil || payload.Secret == "" {
		return Entry{}, errors.New("Invalid keychain payload.")
	}
	e.Secret = payload.Secret
	if payload.Username != nil {
		e.Username = *payload.Username
	}
	return e, nil
}

// List is every entry's metadata, by name, with its shell variable.
func (k *Keychain) List(ctx context.Context) ([]Meta, error) {
	rows, err := k.db.QueryContext(ctx, `select name, type, user_note, agent_note, created_at, updated_at from keychain_entries order by name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Meta
	for rows.Next() {
		var m Meta
		if err := rows.Scan(&m.Name, &m.Type, &m.UserNote, &m.AgentNote, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	vars := injectable(out)
	for i := range out {
		out[i].EnvVar = vars[out[i].Name]
	}
	return out, nil
}

// Delete removes an entry; false when there was none.
func (k *Keychain) Delete(ctx context.Context, name string) (bool, error) {
	res, err := k.db.ExecContext(ctx, `delete from keychain_entries where name = ?`, name)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// UpdateNotes replaces an entry's notes; false when there is no entry.
func (k *Keychain) UpdateNotes(ctx context.Context, name, userNote, agentNote string) (bool, error) {
	res, err := k.db.ExecContext(ctx, `update keychain_entries set user_note = ?, agent_note = ?, updated_at = ? where name = ?`, userNote, agentNote, now(), name)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

var (
	shellName      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	separatorRuns  = regexp.MustCompile(`[/.-]+`)
	invalidEnvRune = regexp.MustCompile(`[^A-Za-z0-9_]`)
)

// EnvName is an entry name's shell variable: each run of "/", "-" or "."
// becomes "_", other characters outside [A-Za-z0-9_] are dropped and the
// result is upper-cased; "" when that is empty or starts with a digit.
func EnvName(name string) string {
	v := strings.ToUpper(invalidEnvRune.ReplaceAllString(separatorRuns.ReplaceAllString(name, "_"), ""))
	if v == "" || v[0] >= '0' && v[0] <= '9' {
		return ""
	}
	return v
}

// injectable maps entry names to their shell variables, as Piclaw's
// listInjectableKeychainEntries: a name that already is a shell variable
// name is used as it is, others by EnvName; of two entries with the same
// variable the first by name keeps it.
func injectable(entries []Meta) map[string]string {
	sorted := append([]Meta(nil), entries...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })
	out, seen := map[string]string{}, map[string]bool{}
	for _, e := range sorted {
		v := e.Name
		if !shellName.MatchString(v) {
			v = EnvName(v)
		}
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out[e.Name] = v
	}
	return out
}

// referencePatterns are the literal variable references a command can
// make: $env:NAME (PowerShell), ${NAME}, $NAME and %NAME% (cmd).
var referencePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\$env:([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`),
	regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)`),
	regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`),
}

func referencedNames(texts []string) map[string]bool {
	names := map[string]bool{}
	for _, text := range texts {
		for _, re := range referencePatterns {
			for _, m := range re.FindAllStringSubmatch(text, -1) {
				names[m[1]] = true
			}
		}
	}
	return names
}

// Environment is the keychain variables command names literally, as
// NAME=value; a variable already in the process environment keeps its
// value. Without a master key it is empty.
func (k *Keychain) Environment(ctx context.Context, command string) ([]string, error) {
	named := referencedNames([]string{command})
	if len(named) == 0 {
		return nil, nil
	}
	entries, err := k.List(ctx)
	if err != nil {
		return nil, err
	}
	var env []string
	for _, e := range entries {
		if e.EnvVar == "" || !named[e.EnvVar] {
			continue
		}
		if _, set := os.LookupEnv(e.EnvVar); set {
			continue
		}
		entry, err := k.Get(ctx, e.Name)
		if errors.Is(err, ErrDisabled) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		env = append(env, e.EnvVar+"="+entry.Secret)
	}
	return env, nil
}

// placeholders are the keychain:<name>[:field] references in input, as
// Piclaw's findKeychainPlaceholders finds them.
func placeholders(input string) []string {
	var out []string
	for cursor := 0; cursor < len(input); {
		start := strings.Index(input[cursor:], prefix)
		if start < 0 {
			break
		}
		start += cursor
		end := start + len(prefix)
		if end >= len(input) || !isRefStart(input[end]) {
			cursor = end
			continue
		}
		for end < len(input) && !strings.HasPrefix(input[end:], ":"+prefix) && isRefChar(input[end]) {
			end++
		}
		out = append(out, input[start:end])
		cursor = end
	}
	return out
}

func isRefStart(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '/' || c == '-'
}

func isRefChar(c byte) bool { return isRefStart(c) || c == ':' }

// parseReference is Piclaw's parseKeychainReference: a trailing :username
// or :user is the username, :secret, :password or :token the secret.
func parseReference(value string) (name string, username bool, err error) {
	raw := strings.TrimPrefix(value, prefix)
	if raw == "" || strings.HasPrefix(raw, ":") {
		return "", false, fmt.Errorf("Invalid keychain reference: %s", value)
	}
	i := strings.LastIndex(raw, ":")
	if i < 0 {
		return raw, false, nil
	}
	name, field := raw[:i], raw[i+1:]
	if name == "" {
		return "", false, fmt.Errorf("Invalid keychain reference: %s", value)
	}
	switch field {
	case "username", "user":
		return name, true, nil
	case "secret", "password", "token":
		return name, false, nil
	}
	if !strings.Contains(name, ":") {
		return "", false, fmt.Errorf("Invalid keychain reference: %s", value)
	}
	return raw, false, nil
}

// ResolvePlaceholders replaces keychain:<name> placeholders in input with
// the entries' secrets, and keychain:<name>:username (or :user) with their
// usernames.
func (k *Keychain) ResolvePlaceholders(ctx context.Context, input string) (string, error) {
	if !strings.Contains(input, prefix) {
		return input, nil
	}
	found := placeholders(input)
	if len(found) == 0 {
		return input, nil
	}
	values := map[string]string{}
	for _, p := range found {
		if _, ok := values[p]; ok {
			continue
		}
		name, username, err := parseReference(p)
		if err != nil {
			return "", err
		}
		e, err := k.Get(ctx, name)
		if err != nil {
			return "", err
		}
		if username {
			if e.Username == "" {
				return "", fmt.Errorf("Keychain entry %s has no username.", name)
			}
			values[p] = e.Username
		} else {
			values[p] = e.Secret
		}
	}
	keys := make([]string, 0, len(values))
	for p := range values {
		keys = append(keys, p)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, p := range keys {
		input = strings.ReplaceAll(input, p, values[p])
	}
	return input, nil
}

// PrepareShell is a shell command as it runs: its placeholders resolved,
// and the environment additions for the variables it names.
func (k *Keychain) PrepareShell(ctx context.Context, command string) (string, []string, error) {
	env, err := k.Environment(ctx, command)
	if err != nil {
		return "", nil, err
	}
	resolved, err := k.ResolvePlaceholders(ctx, command)
	if err != nil {
		return "", nil, err
	}
	return resolved, env, nil
}
