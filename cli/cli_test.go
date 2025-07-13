package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ladzaretti/vlt-cli/cli"
	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/input"
	"github.com/ladzaretti/vlt-cli/vault"
	"github.com/ladzaretti/vlt-cli/vault/sqlite/vaultdb"

	"github.com/google/go-cmp/cmp"
)

type vaultEnv struct {
	tempDir    string
	configPath string
	vaultPath  string
}

func setupTestVaultEnv(t *testing.T) vaultEnv {
	t.Helper()
	tempDir := t.TempDir()

	f, err := os.CreateTemp(tempDir, ".vlt.*.toml")
	if err != nil {
		t.Fatalf("failed to create temp config file: %v", err)
	}
	t.Cleanup(func() { //nolint:wsl
		_ = f.Close()
	})

	configPath, vaultPath := f.Name(), path.Join(tempDir, ".vlt")

	content := fmt.Sprintf(`
		[vault]
		path = '%s'
		session_duration = '%s'
	`, vaultPath, "0m")

	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write config content: %v", err)
	}

	return vaultEnv{
		tempDir:    tempDir,
		configPath: configPath,
		vaultPath:  vaultPath,
	}
}

// setupIOStreams creates IOStreams with a mocked stdin.
func setupIOStreams(t *testing.T, in []byte, stdinInfoFn func(string, int) os.FileInfo) (ioStreams *genericclioptions.IOStreams, out *bytes.Buffer, errOut *bytes.Buffer) {
	t.Helper()

	var (
		buf       = bytes.NewBuffer(in)
		stdinInfo = stdinInfoFn("stdin", len(in))
	)

	stdinReader := genericclioptions.NewTestFdReader(buf, 0, stdinInfo)

	ioStreams, _, out, errOut = genericclioptions.NewTestIOStreams(stdinReader)

	clierror.SetErrorHandler(clierror.PrintErrHandler)
	clierror.SetErrWriter(ioStreams.ErrOut)

	t.Cleanup(func() {
		clierror.ResetErrorHandler()
		clierror.ResetErrWriter()
	})

	return
}

func newTTYFileInfo(name string, size int) os.FileInfo {
	return genericclioptions.NewMockFileInfo(name, int64(size), os.ModeCharDevice, false, time.Now())
}

func newNonTTYFileInfo(name string, size int) os.FileInfo {
	return genericclioptions.NewMockFileInfo(name, int64(size), 0, false, time.Now())
}

const mockedPassword = "mocked_password_input"

func mustInitializeVault(t *testing.T, configPath string, vaultPassword string) {
	t.Helper()

	ioStreams, _, errOut := setupIOStreams(t, nil, newTTYFileInfo)

	input.SetDefaultReadPassword(func(_ int) ([]byte, error) {
		return []byte(vaultPassword), nil
	})

	cmd := cli.NewDefaultVltCommand(ioStreams, []string{
		"create", "--config", configPath,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("create command failed: %v\nstderr: %q", err, errOut.String())
	}
}

// secretWithLabelsComparer compares two [vaultdb.SecretWithLabels]
// for equality on non cryptographic fields.
var secretWithLabelsComparer = cmp.Comparer(func(a, b vaultdb.SecretWithLabels) bool {
	return a.Name == b.Name &&
		bytes.Equal(a.Value, b.Value) &&
		slices.Equal(a.Labels, b.Labels)
})

func TestConfigCommand(t *testing.T) {
	testEnv := setupTestVaultEnv(t)

	stdin := genericclioptions.NewTestFdReader(bytes.NewBuffer(nil), 0, newTTYFileInfo("stdin", 0))
	ioStreams, _, out, errOut := genericclioptions.NewTestIOStreams(stdin)

	cmd := cli.NewDefaultVltCommand(ioStreams, []string{
		"config", "--file", testEnv.configPath,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config command failed: %v\nstderr: %s", err, errOut.String())
	}

	var config struct {
		Path     string             `json:"path"`
		Parsed   cli.FileConfig     `json:"parsed_config"`   //nolint:tagliatelle
		Resolved cli.ResolvedConfig `json:"resolved_config"` //nolint:tagliatelle
	}

	err := json.Unmarshal(out.Bytes(), &config)
	if err != nil {
		t.Fatalf("failed to unmarshal output: %v\noutput: %s", err, out.String())
	}

	if got, want := config.Parsed.Vault.SessionDuration, "0m"; got != want {
		t.Errorf("got parsed session duration %q, want %q", got, want)
	}

	if got := config.Parsed.Vault.Path; got == "" {
		t.Error("got empty parsed vault path, want non-empty path")
	}

	if got, want := config.Resolved.SessionDuration, cli.Duration(0); got != want {
		t.Errorf("got resolved session duration %v, want %v", got, want)
	}

	if got := config.Resolved.VaultPath; got == "" {
		t.Error("got empty resolved vault path, want non-empty path")
	}
}

func TestCreateCommand_WithPrompt(t *testing.T) {
	vaultEnv := setupTestVaultEnv(t)

	mustInitializeVault(t, vaultEnv.configPath, mockedPassword)

	ioStreams, _, errOut := setupIOStreams(t, nil, newTTYFileInfo)
	cmd := cli.NewDefaultVltCommand(ioStreams, []string{
		"create", "--config", vaultEnv.configPath,
	})

	if gotError, wantError := cmd.Execute(), "vault file already exists"; gotError == nil || gotError.Error() != wantError {
		t.Errorf("got error %v, want %q", gotError, wantError)
	}

	if gotError, wantError := errOut.String(), "vlt: vault file already exists"; !strings.Contains(gotError, wantError) {
		t.Errorf("got stderr %q, want it to contain %q", errOut.String(), wantError)
	}
}

var (
	secret1 = vaultdb.SecretWithLabels{
		Name:   "name_1",
		Labels: []string{"label_1"},
		Value:  []byte("secret_1"),
	}

	secret2 = vaultdb.SecretWithLabels{
		Name:   "name_2",
		Labels: []string{"label_2"},
		Value:  []byte("secret_2"),
	}

	secret3 = vaultdb.SecretWithLabels{
		Name:   "name_3",
		Labels: []string{"label_3"},
		Value:  []byte("secret_3"),
	}

	secret4 = vaultdb.SecretWithLabels{
		Name:   "name_4",
		Labels: []string{"label_4"},
		Value:  []byte("secret_4"),
	}
)

func TestSaveCommand(t *testing.T) {
	vaultEnv := setupTestVaultEnv(t)

	mustInitializeVault(t, vaultEnv.configPath, mockedPassword)

	ioStreams, _, errOut := setupIOStreams(t, []byte(secret1.Name+"\n"+secret1.Labels[0]+"\n"), newTTYFileInfo)
	cmd := cli.NewDefaultVltCommand(ioStreams, []string{
		"save", "--config", vaultEnv.configPath,
	})

	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error from save command: %v", err)
	}

	if got := errOut.String(); got != "" {
		t.Errorf("unexpected stderr output: %q", got)
	}

	ioStreams, _, errOut = setupIOStreams(t, secret2.Value, newNonTTYFileInfo)
	cmd = cli.NewDefaultVltCommand(ioStreams, []string{
		"save",
		"--config", vaultEnv.configPath,
		"--name", secret2.Name,
		"--label", secret2.Labels[0],
	})

	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error from save command: %v", err)
	}

	if got := errOut.String(); got != "" {
		t.Errorf("unexpected stderr output: %q", got)
	}

	v, err := vault.Open(t.Context(), vaultEnv.vaultPath, vault.WithPassword([]byte(mockedPassword)))
	if err != nil {
		t.Fatalf("failed to open vault: %v", err)
	}
	t.Cleanup(func() { _ = v.Close() }) //nolint:wsl

	gotSecrets, err := v.ExportSecrets(t.Context())
	if err != nil {
		t.Errorf("unexpected error while exporting secrets: %v", err)
	}

	wantSecrets := map[int]vaultdb.SecretWithLabels{
		1: {
			Name:   secret1.Name,
			Value:  []byte(mockedPassword),
			Labels: secret1.Labels,
		},
		2: secret2,
	}

	if diff := cmp.Diff(wantSecrets, gotSecrets, secretWithLabelsComparer); diff != "" {
		t.Errorf("secrets mismatch (-want +got):\n%s", diff)
	}
}

const (
	firefoxImportHeader  = "url,username,password,httpRealm,formActionOrigin,guid,timeCreated,timeLastUsed,timePasswordChanged"
	chromiumImportHeader = "name,url,username,password,note"
)

func firefoxImportRecord(data vaultdb.SecretWithLabels) string {
	return fmt.Sprintf(
		"%s,%s,%s,,,,,,",
		data.Labels[0], // url
		data.Name,      // username
		data.Value,     // password
	)
}

func chromiumImportRecord(data vaultdb.SecretWithLabels) string {
	return fmt.Sprintf(
		"%s,,%s,%s,",
		data.Labels[0], // name
		data.Name,      // username
		data.Value,     // password
	)
}

func TestImportCommand(t *testing.T) { //nolint:revive
	testCases := []struct {
		name        string
		importData  string
		wantSecrets map[int]vaultdb.SecretWithLabels
	}{
		{
			name: "firefox import",
			importData: strings.Join([]string{
				firefoxImportHeader,
				firefoxImportRecord(secret1),
				firefoxImportRecord(secret2),
				firefoxImportRecord(secret3),
				firefoxImportRecord(secret4),
			}, "\n"),
			wantSecrets: map[int]vaultdb.SecretWithLabels{
				1: secret1,
				2: secret2,
				3: secret3,
				4: secret4,
			},
		},
		{
			name: "chromium import",
			importData: strings.Join([]string{
				chromiumImportHeader,
				chromiumImportRecord(secret1),
				chromiumImportRecord(secret2),
				chromiumImportRecord(secret3),
				chromiumImportRecord(secret4),
			}, "\n"),
			wantSecrets: map[int]vaultdb.SecretWithLabels{
				1: secret1,
				2: secret2,
				3: secret3,
				4: secret4,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			vaultEnv := setupTestVaultEnv(t)

			mustInitializeVault(t, vaultEnv.configPath, mockedPassword)

			f, err := os.CreateTemp(vaultEnv.tempDir, "import.csv")
			if err != nil {
				t.Fatalf("failed to create import file: %v", err)
			}
			t.Cleanup(func() { //nolint:wsl
				_ = f.Close()
			})

			if _, err := f.WriteString(tt.importData); err != nil {
				t.Fatalf("failed to write import file content: %v", err)
			}

			ioStreams, _, errOut := setupIOStreams(t, nil, newTTYFileInfo)
			cmd := cli.NewDefaultVltCommand(ioStreams, []string{
				"import", "--config", vaultEnv.configPath, f.Name(),
			})

			if err := cmd.Execute(); err != nil {
				t.Errorf("unexpected error from import command: %v", err)
			}

			if got := errOut.String(); got != "" {
				t.Errorf("unexpected stderr output: %q", got)
			}

			v, err := vault.Open(t.Context(), vaultEnv.vaultPath, vault.WithPassword([]byte(mockedPassword)))
			if err != nil {
				t.Fatalf("failed to open vault: %v", err)
			}
			t.Cleanup(func() { //nolint:wsl
				_ = v.Close()
			})

			gotSecrets, err := v.ExportSecrets(t.Context())
			if err != nil {
				t.Errorf("unexpected error while exporting secrets: %v", err)
			}

			if diff := cmp.Diff(tt.wantSecrets, gotSecrets, secretWithLabelsComparer); diff != "" {
				t.Errorf("secrets mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
