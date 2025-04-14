package vlt

import (
	"context"
	"database/sql"
	"embed"
	"errors"
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

// Vault represents a connection to a vault.
type Vault struct {
	Path  string       // Path is the path to the underlying SQLite file.
	db    *sql.DB      // db is the connection to the database.
	store *store.Store // store provides methods to interact with the vault data.
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

func (vlt *Vault) SetMasterKey(ctx context.Context, k string) error {
	return vlt.store.InsertMasterKey(ctx, k)
}

func (vlt *Vault) GetMasterKey(ctx context.Context) (string, error) {
	return vlt.store.QueryMasterKey(ctx)
}

// InsertNewSecret inserts a new secret with its labels
// into the vault using a transaction.
//
// Returns the ID of the inserted secret or an error if the operation fails.
func (vlt *Vault) InsertNewSecret(ctx context.Context, name string, secret string, labels []string) (id int64, retErr error) {
	tx, err := vlt.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}

	storeTx := vlt.store.WithTx(tx)

	secretID, err := storeTx.InsertNewSecret(ctx, name, secret)
	if err != nil {
		if err2 := tx.Rollback(); err2 != nil {
			return 0, errf("insert new secret: rollback: %w", errors.Join(err2, err))
		}

		return 0, errf("insert new secret: %w", err)
	}

	for _, l := range labels {
		if _, err := storeTx.InsertLabel(ctx, l, secretID); err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				return 0, errf("insert label: rollback: %w", errors.Join(err2, err))
			}

			return 0, errf("insert label: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, errf("tx commit: %w", err)
	}

	return secretID, nil
}

func (vlt *Vault) UpsertSecret(ctx context.Context, name, secret string) (int64, error) {
	return vlt.store.UpsertSecret(ctx, name, secret)
}

func (vlt *Vault) SecretsByLabels(ctx context.Context, labels []string) (map[int]store.LabeledSecret, error) {
	return vlt.store.SecretsByLabels(ctx, labels)
}
