package vault_test

import (
	"path"
	"testing"

	"github.com/ladzaretti/vlt-cli/vault"
)

// https://github.com/spf13/cobra/issues/1419
// https://github.com/cli/cli/blob/c0c28622bd62b273b32838dfdfa7d5ffc739eeeb/command/pr_test.go#L55-L67
func TestVault_New(t *testing.T) {
	dir := t.TempDir()
	vaultPath := path.Join(dir, ".vlt.temp")

	v, err := vault.New(t.Context(), vaultPath, []byte("password"))
	if err != nil {
		t.Fatalf("failed to create vault: %v", err)
	}

	_, err = v.InsertNewSecret(t.Context(), "name", []byte("secret"), []string{"label1", "label2"})
	if err != nil {
		t.Errorf("failed to insert new secret: %v", err)
	}

	m, err := v.ExportSecrets(t.Context())
	if err != nil {
		t.Fatalf("failed to export secrets: %v", err)
	}

	_, err = v.Seal(t.Context())
	if err != nil {
		t.Errorf("failed to seal vault: %v", err)
	}

	err = v.Close()
	if err != nil {
		t.Errorf("failed to close vault: %v", err)
	}

	v, err = vault.Open(t.Context(), vaultPath, vault.WithPassword([]byte("password")))
	if err != nil {
		t.Fatalf("failed to reopen vault: %v", err)
	}

	m2, err := v.ExportSecrets(t.Context())
	if err != nil {
		t.Fatalf("failed to export secrets after reopen: %v", err)
	}

	if got, want := len(m2), len(m); got != want {
		t.Errorf("got %d secrets after reopen, want %d", got, want)
	}
}
