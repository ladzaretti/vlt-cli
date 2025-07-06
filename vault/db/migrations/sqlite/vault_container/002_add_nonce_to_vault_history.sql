ALTER TABLE vault_history
ADD COLUMN nonce BLOB;

DROP TRIGGER IF EXISTS after_vault_update;

CREATE TRIGGER IF NOT EXISTS after_vault_update AFTER
UPDATE OF vault_encrypted ON vault_container WHEN OLD.checksum <> NEW.checksum BEGIN
INSERT INTO
    vault_history (snapshot, checksum, nonce)
VALUES
    (OLD.vault_encrypted, OLD.checksum, OLD.nonce);

END;