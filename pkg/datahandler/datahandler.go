package datahandler

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

type DataHandler struct {
	db DBTX
}

func New(db DBTX) *DataHandler {
	return &DataHandler{
		db: db,
	}
}

func (*DataHandler) WithTx(tx *sql.Tx) *DataHandler {
	return &DataHandler{
		db: tx,
	}
}
