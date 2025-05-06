CREATE TABLE
    IF NOT EXISTS master_key (
        id INTEGER PRIMARY KEY CHECK (id = 0),
        -- PHC-formatted string used for authenticating the master password.
        -- Includes Argon2id parameters, salt, and the hash of the derived key.
        auth_phc TEXT NOT NULL,
        -- PHC-formatted string used for deriving the master encryption key.
        -- Includes Argon2id parameters and salt, but no hash.
        kdf_phc TEXT NOT NULL
    );

CREATE TABLE
    IF NOT EXISTS secrets (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        
        secret BLOB NOT NULL,

        -- 96-bit (12-byte) nonce used for AES-GCM encryption.
        -- Required for decryption; generated randomly per secret.
        nonce BLOB NOT NULL,
        
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT NULL
    );

CREATE TABLE
    IF NOT EXISTS labels (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,
        secret_id TEXT NOT NULL REFERENCES secrets (id) ON DELETE CASCADE,
        UNIQUE (name, secret_id)
    );

CREATE TRIGGER IF NOT EXISTS update_secrets_updated_at AFTER
UPDATE ON secrets FOR EACH ROW BEGIN
UPDATE secrets
SET
    updated_at = CURRENT_TIMESTAMP
WHERE
    id = OLD.id;

END;