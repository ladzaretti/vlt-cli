package vlt

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/ladzaretti/vlt-cli/pkg/vlt/store"

	"github.com/ladzaretti/migrate"

	// Package sqlite is a CGo-free port of SQLite/SQLite3.
	_ "modernc.org/sqlite"
)

var (
	//go:embed db/migrations/sqlite
	embedFS embed.FS

	embeddedMigrations = migrate.EmbeddedMigrations{
		FS:   embedFS,
		Path: "db/migrations/sqlite",
	}
)

type Vault struct {
	Path  string
	db    *sql.DB
	store *store.Store
}

// New opens or creates a vault at the specified path and applies migrations.
func New(path string) (*Vault, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, errf("sqlite open: %v", err)
	}

	m := migrate.New(db, migrate.SQLiteDialect{})

	_, err = m.Apply(embeddedMigrations)
	if err != nil {
		return nil, errf("migration: %v", err)
	}

	vlt := &Vault{
		Path:  path,
		db:    db,
		store: store.New(db),
	}

	return vlt, nil
}

func errf(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

func (vlt *Vault) SetMasterKey(k string) error {
	return vlt.store.InsertMasterKey(context.Background(), k)
}

func (vlt *Vault) GetMasterKey() (string, error) {
	return vlt.store.QueryMasterKey(context.Background())
}
