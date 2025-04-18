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

// SecretsWithLabels returns all secrets along with all labels associated with each.
func (vlt *Vault) SecretsWithLabels(ctx context.Context) (map[int]store.LabeledSecret, error) {
	return vlt.store.SecretsByColumn(ctx, "")
}

// SecretsByLabels returns secrets that match any of the provided label patterns,
// along with all labels associated with each secret.
//
// If no patterns are provided, it returns all secrets along with all their labels.
func (vlt *Vault) SecretsByLabels(ctx context.Context, labelPatterns ...string) (map[int]store.LabeledSecret, error) {
	return vlt.store.SecretsByColumn(ctx, "label", labelPatterns...)
}

// SecretsByName returns secrets that match the provided name pattern,
// along with all labels associated with it.
//
// If no pattern is provided, it returns all secrets along with all their labels.
func (vlt *Vault) SecretsByName(ctx context.Context, namePattern string) (map[int]store.LabeledSecret, error) {
	return vlt.store.SecretsByColumn(ctx, "secret_name", namePattern)
}

// SecretsByIDs returns a map of secrets that match any of the provided IDs,
// along with all labels associated with each.
//
// If the IDs slice is empty, the function returns [store.ErrNoIDsProvided].
func (vlt *Vault) SecretsByIDs(ctx context.Context, ids []int) (map[int]store.LabeledSecret, error) {
	return vlt.store.SecretsByIDs(ctx, ids)
}

// SecretsByLabelsAndName returns secrets with labels that match any of the
// provided label and name glob patterns.
//
// If no label patterns are provided, it returns [store.ErrNoLabelsProvided].
func (vlt *Vault) SecretsByLabelsAndName(ctx context.Context, name string, labels ...string) (map[int]store.LabeledSecret, error) {
	return vlt.store.SecretsByLabelsAndName(ctx, name, labels...)
}

// Secret retrieves the secret for the given secret ID.
func (vlt *Vault) Secret(ctx context.Context, id int) (string, error) {
	return vlt.store.Secret(ctx, id)
}
