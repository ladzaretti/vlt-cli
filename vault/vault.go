package vault

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"sync"

	"github.com/ladzaretti/vlt-cli/vault/sqlite/vaultcontainer"
	"github.com/ladzaretti/vlt-cli/vault/sqlite/vaultdb"
	"github.com/ladzaretti/vlt-cli/vaultcrypto"

	"github.com/ladzaretti/migrate"

	// Package sqlite is a CGo-free port of SQLite/SQLite3.
	_ "modernc.org/sqlite"
)

var ErrAuthenticationFailed = errors.New("authentication failed")

var (
	//go:embed db/migrations/sqlite/vault_container
	masterFS embed.FS

	vaultContainerMigrations = migrate.EmbeddedMigrations{
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

type cleanupFunc func() error

// Vault manages access to two related databases:
// the in-memory secret-holding vault and the on-disk vault container.
//
// The vault is loaded entirely into memory and holds the actual user data.
// This in-memory database is serialized, encrypted using AES-GCM, and then persisted within
// the vault container database.
//
// A user-supplied password is used to derive cryptographic keys via Argon2id.
type Vault struct {
	Path                 string                // Path to the underlying SQLite file.
	aesgcm               *vaultcrypto.AESGCM   // aesgcm is used for cryptographic ops on the vault data.
	nonce                []byte                // nonce is the cryptographic nonce used to encrypt the serialized vault database.
	conn                 *sql.Conn             // conn is the connection to the vault database, it is used for serializing and deserializing.
	db                   *vaultdb.VaultDB      // db provides an interface to the in-memory database holding the actual user data.
	buf                  []byte                // buf holds the backing in-memory SQLite database. retained to prevent GC while the DB is active, released in [Vault.Close].
	vaultContainerHandle *vaultContainerHandle // vaultContainerHandle connects to the vault container database.
	cleanupFuncs         []cleanupFunc         // cleanupFuncs contains deferred cleanup functions.
	closeOnce            sync.Once             // closeOnce protects [Vault.Close].
}

type session struct {
	key, nonce []byte
}

// config options for creating a [Vault] instance.
type config struct {
	snapshot []byte // snapshot is the serialized vault container database to restore from, if set.
	password []byte
	session
}

type Option func(*config)

// WithSnapshot sets a snapshot to restore the [Vault] from.
// obtained via [Vault.Serialize].
func WithSnapshot(snapshot []byte) Option {
	copied := make([]byte, len(snapshot))
	copy(copied, snapshot) // copied to avoid side effects from the underlying sqlite3 driver.

	return func(c *config) {
		c.snapshot = copied
	}
}

// WithPassword sets the password used to unlock the vault.
func WithPassword(p []byte) Option {
	return func(c *config) {
		c.password = p
	}
}

// WithSessionKey sets the AES-GCM key and nonce used
// for session-based unlocking.
func WithSessionKey(key, nonce []byte) Option {
	return func(c *config) {
		c.key = key
		c.nonce = nonce
	}
}

func newVault(path string, nonce []byte, aesgcm *vaultcrypto.AESGCM, vch *vaultContainerHandle) *Vault {
	return &Vault{
		Path:                 path,
		nonce:                nonce,
		aesgcm:               aesgcm,
		vaultContainerHandle: vch,
	}
}

// New creates a new vault container database at the given path if needed,
// derives the encryption key from the provided password,
// initializes and stores a new encrypted vault in serialized form, and loads it into memory.
//
// If a vault already exists at that path, it is overwritten.
// The previous vault data is saved in the vault history table,
// but is not used unless explicitly restored.
//
// On success, the function returns a [*Vault] ready for use.
func New(ctx context.Context, path string, password []byte, opts ...Option) (vlt *Vault, retErr error) {
	config := &config{}
	for _, opt := range opts {
		opt(config)
	}

	vaultContainerHandle, err := newVaultContainerHandle(ctx, path, config.snapshot)
	if err != nil {
		return nil, fmt.Errorf("vault.new: failed to initialize vault container handle: %w", err)
	}
	defer func() { //nolint:wsl
		if retErr != nil {
			_ = vaultContainerHandle.cleanup()
			_ = vlt.cleanup()

			return
		}
	}()

	cipherdata, err := vaultCipherData(password)
	if err != nil {
		return nil, fmt.Errorf("vault.new: failed to create vault cipher data: %w", err)
	}

	phc, err := vaultcrypto.DecodeAragon2idPHC(cipherdata.KDFPHC)
	if err != nil {
		return nil, fmt.Errorf("vault.new: failed to decode KDF PHC: %w", err)
	}

	aes, err := deriveAESGCM(phc, password)
	if err != nil {
		return nil, fmt.Errorf("vault.new: failed to derive AES-GCM key: %w", err)
	}

	vlt = newVault(path, cipherdata.Nonce, aes, vaultContainerHandle)

	if err := vlt.open(ctx, nil); err != nil {
		return vlt, fmt.Errorf("vault.new: failed to open vault: %w", err)
	}

	serialized, err := Serialize(vlt.conn)
	if err != nil {
		return vlt, fmt.Errorf("vault.new: failed to serialize vault connection: %w", err)
	}

	ciphervault, err := aes.Seal(cipherdata.Nonce, serialized)
	if err != nil {
		return vlt, fmt.Errorf("vault.new: failed to seal serialized vault: %w", err)
	}

	if err := vaultContainerHandle.db.InsertNewVault(ctx, cipherdata.AuthPHC, cipherdata.KDFPHC, cipherdata.Nonce, ciphervault); err != nil {
		return vlt, fmt.Errorf("vault.new: failed to insert new vault into vault container database: %w", err)
	}

	return vlt, nil
}

// Login verifies the password and derives the AES-GCM key
// for the vault at the given path.
func Login(ctx context.Context, path string, password []byte, opts ...Option) (key []byte, nonce []byte, _ error) {
	config := &config{}
	for _, opt := range opts {
		opt(config)
	}

	vaultContainerHandle, err := newVaultContainerHandle(ctx, path, config.snapshot)
	if err != nil {
		return nil, nil, errf("vault.login: failed to initialize vault container handle: %w", err)
	}
	defer func() { //nolint:wsl
		_ = vaultContainerHandle.cleanup()
	}()

	cipherdata, err := vaultContainerHandle.db.SelectVault(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("vault.login: failed to select vault from container database: %w", err)
	}

	if err := verifyPassword(password, cipherdata.AuthPHC); err != nil {
		return nil, nil, errf("vault.login: password verification failed: %w", err)
	}

	phc, err := vaultcrypto.DecodeAragon2idPHC(cipherdata.KDFPHC)
	if err != nil {
		return nil, nil, errf("vault.login: failed to decode KDF PHC: %w", err)
	}

	kdf := vaultcrypto.NewArgon2idKDF(vaultcrypto.WithPHC(phc))
	key = kdf.Derive(password)

	return key, cipherdata.Nonce, nil
}

// Open opens an existing vault container database at the given path,
// derives the encryption key from the provided password,
// and decrypts and deserializes the vault contents into memory.
//
// On success, the function returns a [*Vault] ready for use.
func Open(ctx context.Context, path string, opts ...Option) (vlt *Vault, retErr error) {
	config := &config{}
	for _, opt := range opts {
		opt(config)
	}

	vaultContainerHandle, err := newVaultContainerHandle(ctx, path, config.snapshot)
	if err != nil {
		return nil, errf("vault.open: failed to initialize vault container handle: %w", err)
	}
	defer func() { //nolint:wsl
		if retErr != nil {
			_ = vaultContainerHandle.cleanup()
			return
		}
	}()

	cipherdata, err := vaultContainerHandle.db.SelectVault(ctx)
	if err != nil {
		return nil, errf("vault.open: failed to select vault from container database: %w", err)
	}

	var (
		aes   *vaultcrypto.AESGCM
		nonce []byte
	)

	// choose key derivation method: password-based or session-based
	switch {
	case len(config.password) > 0:
		a, err := deriveAESFromPassword(cipherdata, config.password)
		if err != nil {
			return nil, errf("vault.open: failed to derive AES key from password: %w", err)
		}

		aes, nonce = a, cipherdata.Nonce
	case config.key != nil && config.nonce != nil:
		a, err := vaultcrypto.NewAESGCM(config.key)
		if err != nil {
			return nil, errf("vault.open: failed to initialize AES-GCM cipher: %w", err)
		}

		aes, nonce = a, config.nonce
	default:
		return nil, errf("vault.open: no password or session key provided")
	}

	vlt = newVault(path, nonce, aes, vaultContainerHandle)
	defer func() { //nolint:wsl
		if retErr != nil {
			_ = vlt.cleanup()
			return
		}
	}()

	if err := vlt.open(ctx, cipherdata.Vault); err != nil {
		return vlt, errf("vault.open: failed to open vault: %w", err)
	}

	return vlt, nil
}

func deriveAESFromPassword(cipherdata *vaultcontainer.CipherData, password []byte) (*vaultcrypto.AESGCM, error) {
	if err := verifyPassword(password, cipherdata.AuthPHC); err != nil {
		return nil, errf("derive AES from password: password verification failed: %w", err)
	}

	phc, err := vaultcrypto.DecodeAragon2idPHC(cipherdata.KDFPHC)
	if err != nil {
		return nil, errf("derive AES from password: failed to decode KDF PHC: %w", err)
	}

	aes, err := deriveAESGCM(phc, password)
	if err != nil {
		return nil, errf("derive AES from password: failed to derive AES-GCM key: %w", err)
	}

	return aes, nil
}

// Close serializes the in-memory SQLite database, encrypts it, and stores the
// resulting ciphertext in the vault container database.
//
// It is safe to call Close multiple times; only the first call has an effect.
//
// After calling Close, the in-memory database buffer [Vault.buf] is eligible for gc
// and should not be used again unless reinitialized.
func (vlt *Vault) Close(ctx context.Context) (retErr error) {
	if vlt == nil {
		return nil
	}

	vlt.closeOnce.Do(func() {
		retErr = vlt.close(ctx)
	})

	return retErr
}

//nolint:revive
func (vlt *Vault) close(ctx context.Context) error {
	if err := vlt.seal(ctx); err != nil {
		return err
	}

	vlt.buf = nil // release backing buffer to allow garbage collection.

	return vlt.cleanup()
}

// seal serializes the in-memory SQLite database, encrypts it, and stores the
// resulting ciphertext using the vault container.
func (vlt *Vault) seal(ctx context.Context) error {
	serialized, err := Serialize(vlt.conn)
	if err != nil {
		return errf("seal: failed to serialize vault connection: %w", err)
	}

	ciphervault, err := vlt.aesgcm.Seal(vlt.nonce, serialized)
	if err != nil {
		return errf("seal: failed to seal data with AES-GCM: %w", err)
	}

	if err := vlt.vaultContainerHandle.db.UpdateVault(ctx, ciphervault); err != nil {
		return errf("seal: failed to update vault in the vault container database: %w", err)
	}

	return nil
}

// Serialize returns the serialized form of the vault container, including the encrypted vault.
//
// It first seals the in-memory Vault to ensure the latest state is captured,
// then serializes the entire database.
//
// This is primarily used to produce a reusable snapshot of the vault container state
// for testing.
func (vlt *Vault) Serialize(ctx context.Context) ([]byte, error) {
	if err := vlt.seal(ctx); err != nil {
		return nil, errf("serialize: sealing vault failed: %w", err)
	}

	return Serialize(vlt.vaultContainerHandle.conn)
}

func (vlt *Vault) cleanup() error {
	if vlt == nil {
		return nil
	}

	if err := executeCleanup(vlt.cleanupFuncs); err != nil {
		return errf("cleanup: cleanup failed: %w", err)
	}

	return nil
}

// verifyPassword checks whether the given password matches the Argon2id PHC hash.
func verifyPassword(password []byte, phc string) error {
	authPHC, err := vaultcrypto.DecodeAragon2idPHC(phc)
	if err != nil {
		return errf("verify password: failed to decode auth PHC: %w", err)
	}

	kdf := vaultcrypto.NewArgon2idKDF(vaultcrypto.WithPHC(authPHC))
	derived := kdf.Derive(password)

	if subtle.ConstantTimeCompare(authPHC.Hash, derived) != 1 {
		return ErrAuthenticationFailed
	}

	return nil
}

// executeCleanup executes cleanup functions in reverse order,
// similar to defer statements.
//
// used functions are nilled out.
func executeCleanup(fs []cleanupFunc) error {
	var errs []error
	for i := len(fs) - 1; i >= 0; i-- {
		f := fs[i]
		if f == nil {
			continue
		}

		fs[i] = nil

		errs = append(errs, f())
	}

	return errors.Join(errs...)
}

// vaultContainerHandle manages the database connection and access
// to the vault container database schema used for storing the encrypted vault.
type vaultContainerHandle struct {
	conn         *sql.Conn
	db           *vaultcontainer.VaultContainer
	cleanupFuncs []cleanupFunc
}

func (h *vaultContainerHandle) cleanup() error {
	if h == nil {
		return nil
	}

	return executeCleanup(h.cleanupFuncs)
}

func newVaultContainerHandle(ctx context.Context, path string, snapshot []byte) (_ *vaultContainerHandle, retErr error) {
	handle := &vaultContainerHandle{}
	defer func() { //nolint:wsl
		if retErr != nil {
			retErr = errors.Join(retErr, handle.cleanup())
			return
		}
	}()

	var (
		db   *sql.DB
		conn *sql.Conn
	)

	handle.cleanupFuncs = append(handle.cleanupFuncs, func() error {
		// prefer conn.Close if available to avoid double-closing
		// the shared driver connection.
		if conn != nil {
			return conn.Close()
		}

		if db != nil {
			return db.Close()
		}

		return nil
	})

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, errf("new vault container handle: failed to open database: %w", err)
	}

	conn, err = db.Conn(ctx)
	if err != nil {
		return nil, errf("new vault container handle: failed to get database connection: %w", err)
	}

	if snapshot != nil {
		if err := Deserialize(conn, snapshot); err != nil {
			return nil, errf("new vault container handle: failed to deserialize snapshot: %w", err)
		}
	}

	m := migrate.New(db, migrate.SQLiteDialect{})

	_, err = m.Apply(vaultContainerMigrations)
	if err != nil {
		return nil, errf("new vault container handle: failed to apply migrations: %w", err)
	}

	handle.conn = conn
	handle.db = vaultcontainer.New(db)

	return handle, nil
}

// vaultCipherData generates [vaultcontainer.CipherData] containing salts, nonce,
// and derived authentication hash used for password authentication and vault encryption.
func vaultCipherData(password []byte) (*vaultcontainer.CipherData, error) {
	authSalt, err := vaultcrypto.RandBytes(16)
	if err != nil {
		return nil, errf("vault cipher data: failed to generate auth salt: %w", err)
	}

	authKDF := vaultcrypto.NewArgon2idKDF(vaultcrypto.WithSalt(authSalt))
	authPHC := authKDF.PHC()
	authPHC.Hash = authKDF.Derive(password)

	vaultSalt, err := vaultcrypto.RandBytes(16)
	if err != nil {
		return nil, errf("vault cipher data: failed to generate vault salt: %w", err)
	}

	vaultKDF := vaultcrypto.NewArgon2idKDF(vaultcrypto.WithSalt(vaultSalt))

	vaultNonce, err := vaultcrypto.RandBytes(12)
	if err != nil {
		return nil, errf("vault cipher data: failed to generate vault nonce: %w", err)
	}

	return &vaultcontainer.CipherData{
		AuthPHC: authPHC.String(),
		KDFPHC:  vaultKDF.PHC().String(),
		Nonce:   vaultNonce,
	}, nil
}

// open decrypts and loads the encrypted vault into memory by deserializing
// the SQLite database into a preallocated buffer.
//
// The buffer is retained in vlt.buf for the lifetime of the in-memory database
// and must remain valid until Seal() is called, which releases it.
//
// This method should only be called once during initialization and must not
// be called concurrently.
func (vlt *Vault) open(ctx context.Context, ciphervault []byte) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = errf("open: %w", retErr)
			return
		}
	}()

	var (
		db   *sql.DB
		conn *sql.Conn
	)

	vlt.cleanupFuncs = append(vlt.cleanupFuncs, func() error {
		// prefer conn.Close if available to avoid double-closing
		// the shared driver connection.
		if conn != nil {
			return conn.Close()
		}

		if db != nil {
			return db.Close()
		}

		return nil
	})

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return err
	}

	conn, err = db.Conn(ctx)
	if err != nil {
		return err
	}

	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return err
	}

	if ciphervault != nil {
		decrypted, err := vlt.aesgcm.Open(vlt.nonce, ciphervault)
		if err != nil {
			return err
		}

		vlt.buf = decrypted

		if err := Deserialize(conn, vlt.buf); err != nil {
			return err
		}
	}

	m := migrate.New(conn, migrate.SQLiteDialect{})

	_, err = m.Apply(vaultMigrations)
	if err != nil {
		return err
	}

	vlt.conn = conn
	vlt.db = vaultdb.New(conn)

	return nil
}

// deriveAESGCM derives an AES-GCM cipher using the given PHC and password.
// The [vaultcrypto.Argon2idPHC] provides the key derivation parameters,
// and the password is used to derive the encryption key.
func deriveAESGCM(phc vaultcrypto.Argon2idPHC, password []byte) (*vaultcrypto.AESGCM, error) {
	kdf := vaultcrypto.NewArgon2idKDF(vaultcrypto.WithPHC(phc))

	key := kdf.Derive(password)

	aes, err := vaultcrypto.NewAESGCM(key)
	if err != nil {
		return nil, errf("derive AES-GCM: %w", err)
	}

	return aes, nil
}

func errf(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

type insertConfig struct {
	id *int
}

type InsertOpt func(*insertConfig)

func InsertWithID(id int) InsertOpt {
	return func(c *insertConfig) {
		c.id = &id
	}
}

func newInsertConfig(opts ...InsertOpt) *insertConfig {
	c := &insertConfig{}
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// InsertNewSecret inserts a new secret with its labels
// into the vault using a transaction.
//
// Returns the ID of the inserted secret or an error if the operation fails.
func (vlt *Vault) InsertNewSecret(ctx context.Context, name string, secret []byte, labels []string, opts ...InsertOpt) (id int, retErr error) {
	insertConfig := newInsertConfig(opts...)

	tx, err := vlt.conn.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}

	storeTx := vlt.db.WithTx(tx)

	nonce, err := vaultcrypto.RandBytes(12)
	if err != nil {
		if err2 := tx.Rollback(); err2 != nil {
			return 0, errf("insert new secret: rollback: %w", errors.Join(err2, err))
		}

		return 0, errf("insert new secret: %w", err)
	}

	ciphertext, err := vlt.aesgcm.Seal(nonce, secret)
	if err != nil {
		if err2 := tx.Rollback(); err2 != nil {
			return 0, errf("insert new secret: rollback: %w", errors.Join(err2, err))
		}

		return 0, errf("insert new secret: %w", err)
	}

	var secretID int

	if insertConfig.id != nil {
		secretID, err = storeTx.InsertNewSecretWithID(ctx, *insertConfig.id, name, nonce, ciphertext)
	} else {
		secretID, err = storeTx.InsertNewSecret(ctx, name, nonce, ciphertext)
	}

	if err != nil {
		if err2 := tx.Rollback(); err2 != nil {
			return 0, errf("insert new secret: rollback: %w", errors.Join(err2, err))
		}

		return 0, errf("insert new secret: %w", err)
	}

	for _, l := range labels {
		if _, err := storeTx.InsertLabel(ctx, l, secretID); err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				return 0, errf("insert new secret: insert label: rollback: %w", errors.Join(err2, err))
			}

			return 0, errf("insert new secret: insert label: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, errf("insert new secret: tx commit: %w", err)
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
				return errf("update secret: name: rollback: %w", errors.Join(err2, err))
			}

			return errf("update secret: name: %w", err)
		}
	}

	for _, l := range addLabels {
		if _, err := updateTx.InsertLabel(ctx, l, id); err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				return errf("update secret: insert label: rollback: %w", errors.Join(err2, err))
			}

			return errf("update secret: insert label: %w", err)
		}
	}

	for _, l := range removeLabels {
		if _, err := updateTx.DeleteLabel(ctx, l, int64(id)); err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				return errf("update secret: remote label: rollback: %w", errors.Join(err2, err))
			}

			return errf("update secret: remove label: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return errf("update secret: tx commit: %w", err)
	}

	return nil
}

// UpdateSecret updates the secret value of the secret identified by id.
func (vlt *Vault) UpdateSecret(ctx context.Context, id int, secret []byte) (int64, error) {
	nonce, err := vaultcrypto.RandBytes(12)
	if err != nil {
		return 0, errf("update secret: %w", err)
	}

	ciphertext, err := vlt.aesgcm.Seal(nonce, secret)
	if err != nil {
		return 0, errf("update secret: %w", err)
	}

	return vlt.db.UpdateSecret(ctx, id, nonce, ciphertext)
}

// ExportSecrets exports all secret-related data stored in the database.
func (vlt *Vault) ExportSecrets(ctx context.Context) (map[int]vaultdb.SecretWithLabels, error) {
	encryptedSecrets, err := vlt.db.ExportSecrets(ctx)
	if err != nil {
		return nil, err
	}

	for id, s := range encryptedSecrets {
		decrypted, err := vlt.aesgcm.Open(s.Nonce, s.Ciphertext)
		if err != nil {
			return nil, err
		}

		s.Value = decrypted

		encryptedSecrets[id] = s
	}

	return encryptedSecrets, nil
}

// FilterSecrets returns secrets that match the given filters.
func (vlt *Vault) FilterSecrets(ctx context.Context, wildcard string, name string, labels []string) (map[int]vaultdb.SecretWithLabels, error) {
	filters := vaultdb.Filters{
		Wildcard: wildcard,
		Name:     name,
		Labels:   labels,
	}

	return vlt.db.FilterSecrets(ctx, filters)
}

// SecretsByIDs returns a map of secrets that match any of the provided IDs,
// along with all labels associated with each.
//
// If the IDs slice is empty, the function returns [vaultdb.ErrNoIDsProvided].
func (vlt *Vault) SecretsByIDs(ctx context.Context, ids ...int) (map[int]vaultdb.SecretWithLabels, error) {
	return vlt.db.SecretsByIDs(ctx, ids)
}

// ShowSecret returns the decrypted ciphertext associated with the given secret ID.
func (vlt *Vault) ShowSecret(ctx context.Context, id int) ([]byte, error) {
	nonce, ciphertext, err := vlt.db.ShowSecret(ctx, id)
	if err != nil {
		return nil, errf("show secret: %w", err)
	}

	secret, err := vlt.aesgcm.Open(nonce, ciphertext)
	if err != nil {
		return nil, errf("show secret: %w", err)
	}

	return secret, nil
}

// DeleteSecretsByIDs deletes secrets by their IDs, along with their labels.
func (vlt *Vault) DeleteSecretsByIDs(ctx context.Context, ids ...int) (int64, error) {
	return vlt.db.DeleteSecretsByIDs(ctx, ids)
}
