CREATE TABLE
    IF NOT EXISTS vault_container (
        id INTEGER PRIMARY KEY CHECK (id = 0),
        -- PHC-formatted string used for authenticating the master password.
        -- Includes Argon2id parameters, salt, and the hash of the derived key.
        auth_phc TEXT NOT NULL,
        -- PHC-formatted string used for deriving the master encryption key.
        -- Includes Argon2id parameters and salt, but no hash.
        kdf_phc TEXT NOT NULL,
        nonce BLOB NOT NULL,
        vault_encrypted BLOB NOT NULL
    );

CREATE TABLE
    IF NOT EXISTS vault_history (
        id INTEGER PRIMARY KEY,
        created_at TEXT NOT NULL DEFAULT (datetime ('now')),
        snapshot BLOB NOT NULL
    );

CREATE TRIGGER IF NOT EXISTS after_vault_update AFTER
UPDATE OF vault_encrypted ON vault_container BEGIN
INSERT INTO
    vault_history (snapshot)
VALUES
    (OLD.vault_encrypted);

END;