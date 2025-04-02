package vlt

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log"

	"github.com/ladzaretti/vlt-cli/pkg/database"

	// Package sqlite is a CGo-free port of SQLite/SQLite3.
	_ "modernc.org/sqlite"

	"github.com/ladzaretti/migrate"
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
	db          *sql.DB
	dataHandler *database.Handler
}

func errf(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

func Open(path string) (*Vault, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, errf("sqlite open: %v", err)
	}

	m := migrate.New(db, migrate.SQLiteDialect{})

	n, err := m.Apply(embeddedMigrations)
	if err != nil {
		return nil, errf("migration: %v", err)
	}

	log.Printf("Number migration scripts applied: %d", n)

	vlt := &Vault{db: db, dataHandler: database.NewHandler(db)}

	return vlt, nil
}

func (v *Vault) SetMasterKey(k string) error {
	return v.dataHandler.SaveMasterKey(context.Background(), k)
}
