package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	cmdutil "github.com/ladzaretti/vlt-cli/pkg/util"
)

// ErrNoLabelsProvided indicates that no labels were provided as an argument.
var ErrNoLabelsProvided = errors.New("no labels provided")

// DBTX defines the subset of database operations used by [Store].
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

// WithTx returns a new Store using the given transaction.
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

//nolint:gosec
const insertSecret = `
	INSERT INTO
		secrets (name, secret)
	VALUES
		($1, $2)
`

func (s *Store) InsertNewSecret(ctx context.Context, name string, secret string) (int64, error) {
	res, err := s.db.ExecContext(ctx, insertSecret, name, secret)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

//nolint:gosec
const upsertNewSecret = `
	INSERT INTO
		secrets (id, secret)
	VALUES
		($1, $2) ON CONFLICT (id) DO
	UPDATE
	SET
		secret = EXCLUDED.secret;
`

func (s *Store) UpsertSecret(ctx context.Context, id string, secret string) (n int64, retErr error) {
	res, err := s.db.ExecContext(ctx, upsertNewSecret, id, secret)
	if err != nil {
		return 0, err
	}

	n, err = res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return n, nil
}

const insertLabel = `
	INSERT INTO
		labels (name, secret_id)
	VALUES
		($1, $2) ON CONFLICT (name, secret_id) DO NOTHING
`

func (s *Store) InsertLabel(ctx context.Context, name string, secretID int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, insertLabel, name, secretID)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

// secretLabelRow represents a row resulting from a join
// between the secrets and labels tables.
type secretLabelRow struct {
	id    int
	name  string
	label string
}

// LabeledSecret represents a secret with some of its associated labels.
type LabeledSecret struct {
	Name   string
	Labels []string
}

//nolint:gosec
const querySecretsByLabels = `
	SELECT
		s.id,
		s.name,
		l.name AS label
	FROM
		secrets s
		JOIN labels l ON s.id = l.secret_id
	WHERE
		l.name IN (%s)
`

// SecretsByLabels returns a map of secrets that have any of the provided labels.
// The returned map is keyed by secret ID, and each map value contains
// all labels from the provided labels slice that reference the given secret.
//
// If the labels slice is empty, the function returns [ErrNoLabelsProvided].
func (s *Store) SecretsByLabels(ctx context.Context, labels []string) (map[int]LabeledSecret, error) {
	if len(labels) == 0 {
		return nil, ErrNoLabelsProvided
	}

	placeholders := make([]string, len(labels))
	cmdutil.Fill(placeholders, "?")

	query := fmt.Sprintf(querySecretsByLabels, strings.Join(placeholders, ","))

	rows, err := s.db.QueryContext(ctx, query, cmdutil.ToAnySlice(labels)...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }() //nolint:wsl

	var secrets []secretLabelRow
	for rows.Next() {
		var secret secretLabelRow
		if err := rows.Scan(&secret.id, &secret.name, &secret.label); err != nil {
			return nil, err
		}

		secrets = append(secrets, secret)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return reduce(secrets), nil
}

func reduce(secrets []secretLabelRow) map[int]LabeledSecret {
	m := make(map[int]LabeledSecret)

	for _, s := range secrets {
		v, ok := m[s.id]
		if !ok {
			v = LabeledSecret{
				Name:   s.name,
				Labels: []string{s.label},
			}
		} else {
			v.Labels = append(v.Labels, s.label)
		}

		m[s.id] = v
	}

	return m
}
