package vaultcontainer

import (
	"context"
	"database/sql"

	"github.com/ladzaretti/vlt-cli/vlt/types"
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
		vault_container (id, auth_phc, kdf_phc, nonce, vault_encrypted)
	VALUES
		(0, ?, ?, ?, ?) ON CONFLICT (id) DO
	UPDATE
	SET
		auth_phc = excluded.auth_phc,
		kdf_phc = excluded.kdf_phc,
		nonce = excluded.nonce,
		vault_encrypted = excluded.vault_encrypted;
`

func (vc *VaultContainer) InsertNewVault(ctx context.Context, auth string, kdf string, nonce []byte, ciphervault []byte) error {
	if _, err := vc.db.ExecContext(ctx, insertVault, auth, kdf, nonce, ciphervault); err != nil {
		return err
	}

	return nil
}

const updateVault = `
	UPDATE
		vault_container
	SET
		vault_encrypted = ?
	WHERE
		id = 0;
`

func (vc *VaultContainer) UpdateVault(ctx context.Context, ciphervault []byte) error {
	_, err := vc.db.ExecContext(ctx, updateVault, ciphervault)
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
