package schemaops

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ladzaretti/migrate/types"
)

var ErrNoSchemaVersion = errors.New("no schema version found")

func CreateTable(ctx context.Context, db types.CoreDB, dialect types.Dialect) error {
	return execContext(ctx, db, dialect.CreateVersionTableQuery())
}

func CurrentVersion(ctx context.Context, db types.CoreDB, dialect types.Dialect) (*types.SchemaVersion, error) {
	row := db.QueryRowContext(ctx, dialect.CurrentVersionQuery())

	return scanVersion(row)
}

func SaveVersion(ctx context.Context, db types.CoreDB, dialect types.Dialect, s types.SchemaVersion) error {
	return execContext(ctx, db, dialect.SaveVersionQuery(), s.Version, s.Checksum)
}

func execContext(ctx context.Context, db types.CoreDB, query string, args ...any) error {
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("exec context: %v", err)
	}

	return nil
}

func scanVersion(row *sql.Row) (*types.SchemaVersion, error) {
	ver := types.SchemaVersion{}

	if err := row.Scan(&ver.ID, &ver.Version, &ver.Checksum); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoSchemaVersion
		}

		return &types.SchemaVersion{}, fmt.Errorf("scan schema version: %v", err)
	}

	return &ver, nil
}
