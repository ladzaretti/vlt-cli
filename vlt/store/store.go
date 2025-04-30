package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	cmdutil "github.com/ladzaretti/vlt-cli/util"
	"github.com/ladzaretti/vlt-cli/vaulterrors"
)

var (
	// ErrNoLabelsProvided indicates that no labels were provided as an argument.
	ErrNoLabelsProvided = errors.New("no labels provided")

	// ErrNoIDsProvided indicates that no ids were provided as an argument.
	ErrNoIDsProvided = errors.New("no IDs provided")
)

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

func (s *Store) InsertNewSecret(ctx context.Context, name string, secret string) (int, error) {
	res, err := s.db.ExecContext(ctx, insertSecret, name, secret)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

const updateSecret = `
	UPDATE secrets
	SET
		secret = $1
	WHERE
		id = $2
`

func (s *Store) UpdateSecret(ctx context.Context, id int, secret string) (n int64, retErr error) {
	res, err := s.db.ExecContext(ctx, updateSecret, secret, id)
	if err != nil {
		return 0, err
	}

	n, err = res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return n, nil
}

const updateName = `
	UPDATE secrets
	SET
		name = $1
	WHERE
		id = $2
`

func (s *Store) UpdateName(ctx context.Context, id int, name string) (n int64, retErr error) {
	res, err := s.db.ExecContext(ctx, updateName, name, id)
	if err != nil {
		return 0, err
	}

	n, err = res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return n, nil
}

//nolint:gosec
const selectSecret = `
	SELECT
		secret
	FROM
		secrets
	WHERE
		id = $1
`

// secret returns the secret associated with the given secret id.
func (s *Store) Secret(ctx context.Context, id int) (string, error) {
	var secret string

	err := s.db.QueryRowContext(ctx, selectSecret, id).Scan(&secret)
	if err != nil {
		return "", err
	}

	return secret, nil
}

const insertLabel = `
	INSERT INTO
		labels (name, secret_id)
	VALUES
		($1, $2) ON CONFLICT (name, secret_id) DO NOTHING
`

func (s *Store) InsertLabel(ctx context.Context, name string, secretID int) (int64, error) {
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

const deleteLabel = `
	DELETE FROM labels
	WHERE
		name = $1
		AND secret_id = $2
`

func (s *Store) DeleteLabel(ctx context.Context, name string, secretID int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, deleteLabel, name, secretID)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

// secretWithLabelRow represents a row resulting from a join
// between the secrets and labels tables.
type secretWithLabelRow struct {
	id    int
	name  string
	label sql.NullString
}

// SecretWithLabels represents a secret with some of its associated labels.
type SecretWithLabels struct {
	Name   string
	Labels []string
}

// SecretsWithLabels returns all secrets along with all labels associated with each.
func (s *Store) SecretsWithLabels(ctx context.Context) (map[int]SecretWithLabels, error) {
	return s.secretsByColumn(ctx, "", "LEFT JOIN")
}

// SecretsByName returns secrets that match the provided name pattern,
// along with all labels associated with it.
//
// If no pattern is provided, it returns all secrets along with all their labels.
func (s *Store) SecretsByName(ctx context.Context, namePattern string) (map[int]SecretWithLabels, error) {
	return s.secretsByColumn(ctx, "secret_name", "LEFT JOIN", namePattern)
}

// SecretsByLabels returns secrets that match any of the provided label patterns,
// along with all labels associated with each secret that matches the labelPatterns.
//
// If no patterns are provided, an [vaulterrors.ErrMissingLabels] error is returned.
func (s *Store) SecretsByLabels(ctx context.Context, labelPatterns ...string) (map[int]SecretWithLabels, error) {
	if len(labelPatterns) == 0 {
		return nil, vaulterrors.ErrMissingLabels
	}

	return s.secretsByColumn(ctx, "label", "INNER JOIN", labelPatterns...)
}

// secretsByColumn returns secrets with labels that
// match a where clause with all glob patterns for the given col.
//
// If no patterns are provided, no where clause is generated.
func (s *Store) secretsByColumn(ctx context.Context, col string, join string, patterns ...string) (map[int]SecretWithLabels, error) {
	query := fmt.Sprintf(`
	SELECT
		s.id,
		s.name AS secret_name,
		l.name AS label
	FROM
		secrets s
		%s labels l ON s.id = l.secret_id
	%s
	`, join, whereGlobOrClause(col, patterns...))

	return s.secretsJoinLabels(ctx, query, cmdutil.ToAnySlice(patterns)...)
}

// SecretsByIDs returns a map of secrets and their labels for the given IDs.
//
// If the IDs slice is empty, the function returns [ErrNoIDsProvided].
func (s *Store) SecretsByIDs(ctx context.Context, ids []int) (map[int]SecretWithLabels, error) {
	if len(ids) == 0 {
		return nil, ErrNoIDsProvided
	}

	placeholders := make([]string, len(ids))
	for i := range ids {
		placeholders[i] = "?"
	}

	query := `
	SELECT
		s.id,
		s.name,
		l.name AS label
	FROM
		secrets s
		LEFT JOIN labels l ON s.id = l.secret_id
	WHERE
		s.id IN (` + strings.Join(placeholders, ",") + ")"

	return s.secretsJoinLabels(ctx, query, cmdutil.ToAnySlice(ids)...)
}

// SecretsByLabelsAndName returns secrets with labels that match any of the
// provided label and name glob patterns.
//
// If no label patterns are provided, it returns [ErrNoLabelsProvided].
func (s *Store) SecretsByLabelsAndName(ctx context.Context, name string, labels ...string) (map[int]SecretWithLabels, error) {
	if len(labels) == 0 {
		return nil, ErrNoLabelsProvided
	}

	query := `
	SELECT
		s.id,
		s.name AS secret_name,
		l.name AS label
	FROM
		secrets s
		JOIN labels l ON s.id = l.secret_id
	` + whereGlobOrClause("label", labels...) +
		"AND secret_name GLOB ?"

	args := append(cmdutil.ToAnySlice(labels), name)

	return s.secretsJoinLabels(ctx, query, args...)
}

// secretsJoinLabels executes a query to join secrets with their labels.
func (s *Store) secretsJoinLabels(ctx context.Context, query string, args ...any) (map[int]SecretWithLabels, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }() //nolint:wsl

	var secrets []secretWithLabelRow
	for rows.Next() {
		var secret secretWithLabelRow
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

// DeleteByIDs deletes secrets by their IDs, along with their labels.
func (s *Store) DeleteByIDs(ctx context.Context, ids []int) (int64, error) {
	if len(ids) == 0 {
		return 0, ErrNoIDsProvided
	}

	placeholders := make([]string, len(ids))
	for i := range ids {
		placeholders[i] = "?"
	}

	query := `
	DELETE 
	FROM 
		secrets
	WHERE
		id IN (` + strings.Join(placeholders, ",") + ")"

	res, err := s.db.ExecContext(ctx, query, cmdutil.ToAnySlice(ids)...)
	if err != nil {
		return 0, err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return n, err
	}

	return n, nil
}

// whereGlobOrClause generates a WHERE GLOB OR clause
// for the given column and patterns.
func whereGlobOrClause(col string, patterns ...string) string {
	if len(patterns) == 0 {
		return ""
	}

	clauses := make([]string, len(patterns))
	for i := range clauses {
		clauses[i] = col + " GLOB ?"
	}

	return "WHERE " + strings.Join(clauses, " OR ")
}

func reduce(secrets []secretWithLabelRow) map[int]SecretWithLabels {
	m := make(map[int]SecretWithLabels)

	for _, s := range secrets {
		v, ok := m[s.id]
		if !ok {
			v = SecretWithLabels{
				Name:   s.name,
				Labels: []string{},
			}
		}

		if s.label.Valid {
			v.Labels = append(v.Labels, s.label.String)
		}

		m[s.id] = v
	}

	return m
}
