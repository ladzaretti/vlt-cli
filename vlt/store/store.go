package store

import (
	"context"
	"database/sql"
)

type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Store struct {
	db DBTX
}

func New(db DBTX) *Store {
	return &Store{
		db: db,
	}
}

func (*Store) WithTx(tx *sql.Tx) *Store {
	return &Store{
		db: tx,
	}
}

const insertMasterKey = `
	INSERT INTO
		master_key (id, key)
	VALUES
		(0, $1) ON CONFLICT (id) DO NOTHING
`

func (s *Store) InsertMasterKey(ctx context.Context, key string) error {
	if _, err := s.db.ExecContext(ctx, insertMasterKey, key); err != nil {
		return err
	}

	return nil
}

const selectMasterKey = `
	SELECT
		key
	FROM
		master_key
	WHERE
		id = 0
`

func (s *Store) QueryMasterKey(ctx context.Context) (string, error) {
	var masterKey string

	err := s.db.QueryRowContext(ctx, selectMasterKey).Scan(&masterKey)
	if err != nil {
		return "", err
	}

	return masterKey, nil
}
