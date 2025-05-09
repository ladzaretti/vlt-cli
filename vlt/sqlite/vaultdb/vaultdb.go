package vaultdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	cmdutil "github.com/ladzaretti/vlt-cli/util"
	"github.com/ladzaretti/vlt-cli/vaulterrors"
	"github.com/ladzaretti/vlt-cli/vlt/types"
)

var (
	// ErrNoLabelsProvided indicates that no labels were provided as an argument.
	ErrNoLabelsProvided = errors.New("no labels provided")

	// ErrNoIDsProvided indicates that no ids were provided as an argument.
	ErrNoIDsProvided = errors.New("no IDs provided")
)

type VaultDB struct {
	db types.DBTX
}

func New(db types.DBTX) *VaultDB {
	return &VaultDB{
		db: db,
	}
}

// WithTx returns a new Store using the given transaction.
func (*VaultDB) WithTx(tx *sql.Tx) *VaultDB {
	return &VaultDB{
		db: tx,
	}
}

//nolint:gosec
const insertSecret = `
	INSERT INTO
		secrets (name, secret)
	VALUES
		($1, $2)
`

func (s *VaultDB) InsertNewSecret(ctx context.Context, name string, secret string) (int, error) {
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

func (s *VaultDB) UpdateSecret(ctx context.Context, id int, secret string) (n int64, retErr error) {
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

func (s *VaultDB) UpdateName(ctx context.Context, id int, name string) (n int64, retErr error) {
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
func (s *VaultDB) Secret(ctx context.Context, id int) (string, error) {
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

func (s *VaultDB) InsertLabel(ctx context.Context, name string, secretID int) (int64, error) {
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

func (s *VaultDB) DeleteLabel(ctx context.Context, name string, secretID int64) (int64, error) {
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
	value sql.NullString
	label sql.NullString
}

// SecretWithLabels represents a secret with some of its associated labels.
type SecretWithLabels struct {
	Name   string
	Secret string
	Labels []string
}

// SecretsWithLabels returns all secrets along with all labels associated with each.
func (s *VaultDB) SecretsWithLabels(ctx context.Context) (map[int]SecretWithLabels, error) {
	return s.secretsByColumn(ctx, "", "LEFT JOIN")
}

// SecretsByName returns secrets that match the provided name pattern,
// along with all labels associated with it.
//
// If no pattern is provided, it returns all secrets along with all their labels.
func (s *VaultDB) SecretsByName(ctx context.Context, namePattern string) (map[int]SecretWithLabels, error) {
	return s.secretsByColumn(ctx, "secret_name", "LEFT JOIN", namePattern)
}

// SecretsByLabels returns secrets that match any of the provided label patterns,
// along with all labels associated with each secret that matches the labelPatterns.
//
// If no patterns are provided, an [vaulterrors.ErrMissingLabels] error is returned.
func (s *VaultDB) SecretsByLabels(ctx context.Context, labelPatterns ...string) (map[int]SecretWithLabels, error) {
	if len(labelPatterns) == 0 {
		return nil, vaulterrors.ErrMissingLabels
	}

	return s.secretsByColumn(ctx, "label", "INNER JOIN", labelPatterns...)
}

// secretsByColumn returns secrets with labels that
// match a where clause with all glob patterns for the given col.
//
// If no patterns are provided, no where clause is generated.
func (s *VaultDB) secretsByColumn(ctx context.Context, col string, join string, patterns ...string) (map[int]SecretWithLabels, error) {
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
func (s *VaultDB) SecretsByIDs(ctx context.Context, ids []int) (map[int]SecretWithLabels, error) {
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
func (s *VaultDB) SecretsByLabelsAndName(ctx context.Context, name string, labels ...string) (map[int]SecretWithLabels, error) {
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
func (s *VaultDB) secretsJoinLabels(ctx context.Context, query string, args ...any) (map[int]SecretWithLabels, error) {
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

// ExportSecrets exports all secret-related data stored in the database.
func (s *VaultDB) ExportSecrets(ctx context.Context) (map[int]SecretWithLabels, error) {
	query := `	
	SELECT
		s.id,
		s.name AS secret_name,
		s.secret,
		l.name AS label
	FROM
		secrets s
		JOIN labels l ON s.id = l.secret_id;
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }() //nolint:wsl

	var secrets []secretWithLabelRow
	for rows.Next() {
		var secret secretWithLabelRow
		if err := rows.Scan(&secret.id, &secret.name, &secret.value, &secret.label); err != nil {
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
func (s *VaultDB) DeleteByIDs(ctx context.Context, ids []int) (int64, error) {
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

	for _, secret := range secrets {
		v, ok := m[secret.id]
		if !ok {
			v = SecretWithLabels{
				Name:   secret.name,
				Labels: []string{},
			}
		}

		if secret.label.Valid {
			v.Labels = append(v.Labels, secret.label.String)
		}

		if len(v.Secret) == 0 && secret.value.Valid {
			v.Secret = secret.value.String
		}

		m[secret.id] = v
	}

	return m
}
