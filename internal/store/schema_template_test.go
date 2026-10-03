package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// New stores copy the schema template; it must be exactly what initSchema
// builds: the same schema objects, migration records and user_version.
func TestSchemaTemplateMatchesInitSchema(t *testing.T) {
	dir := t.TempDir()
	// An existing (empty) file is migrated by initSchema.
	migratedPath := filepath.Join(dir, "migrated.db")
	if err := os.WriteFile(migratedPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	migrated, err := Open(migratedPath)
	if err != nil {
		t.Fatal(err)
	}
	defer migrated.Close()
	// A new file and a new in-memory store come from the template.
	createdPath := filepath.Join(dir, "created.db")
	created, err := Open(createdPath)
	if err != nil {
		t.Fatal(err)
	}
	defer created.Close()
	memory, err := Open("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer memory.Close()

	dump := func(db *sql.DB) string {
		t.Helper()
		var out []string
		rows, err := db.Query(`select type, name, tbl_name, coalesce(sql,'') from sqlite_master order by type, name`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var a, b, c, d string
			if err := rows.Scan(&a, &b, &c, &d); err != nil {
				t.Fatal(err)
			}
			out = append(out, strings.Join([]string{a, b, c, d}, "|"))
		}
		var version int
		if err := db.QueryRow(`pragma user_version`).Scan(&version); err != nil {
			t.Fatal(err)
		}
		out = append(out, "user_version|"+strconv.Itoa(version))
		migrations, err := db.Query(`select version, checksum from workspace_index_migrations order by version`)
		if err != nil {
			t.Fatal(err)
		}
		defer migrations.Close()
		for migrations.Next() {
			var v, sum string
			if err := migrations.Scan(&v, &sum); err != nil {
				t.Fatal(err)
			}
			out = append(out, "migration|"+v+"|"+sum)
		}
		return strings.Join(out, "\n")
	}
	want := dump(migrated.db)
	if got := dump(created.db); got != want {
		t.Fatalf("file from template differs from initSchema:\n%s\n---\n%s", got, want)
	}
	if got := dump(memory.db); got != want {
		t.Fatal("in-memory store from template differs from initSchema")
	}
	var mode string
	if err := created.db.QueryRow(`pragma journal_mode`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal mode %q %v", mode, err)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(dir, ".created.db.template-*")); len(leftovers) != 0 {
		t.Fatalf("template files left: %v", leftovers)
	}
	// The template store is usable.
	if _, err := created.CreateSession(t.Context(), "s1", "t", nil); err != nil {
		t.Fatal(err)
	}
}
