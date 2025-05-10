package vaultcontainer

import (
	"context"
	"crypto/sha1" //nolint:gosec // in this context, SHA-1 is for change detection, not security.
	"database/sql"

	"github.com/ladzaretti/vlt-cli/vault/types"
)

// VaultContainer provides access to the vault container database schema.
//
// This database stores the cryptographic data required to perform operations
// such as encrypting or decrypting the vault and its secrets.
type VaultContainer struct {
	db types.DBTX
}

func New(db types.DBTX) *VaultContainer {
	return &VaultContainer{
		db: db,
	}
}

// WithTx returns a new [VaultContainer] using the given transaction.
func (*VaultContainer) WithTx(tx *sql.Tx) *VaultContainer {
	return &VaultContainer{
		db: tx,
	}
}

const insertVault = `
	INSERT INTO
		vault_container (
			id,
			auth_phc,
			kdf_phc,
			nonce,
			vault_encrypted,
			checksum,
			updated_at
		)
	VALUES
		(0, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP) ON CONFLICT (id) DO
	UPDATE
	SET
		auth_phc = excluded.auth_phc,
		kdf_phc = excluded.kdf_phc,
		nonce = excluded.nonce,
		vault_encrypted = excluded.vault_encrypted,
		checksum = excluded.checksum,
		updated_at = excluded.updated_at
	WHERE
		vault_container.checksum <> excluded.checksum;
`

func (vc *VaultContainer) InsertNewVault(ctx context.Context, auth string, kdf string, nonce []byte, ciphervault []byte) error {
	//nolint:gosec // in this context, SHA-1 is for change detection, not security.
	checksum := sha1.Sum(ciphervault)
	if _, err := vc.db.ExecContext(ctx, insertVault, auth, kdf, nonce, ciphervault, checksum[:]); err != nil {
		return err
	}

	return nil
}

const updateVault = `
	UPDATE vault_container
	SET
		vault_encrypted = $1,
		checksum = $2,
		updated_at = CURRENT_TIMESTAMP
	WHERE
		id = 0
		AND checksum <> $2;
`

func (vc *VaultContainer) UpdateVault(ctx context.Context, ciphervault []byte) error {
	//nolint:gosec // in this context, SHA-1 is for change detection, not security.
	checksum := sha1.Sum(ciphervault)
	_, err := vc.db.ExecContext(ctx, updateVault, ciphervault, checksum[:])

	return err
}

const selectVault = `
	SELECT
		auth_phc, kdf_phc, nonce, vault_encrypted
	FROM
		vault_container
	WHERE
		id = 0;
`

type CipherData struct {
	AuthPHC string
	KDFPHC  string
	Nonce   []byte
	Vault   []byte
}

func (vc *VaultContainer) SelectVault(ctx context.Context) (*CipherData, error) {
	row := vc.db.QueryRowContext(ctx, selectVault)

	var data CipherData
	if err := row.Scan(&data.AuthPHC, &data.KDFPHC, &data.Nonce, &data.Vault); err != nil {
		return nil, err
	}

	return &data, nil
}
