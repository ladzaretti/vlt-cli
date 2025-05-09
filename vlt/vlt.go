package vlt

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/ladzaretti/migrate"
	"github.com/ladzaretti/vlt-cli/vaultcrypto"
	"github.com/ladzaretti/vlt-cli/vlt/sqlite/vaultcontainer"
	"github.com/ladzaretti/vlt-cli/vlt/sqlite/vaultdb"

	// Package sqlite is a CGo-free port of SQLite/SQLite3.
	_ "modernc.org/sqlite"
)

var (
	//go:embed db/migrations/sqlite/vault_container
	masterFS embed.FS

	containerMigrations = migrate.EmbeddedMigrations{
		FS:   masterFS,
		Path: "db/migrations/sqlite/vault_container",
	}

	//go:embed db/migrations/sqlite/vault
	vaultFS embed.FS

	vaultMigrations = migrate.EmbeddedMigrations{
		FS:   vaultFS,
		Path: "db/migrations/sqlite/vault",
	}
)

// Vault represents a connection to a vault.
type Vault struct {
	Path   string              // Path is the path to the underlying SQLite file.
	conn   *sql.Conn           // conn is the connection to the database.
	aesgcm *vaultcrypto.AESGCM // aesgcm is used for cryptographic ops on the vault data.
	nonce  []byte              // nonce is the cryptographic nonce used to encrypt the serialized vault data.
	db     *vaultdb.VaultDB    // store provides methods to interact with the vault data.
}

func New(ctx context.Context, password string, path string) (*Vault, error) {
	vc, cleanup, err := openVaultContainer(path)
	if err != nil {
		return nil, errf("new: %w", err)
	}
	defer func() { _ = cleanup() }()

	cipherdata, err := vaultCipherData([]byte(password))
	if err != nil {
		return nil, errf("new: %w", err)
	}

	phc, err := vaultcrypto.DecodeAragon2idPHC(cipherdata.KDFPHC)
	if err != nil {
		return nil, errf("new: %w", err)
	}

	aes, err := deriveVaultAES(phc, password)
	if err != nil {
		return nil, err
	}

	v := &Vault{Path: path, aesgcm: aes, nonce: cipherdata.Nonce}
	if err := v.open(ctx, nil); err != nil {
		return nil, errf("new: %w", err)
	}

	serialized, err := Serialize(v.conn)
	if err != nil {
		return nil, errf("new: %w", err)

	}

	ciphervault, err := aes.Seal(cipherdata.Nonce, serialized)
	if err != nil {
		return nil, errf("new: %w", err)
	}

	if err := vc.InsertNewVault(ctx, cipherdata.AuthPHC, cipherdata.KDFPHC, cipherdata.Nonce, ciphervault); err != nil {
		return nil, errf("new: %w", err)
	}

	return v, nil
}

func Open(ctx context.Context, password string, path string) (*Vault, error) {
	vc, cleanup, err := openVaultContainer(path)
	if err != nil {
		return nil, errf("open: %w", err)
	}
	defer func() { _ = cleanup() }()

	cipherData, err := vc.SelectVault(ctx)
	if err != nil {
		return nil, errf("open: %w", err)
	}

	phc, err := vaultcrypto.DecodeAragon2idPHC(cipherData.KDFPHC)
	if err != nil {
		return nil, errf("open: %w", err)
	}

	aes, err := deriveVaultAES(phc, password)
	if err != nil {
		return nil, err
	}

	v := &Vault{Path: path, aesgcm: aes, nonce: cipherData.Nonce}
	if err := v.open(ctx, cipherData.Vault); err != nil {
		return nil, errf("open: %w", err)
	}

	return v, nil
}

func openVaultContainer(path string) (_ *vaultcontainer.VaultContainer, cleanup func() error, _ error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, nil, errf("sqlite open: %v", err)
	}

	cleanup = db.Close

	m := migrate.New(db, migrate.SQLiteDialect{})
	_, err = m.Apply(containerMigrations)
	if err != nil {
		err2 := cleanup()
		return nil, nil, errf("vault container migration: %v", errors.Join(err, err2))
	}

	return vaultcontainer.New(db), cleanup, nil
}

// vaultCipherData generates [vaultcontainer.CipherData] containing salts, nonce,
// and derived authentication hash used for password authentication and vault encryption.
func vaultCipherData(password []byte) (vaultcontainer.CipherData, error) {
	authSalt, err := vaultcrypto.RandBytes(16)
	if err != nil {
		return vaultcontainer.CipherData{}, errf("cipher data: %w", err)
	}

	authKDF := vaultcrypto.NewArgon2idKDF(vaultcrypto.WithSalt(authSalt))
	authPHC := authKDF.PHC()
	authPHC.Hash = authKDF.Derive(password)

	vaultSalt, err := vaultcrypto.RandBytes(16)
	if err != nil {
		return vaultcontainer.CipherData{}, errf("cipher data: %w", err)
	}

	vaultKDF := vaultcrypto.NewArgon2idKDF(vaultcrypto.WithSalt(vaultSalt))

	vaultNonce, err := vaultcrypto.RandBytes(12)
	if err != nil {
		return vaultcontainer.CipherData{}, errf("cipher data: %w", err)
	}

	return vaultcontainer.CipherData{
		AuthPHC: authPHC.String(),
		KDFPHC:  vaultKDF.PHC().String(),
		Nonce:   vaultNonce,
	}, nil
}

func (v *Vault) open(ctx context.Context, ciphervault []byte) error {
	vaultDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return errf("open in-memory vault: %w", err)
	}

	conn, err := vaultDB.Conn(ctx)
	if err != nil {
		return errf("open vault in-memory connection: %w", err)
	}

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return errf("enable sqlite foreign keys support: %w", err)
	}

	if ciphervault != nil {
		decrypted, err := v.aesgcm.Open(v.nonce, ciphervault)
		if err != nil {
			return fmt.Errorf("decrypt vault: %w", err)
		}

		if err := Deserialize(conn, decrypted); err != nil {
			return errf("deserialize vault: %w", err)
		}
	}

	m := migrate.New(conn, migrate.SQLiteDialect{})

	_, err = m.Apply(vaultMigrations)
	if err != nil {
		return errf("vault migration: %v", err)
	}

	v.conn = conn
	v.db = vaultdb.New(conn)

	return nil
}

// deriveVaultAES derives an AES-GCM cipher using the given PHC and password.
// The [vaultcrypto.Argon2idPHC] provides the key derivation parameters,
// and the password is used to derive the encryption key.
func deriveVaultAES(phc vaultcrypto.Argon2idPHC, password string) (*vaultcrypto.AESGCM, error) {
	kdf := vaultcrypto.NewArgon2idKDF(vaultcrypto.WithPHC(phc))

	key := kdf.Derive([]byte(password))

	aes, err := vaultcrypto.NewAESGCM(key)
	if err != nil {
		return nil, errf("vault aesgsm: %w", err)
	}

	return aes, nil
}

func errf(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

// InsertNewSecret inserts a new secret with its labels
// into the vault using a transaction.
//
// Returns the ID of the inserted secret or an error if the operation fails.
func (vlt *Vault) InsertNewSecret(ctx context.Context, name string, secret string, labels []string) (id int, retErr error) {
	tx, err := vlt.conn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}

	storeTx := vlt.db.WithTx(tx)

	secretID, err := storeTx.InsertNewSecret(ctx, name, secret)
	if err != nil {
		if err2 := tx.Rollback(); err2 != nil {
			return 0, errf("insert new secret: rollback: %w", errors.Join(err2, err))
		}

		return 0, errf("insert new secret: %w", err)
	}

	for _, l := range labels {
		if _, err := storeTx.InsertLabel(ctx, l, secretID); err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				return 0, errf("insert label: rollback: %w", errors.Join(err2, err))
			}

			return 0, errf("insert label: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, errf("tx commit: %w", err)
	}

	return secretID, nil
}

// UpdateSecretMetadata updates the metadata of the secret identified by id.
func (vlt *Vault) UpdateSecretMetadata(ctx context.Context, id int, newName string, removeLabels []string, addLabels []string) error {
	tx, err := vlt.conn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	updateTx := vlt.db.WithTx(tx)

	if len(newName) > 0 {
		_, err = vlt.db.UpdateName(ctx, id, newName)
		if err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				return errf("update secret name: rollback: %w", errors.Join(err2, err))
			}

			return errf("update secret name: %w", err)
		}
	}

	for _, l := range addLabels {
		if _, err := updateTx.InsertLabel(ctx, l, id); err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				return errf("insert label: rollback: %w", errors.Join(err2, err))
			}

			return errf("insert label: %w", err)
		}
	}

	for _, l := range removeLabels {
		if _, err := updateTx.DeleteLabel(ctx, l, int64(id)); err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				return errf("remote label: rollback: %w", errors.Join(err2, err))
			}

			return errf("remove label: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return errf("tx commit: %w", err)
	}

	return nil
}

// UpdateSecret updates the secret value of the secret identified by id.
func (vlt *Vault) UpdateSecret(ctx context.Context, id int, secret string) (int64, error) {
	return vlt.db.UpdateSecret(ctx, id, secret)
}

// SecretsWithLabels returns all secrets along with all labels associated with each.
func (vlt *Vault) SecretsWithLabels(ctx context.Context) (map[int]vaultdb.SecretWithLabels, error) {
	return vlt.db.SecretsWithLabels(ctx)
}

// SecretsByLabels returns secrets that match any of the provided label patterns,
// along with all labels associated with each secret.
//
// If no patterns are provided, it returns all secrets along with all their labels.
func (vlt *Vault) SecretsByLabels(ctx context.Context, labelPatterns ...string) (map[int]vaultdb.SecretWithLabels, error) {
	return vlt.db.SecretsByLabels(ctx, labelPatterns...)
}

// ExportSecrets exports all secret-related data stored in the database.
func (vlt *Vault) ExportSecrets(ctx context.Context) (map[int]vaultdb.SecretWithLabels, error) {
	return vlt.db.ExportSecrets(ctx)
}

// SecretsByName returns secrets that match the provided name pattern,
// along with all labels associated with it.
//
// If no pattern is provided, it returns all secrets along with all their labels.
func (vlt *Vault) SecretsByName(ctx context.Context, namePattern string) (map[int]vaultdb.SecretWithLabels, error) {
	return vlt.db.SecretsByName(ctx, namePattern)
}

// SecretsByIDs returns a map of secrets that match any of the provided IDs,
// along with all labels associated with each.
//
// If the IDs slice is empty, the function returns [vaultdb.ErrNoIDsProvided].
func (vlt *Vault) SecretsByIDs(ctx context.Context, ids ...int) (map[int]vaultdb.SecretWithLabels, error) {
	return vlt.db.SecretsByIDs(ctx, ids)
}

// SecretsByLabelsAndName returns secrets with labels that match any of the
// provided label and name glob patterns.
//
// If no label patterns are provided, it returns [vaultdb.ErrNoLabelsProvided].
func (vlt *Vault) SecretsByLabelsAndName(ctx context.Context, name string, labels ...string) (map[int]vaultdb.SecretWithLabels, error) {
	return vlt.db.SecretsByLabelsAndName(ctx, name, labels...)
}

// Secret retrieves the secret for the given secret ID.
func (vlt *Vault) Secret(ctx context.Context, id int) (string, error) {
	return vlt.db.Secret(ctx, id)
}

// DeleteByIDs deletes secrets by their IDs, along with their labels.
func (vlt *Vault) DeleteByIDs(ctx context.Context, ids ...int) (int64, error) {
	return vlt.db.DeleteByIDs(ctx, ids)
}
