package types

import (
	"context"
	"database/sql"
)

// CoreDB defines a minimal database interface for executing SQL queries.
type CoreDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// DBTX defines a database interface that supports query execution and transactions.
type DBTX interface {
	CoreDB
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// Dialect defines the necessary methods required
// to handle schema versioning during migrations.
//
// An acceptance test [migratetest.TestDialect] is available for
// verifying custom-defined Dialects.
type Dialect interface {
	// CreateVersionTableQuery returns the SQL query for creating the schema version table.
	//
	// The schema version table must include columns to store the following data:
	// 	- A column for the row ID,
	// 	- A column for the schema version number,
	// 	- A column for the checksum string.
	CreateVersionTableQuery() string

	// CurrentVersionQuery returns the SQL query for retrieving the current schema version.
	//
	// This query must return at most one row of data.
	// The returned columns should be ordered as follows: row ID,
	// followed by the schema version number, and then the checksum.
	CurrentVersionQuery() string

	// SaveVersionQuery returns the SQL query for upserting the schema version.
	//
	// It upserts the row with a static ID of 0, updating the version and checksum.
	// These values are provided as positional parameters in the order (version, checksum).
	SaveVersionQuery() string
}

// SchemaVersion represents the schema version information for the database.
type SchemaVersion struct {
	// ID is the schema version row ID.
	ID int

	// Version is the current schema version number.
	Version int

	// Checksum is the cumulative checksum of all applied migrations.
	Checksum string
}

func (s *SchemaVersion) Equal(o *SchemaVersion) bool {
	if s == o {
		return true
	}

	if s == nil || o == nil {
		return false
	}

	return s.ID == o.ID && s.Version == o.Version && s.Checksum == o.Checksum
}
