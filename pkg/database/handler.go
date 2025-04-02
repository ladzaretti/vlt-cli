package database

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

type Handler struct {
	db DBTX
}

func NewHandler(db DBTX) *Handler {
	return &Handler{
		db: db,
	}
}

func (*Handler) WithTx(tx *sql.Tx) *Handler {
	return &Handler{
		db: tx,
	}
}

const saveMasterKey = `
	INSERT INTO
		master_key (id, key)
	VALUES
		(0, $1) ON CONFLICT (id) DO NOTHING
`

func (h *Handler) SaveMasterKey(ctx context.Context, key string) error {
	if _, err := h.db.ExecContext(ctx, saveMasterKey, key); err != nil {
		return err
	}

	return nil
}
