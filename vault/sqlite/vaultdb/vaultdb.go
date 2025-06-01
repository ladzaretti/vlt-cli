package vaultdb

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	cmdutil "github.com/ladzaretti/vlt-cli/util"
	"github.com/ladzaretti/vlt-cli/vault/types"
)

var (
	// ErrNoLabelsProvided indicates that no labels were provided as an argument.
	ErrNoLabelsProvided = errors.New("no labels provided")

	// ErrNoIDsProvided indicates that no ids were provided as an argument.
	ErrNoIDsProvided = errors.New("no IDs provided")
)

// VaultDB provides access to the vault's database.
// It handles storage and retrieval of vault secrets.
//
// This type does not perform cryptographic operations.
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
		secrets (name, nonce, ciphertext)
	VALUES
		(?, ?, ?)
`

func (s *VaultDB) InsertNewSecret(ctx context.Context, name string, nonce []byte, ciphertext []byte) (int, error) {
	res, err := s.db.ExecContext(ctx, insertSecret, name, nonce, ciphertext)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(id), nil
}

//nolint:gosec
const insertSecretWithID = `
	INSERT INTO
		secrets (id, name, nonce, ciphertext)
	VALUES
		(?, ?, ?, ?)
`

func (s *VaultDB) InsertNewSecretWithID(ctx context.Context, id int, name string, nonce []byte, ciphertext []byte) (int, error) {
	res, err := s.db.ExecContext(ctx, insertSecretWithID, id, name, nonce, ciphertext)
	if err != nil {
		return 0, err
	}

	insertID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return int(insertID), nil
}

const updateSecret = `
	UPDATE secrets
	SET
		nonce = ?,
		ciphertext = ?
	WHERE
		id = ?
`

func (s *VaultDB) UpdateSecret(ctx context.Context, id int, nonce []byte, ciphertext []byte) (n int64, retErr error) {
	res, err := s.db.ExecContext(ctx, updateSecret, nonce, ciphertext, id)
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
		nonce, ciphertext
	FROM
		secrets
	WHERE
		id = ?
`

// ShowSecret returns the secret ciphertext and nonce associated with the given secret id.
func (s *VaultDB) ShowSecret(ctx context.Context, id int) (nonce []byte, ciphertext []byte, err error) {
	err = s.db.QueryRowContext(ctx, selectSecret, id).Scan(&nonce, &ciphertext)
	if err != nil {
		return nonce, ciphertext, err
	}

	return nonce, ciphertext, err
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
	id         int
	name       string
	nonce      []byte
	ciphertext []byte
	label      sql.NullString
}

// SecretWithLabels represents a secret with some of its associated labels.
type SecretWithLabels struct {
	Name       string
	Nonce      []byte
	Ciphertext []byte
	Value      string
	Labels     []string
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

// Filters defines criteria for querying secrets from the vault.
// All fields support UNIX glob-style wildcard matching.
type Filters struct {
	// Wildcard matches either secret names or labels.
	// If set, it is ORed across both name and label fields.
	Wildcard string

	// Name filters secrets by name.
	Name string

	// Labels filters secrets by matching any of the provided label patterns.
	// Multiple labels are ORed.
	Labels []string
}

// FilterSecrets returns secrets that match the given filters.
func (s *VaultDB) FilterSecrets(ctx context.Context, m Filters) (map[int]SecretWithLabels, error) {
	query := `
		SELECT
			s.id,
			s.name,
			l.name AS label
		FROM
			secrets s
			LEFT JOIN labels l ON s.id = l.secret_id
	`

	var (
		args         []any
		whereClauses []string
	)

	if len(m.Wildcard) > 0 {
		whereClauses = append(whereClauses, "(s.name GLOB ? OR l.name GLOB ?)")
		args = append(args, m.Wildcard, m.Wildcard)
	}

	if len(m.Name) > 0 {
		whereClauses = append(whereClauses, "s.name GLOB ?")
		args = append(args, m.Name)
	}

	if len(m.Labels) > 0 {
		clauses := make([]string, len(m.Labels))
		for i := range clauses {
			clauses[i] = "l.name GLOB ?"
			args = append(args, m.Labels[i]) //nolint:wsl
		}

		whereClauses = append(whereClauses, "("+strings.Join(clauses, " OR ")+")")
	}

	if len(whereClauses) > 0 {
		query += " WHERE " + strings.Join(whereClauses, " AND ")
	}

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
		s.nonce,
		s.ciphertext,
		l.name AS label
	FROM
		secrets s
		LEFT JOIN labels l ON s.id = l.secret_id;
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }() //nolint:wsl

	var secrets []secretWithLabelRow
	for rows.Next() {
		var secret secretWithLabelRow
		if err := rows.Scan(&secret.id, &secret.name, &secret.nonce, &secret.ciphertext, &secret.label); err != nil {
			return nil, err
		}

		secrets = append(secrets, secret)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return reduce(secrets), nil
}

// DeleteSecretsByIDs deletes secrets by their IDs, along with their labels.
func (s *VaultDB) DeleteSecretsByIDs(ctx context.Context, ids []int) (int64, error) {
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

		if len(v.Ciphertext) == 0 {
			v.Ciphertext = secret.ciphertext
			v.Nonce = secret.nonce
		}

		m[secret.id] = v
	}

	return m
}
