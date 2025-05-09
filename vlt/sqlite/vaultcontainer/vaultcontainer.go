package vaultcontainer

import (
	"context"
	"database/sql"

	"github.com/ladzaretti/vlt-cli/vlt/types"
)

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

func (s *VaultContainer) InsertNewVault(ctx context.Context, auth string, kdf string, nonce []byte, ciphervault []byte) error {
	if _, err := s.db.ExecContext(ctx, insertVault, auth, kdf, nonce, ciphervault); err != nil {
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

func (v *VaultContainer) UpdateVault(ctx context.Context, VaultEncrypted []byte) error {
	_, err := v.db.ExecContext(ctx, updateVault, VaultEncrypted)
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

func (v *VaultContainer) SelectVault(ctx context.Context) (*CipherData, error) {
	row := v.db.QueryRowContext(ctx, selectVault)

	var data CipherData
	if err := row.Scan(&data.AuthPHC, &data.KDFPHC, &data.Nonce, &data.Vault); err != nil {
		return nil, err
	}

	return &data, nil
}
