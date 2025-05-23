CREATE TABLE
    IF NOT EXISTS secrets (
        id INTEGER PRIMARY KEY,
        name TEXT NOT NULL,

        ciphertext BLOB NOT NULL,

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