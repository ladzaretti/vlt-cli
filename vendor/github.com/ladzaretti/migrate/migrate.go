//
// This is free and unencumbered software released into the public domain.
//
// Anyone is free to copy, modify, publish, use, compile, sell, or
// distribute this software, either in source code form or as a compiled
// binary, for any purpose, commercial or non-commercial, and by any
// means.
//
// In jurisdictions that recognize copyright laws, the author or authors
// of this software dedicate any and all copyright interest in the
// software to the public domain. We make this dedication for the benefit
// of the public at large and to the detriment of our heirs and
// successors. We intend this dedication to be an overt act of
// relinquishment in perpetuity of all present and future rights to this
// software under copyright law.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
// IN NO EVENT SHALL THE AUTHORS BE LIABLE FOR ANY CLAIM, DAMAGES OR
// OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
// OTHER DEALINGS IN THE SOFTWARE.
//
// For more information, please refer to <https://unlicense.org/>

package migrate

import (
	"context"
	//nolint:gosec // in this context, SHA-1 is for change detection, not security.
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/ladzaretti/migrate/internal/schemaops"
	"github.com/ladzaretti/migrate/types"
)

// Checksum computes a hash value for the given string.
// It is used to validate and compare migration scripts.
type Checksum func(s string) string

// Filter is used to filter migrations by their index
// in the execution order. Return true to apply the migration.
type Filter func(migrationIndex int) bool

type Migrator struct {
	db                     types.DBTX
	dialect                types.Dialect
	migrationFilter        Filter
	checksum               Checksum
	withChecksumValidation bool
	withTx                 bool
	reapplyAll             bool
}

type Opt func(*Migrator)

// New creates a new Migrator with the provided database, dialect, and options.
//
// By default, both transactions and checksum validation are enabled. The checksum
// validation uses a SHA-1 function that ignores formatting (e.g., whitespaces).
// These defaults can be customized using the [Opt] functions.
func New(db types.DBTX, dialect types.Dialect, opts ...Opt) *Migrator {
	m := &Migrator{
		db:                     db,
		dialect:                dialect,
		migrationFilter:        func(_ int) bool { return true },
		checksum:               normalizedSha1,
		withChecksumValidation: true,
		withTx:                 true,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// WithChecksum sets a custom [Checksum] function or uses the default if nil.
func WithChecksum(fn Checksum) Opt {
	return func(m *Migrator) {
		if fn != nil {
			m.checksum = fn
		}
	}
}

func WithTransaction(enabled bool) Opt {
	return func(m *Migrator) {
		m.withTx = enabled
	}
}

func WithChecksumValidation(enabled bool) Opt {
	return func(m *Migrator) {
		m.withChecksumValidation = enabled
	}
}

// WithFilter is used to set a filtering function
// to exclude certain scripts from being applied.
//
// Example:
//
//	// Skip the 4th migration
//	skipForth := func(n int) bool { return n != 4 }
//
//	m := migrate.New(db, s.dialect, migrate.WithFilter(skipForth))
//	n, err := m.Apply(migrations)
func WithFilter(fn Filter) Opt {
	return func(m *Migrator) {
		m.migrationFilter = fn
	}
}

// WithReapplyAll controls whether to reapply existing migrations.
func WithReapplyAll(enabled bool) Opt {
	return func(m *Migrator) {
		m.reapplyAll = enabled
	}
}

func errf(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

// Apply applies the given migrations in the order they are provided.
// Only unapplied migrations are applied.
// That is, if the current schema version is n and n + k scripts are provided,
// only the additional k will be applied.
//
// To re-apply all migrations, use the [WithReapplyAll] [Opt] function.
//
// The initial schema state is considered version 0.
//
// For each schema version, a cumulative checksum is calculated,
// considering all previously applied migrations.
// If an already applied migration has changed,
// validation will fail, and no further migrations will be applied.
//
// It returns the number of migrations applied and any error encountered.
//
// With transactions enabled (default), any error triggers a rollback;
// otherwise, migrations are applied sequentially until an error occurs or all are applied.
//
// To reset the schema and force re-application of migrations,
// along with re-generating checksum values, use the following:
//
//	opts := []migrate.Opts{
//		migrate.WithChecksumValidation(false),
//		migrate.WithReapplyAll(true),
//	}
//	m := migrate.New(db, s.dialect, opts...)
//	m.Apply(migrations)
func (m *Migrator) Apply(from Lister) (int, error) {
	return m.ApplyContext(context.Background(), from)
}

func (m *Migrator) ApplyContext(ctx context.Context, from Lister) (int, error) {
	migrations, err := from.List()
	if err != nil {
		return 0, errf("list migrations source: %v", err)
	}

	if err := schemaops.CreateTable(ctx, m.db, m.dialect); err != nil {
		return 0, errf("create schema version table: %v", err)
	}

	schema, err := m.CurrentSchemaVersion(ctx)
	if err != nil {
		return 0, errf("current schema version: %v", err)
	}

	if schema.Version > len(migrations) {
		return 0, errf("database version (%d) exceeds available migrations (%d)", schema.Version, len(migrations))
	}

	runtimeChecksum := m.checksumHistory(migrations)
	if err := m.validateChecksum(schema, runtimeChecksum); err != nil {
		return 0, errf("schema integrity check failed: %v", err)
	}

	if !m.reapplyAll && schema.Version >= len(migrations) {
		return 0, nil // already up to date
	}

	if !m.withTx {
		n, err := m.applyMigrations(ctx, m.db, schema.Version, migrations, runtimeChecksum)
		if err != nil {
			return n, errf("non-transactional migration: %w", err)
		}

		return n, err
	}

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, errf("start transaction: %v", err)
	}

	n, err := m.applyMigrations(ctx, tx, schema.Version, migrations, runtimeChecksum)
	if err != nil {
		if err2 := tx.Rollback(); err2 != nil {
			return 0, errf("rollback: %v", errors.Join(err2, err))
		}

		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, errf("transaction commit: %v", err)
	}

	return n, err
}

func (m *Migrator) CurrentSchemaVersion(ctx context.Context) (types.SchemaVersion, error) {
	schema, err := schemaops.CurrentVersion(ctx, m.db, m.dialect)
	if err != nil && !errors.Is(err, schemaops.ErrNoSchemaVersion) {
		//nolint:wrapcheck // error is returned from an internal package
		return types.SchemaVersion{}, err
	}

	if schema != nil {
		return *schema, nil
	}

	return types.SchemaVersion{}, nil
}

func (m *Migrator) applyMigrations(ctx context.Context, db types.CoreDB, current int, migrations []string, checksums []string) (n int, retErr error) {
	if len(migrations)+1 != len(checksums) {
		retErr = errf("mismatched migrations and checksums: expected %d checksums (+1 for initial state), but found %d", len(migrations), len(checksums))
		return
	}

	from := current
	if m.reapplyAll {
		from = 0
	}

	for i := from; i < len(migrations); i++ {
		if !m.migrationFilter(i + 1) {
			continue
		}

		sch := types.SchemaVersion{Version: i + 1, Checksum: checksums[i+1]}
		if err := applyMigration(ctx, db, m.dialect, sch, migrations[i]); err != nil {
			retErr = errf("apply migration script %d: %v", i+1, err)
			return
		}

		n++
	}

	return
}

func (m *Migrator) checksumHistory(migrations []string) []string {
	history := make([]string, len(migrations)+1)
	history[0] = "" // version 0 has no migrations applied

	for i, mig := range migrations {
		history[i+1] = m.checksum(history[i] + m.checksum(mig))
	}

	return history
}

func (m *Migrator) validateChecksum(schema types.SchemaVersion, runtimeChecksum []string) error {
	if !m.withChecksumValidation {
		return nil
	}

	if schema.Version == 0 {
		return nil
	}

	if schema.Checksum != runtimeChecksum[schema.Version] {
		return errf("runtime checksum %q != database checksum %q", runtimeChecksum[schema.Version], schema.Checksum)
	}

	return nil
}

func applyMigration(ctx context.Context, db types.CoreDB, dialect types.Dialect, schema types.SchemaVersion, migration string) error {
	if err := execContext(ctx, db, migration); err != nil {
		return err
	}

	if err := schemaops.SaveVersion(ctx, db, dialect, schema); err != nil {
		//nolint:wrapcheck // error is returned from an internal package
		return err
	}

	return nil
}

func execContext(ctx context.Context, db types.CoreDB, query string, args ...any) error {
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("exec context: %v", err)
	}

	return nil
}

func normalizedSha1(query string) string {
	normalized := normalize(query)
	//nolint:gosec // in this context, SHA-1 is for change detection, not security.
	hash := sha1.Sum([]byte(normalized))

	return hex.EncodeToString(hash[:])
}

func normalize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1 // Remove whitespace
		}

		return r
	}, s)
}
