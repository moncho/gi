package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"modernc.org/sqlite"
)

// New stores (tests open them by the hundred) copy a migrated template
// database instead of running initSchema: SQLite then parses the schema once
// instead of executing every statement, migration and check. The template is
// built by initSchema in the same process, so it is the current schema.
var schemaTemplate struct {
	once sync.Once
	uri  string
	keep *sql.Conn // an in-memory database lives while a connection is open
	err  error
}

func buildSchemaTemplate() {
	schemaTemplate.uri = fmt.Sprintf("file:gi_schema_template_%d?mode=memory&cache=shared", os.Getpid())
	db, err := sql.Open("sqlite", schemaTemplate.uri+"&_pragma=foreign_keys(ON)")
	if err == nil {
		schemaTemplate.keep, err = db.Conn(context.Background())
	}
	if err == nil {
		err = initSchema(db)
	}
	schemaTemplate.err = err
}

// schemaEmpty reports a database without any schema object (a new one).
func schemaEmpty(db *sql.DB) bool {
	var n int
	return db.QueryRow(`select count(*) from sqlite_master`).Scan(&n) == nil && n == 0
}

// createFromSchemaTemplate creates the database file path from the
// template: written beside it, then linked into place, which fails if
// another process created the file meanwhile (that one is then opened and
// migrated as usual).
func createFromSchemaTemplate(path string) error {
	dir, base := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	tmp, err := os.CreateTemp(dir, "."+base+".template-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)
	db, err := sql.Open("sqlite", tmpPath)
	if err != nil {
		return err
	}
	err = restoreSchemaTemplate(db)
	if closeErr := db.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Link(tmpPath, path)
}

// restoreSchemaTemplate copies the template into db, a new store.
func restoreSchemaTemplate(db *sql.DB) error {
	schemaTemplate.once.Do(buildSchemaTemplate)
	if schemaTemplate.err != nil {
		return schemaTemplate.err
	}
	conn, err := db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.Raw(func(driverConn any) error {
		restorer, ok := driverConn.(interface {
			NewRestore(string) (*sqlite.Backup, error)
		})
		if !ok {
			return fmt.Errorf("sqlite driver cannot restore")
		}
		restore, err := restorer.NewRestore(schemaTemplate.uri)
		if err != nil {
			return err
		}
		_, stepErr := restore.Step(-1)
		if err := restore.Finish(); stepErr == nil {
			stepErr = err
		}
		return stepErr
	})
}
