package store

import (
	"database/sql"
	"embed"
	"fmt"

	// Package sqlite is a CGo-free port of SQLite/SQLite3.
	_ "modernc.org/sqlite"

	"github.com/ladzaretti/migrate"
)

var (
	//go:embed migrations/sqlite
	embedFS embed.FS

	embeddedMigrations = migrate.EmbeddedMigrations{
		FS:   embedFS,
		Path: "migrations/sqlite",
	}
)

type Store struct {
	db *sql.DB
}

func errf(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, errf("sqlite open: %v", err)
	}

	m := migrate.New(db, migrate.SQLiteDialect{})

	if _, err := m.Apply(embeddedMigrations); err != nil {
		return nil, errf("migration: %v", err)
	}

	return &Store{db: db}, nil
}
