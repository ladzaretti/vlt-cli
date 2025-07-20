package cli_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"testing"
	"time"
	"unicode"

	"github.com/ladzaretti/vlt-cli/cli"
	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/input"
	"github.com/ladzaretti/vlt-cli/randstring"
	"github.com/ladzaretti/vlt-cli/vault"
	"github.com/ladzaretti/vlt-cli/vault/sqlite/vaultdb"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type vaultEnv struct {
	tempDir              string
	configPath           string
	vaultPath            string
	clipboardContentPath string
}

const (
	mockedPastedPassword = "mocked_pasted_password_input" //nolint:gosec
	mockedPromptPassword = "mocked_prompt_password_input"
)

func setupTestVaultEnv(t *testing.T) vaultEnv {
	t.Helper()
	tempDir := t.TempDir()

	ff, err := os.CreateTemp(tempDir, ".clipboard.*")
	if err != nil {
		t.Fatalf("failed to create clipboard content file: %v", err)
	}
	defer func() { //nolint:wsl
		_ = ff.Close()
	}()

	f, err := os.CreateTemp(tempDir, ".vlt.*.toml")
	if err != nil {
		t.Fatalf("failed to create temp config file: %v", err)
	}
	defer func() { //nolint:wsl
		_ = f.Close()
	}()

	clipboardContentPath := ff.Name()
	configPath := f.Name()
	vaultPath := path.Join(tempDir, ".vlt")

	content := fmt.Sprintf(`
		[vault]
		path = '%s'
		session_duration = '%s'
		[clipboard]
		copy_cmd=["tee", "%s"]
		paste_cmd=["printf", %q]
	`, vaultPath, "0m", clipboardContentPath, mockedPastedPassword)

	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write config content: %v", err)
	}

	if err := f.Sync(); err != nil {
		t.Fatalf("failed to flush config file: %v", err)
	}

	return vaultEnv{
		tempDir:              tempDir,
		configPath:           configPath,
		vaultPath:            vaultPath,
		clipboardContentPath: clipboardContentPath,
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

func mustInitializeVault(t *testing.T, configPath string, vaultPassword string) { //nolint:unparam // vaultPassword always receives mockedPassword ("mocked_password_input")
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

var randGenerated = []byte("rand_generated")

// secretWithLabelsComparer compares two [vaultdb.SecretWithLabels]
// for equality, ignoring cryptographic properties.
//
// If either [vaultdb.SecretWithLabels.Value] is randGenerated,
// he other is checked against [randstring.DefaultPasswordPolicy].
var secretWithLabelsComparer = gocmp.Comparer(func(a, b vaultdb.SecretWithLabels) bool {
	if a.Name != b.Name || !slices.Equal(a.Labels, b.Labels) {
		return false
	}

	switch {
	case bytes.Equal(a.Value, randGenerated):
		return validateRandPasswordBytes(b.Value)
	case bytes.Equal(b.Value, randGenerated):
		return validateRandPasswordBytes(a.Value)
	default:
		return bytes.Equal(a.Value, b.Value)
	}
})

func validateRandPasswordBytes(b []byte) bool {
	return validateRandPassword([]rune(string(b)))
}

func validateRandPassword(runes []rune) bool {
	var upper, lower, digit, special int

	for _, r := range runes {
		switch {
		case unicode.IsUpper(r):
			upper++
		case unicode.IsLower(r):
			lower++
		case unicode.IsDigit(r):
			digit++
		case unicode.IsPunct(r), unicode.IsSymbol(r):
			special++
		}
	}

	return len(runes) >= randstring.DefaultPasswordPolicy.MinLength &&
		special >= randstring.DefaultPasswordPolicy.MinSpecial &&
		upper >= randstring.DefaultPasswordPolicy.MinUppercase &&
		lower >= randstring.DefaultPasswordPolicy.MinLowercase &&
		digit >= randstring.DefaultPasswordPolicy.MinNumeric
}

func export(t *testing.T, vaultPath string, vaultPassword []byte) map[int]vaultdb.SecretWithLabels {
	t.Helper()

	v, err := vault.Open(t.Context(), vaultPath, vault.WithPassword(vaultPassword))
	if err != nil {
		t.Fatalf("failed to open vault: %v", err)
	}
	t.Cleanup(func() { //nolint:wsl
		_ = v.Close()
	})

	export, err := v.ExportSecrets(t.Context())
	if err != nil {
		t.Fatalf("unexpected error while exporting secrets: %v", err)
	}

	return export
}

func seedSecrets(t *testing.T, vaultEnv vaultEnv, input string) {
	t.Helper()

	if len(input) == 0 {
		return
	}

	f, err := os.CreateTemp(vaultEnv.tempDir, "import.csv")
	if err != nil {
		t.Fatalf("failed to create import file: %v", err)
	}
	t.Cleanup(func() { //nolint:wsl
		_ = f.Close()
	})

	if _, err := f.WriteString(input); err != nil {
		t.Fatalf("failed to write import file content: %v", err)
	}

	ioStreams, _, errOut := setupIOStreams(t, nil, newTTYFileInfo)

	cmd := cli.NewDefaultVltCommand(ioStreams,
		[]string{"import", "--config", vaultEnv.configPath, f.Name()})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error from import command: %v", err)
	}

	if got := errOut.String(); got != "" {
		t.Fatalf("unexpected stderr output: %q", got)
	}
}

type commandTestCase struct {
	name                 string
	seed                 string
	stdinData            []byte
	stdinInfoFn          func(string, int) os.FileInfo
	args                 []string
	wantErrorAs          any
	wantSecrets          []vaultdb.SecretWithLabels
	wantOutput           string
	wantStderr           string
	wantClipboardContent string
}

func (tt *commandTestCase) run(t *testing.T) {
	vaultEnv := setupTestVaultEnv(t)
	mustInitializeVault(t, vaultEnv.configPath, mockedPromptPassword)
	seedSecrets(t, vaultEnv, tt.seed)

	ioStreams, out, errOut := setupIOStreams(t, tt.stdinData, tt.stdinInfoFn)

	args := []string{"--config", vaultEnv.configPath}
	args = append(args, tt.args...)

	cmd := cli.NewDefaultVltCommand(ioStreams, args)

	if gotError := cmd.Execute(); tt.wantErrorAs != nil {
		if gotError == nil {
			t.Errorf("want error of type %T, got nil", tt.wantErrorAs)
		} else if !errors.As(gotError, &tt.wantErrorAs) && !errors.As(gotError, tt.wantErrorAs) {
			t.Errorf("want error of type %T (%v), got %T (%v)", tt.wantErrorAs, tt.wantErrorAs, gotError, gotError)
		}
	} else if gotError != nil {
		t.Errorf("unexpected error: %v", gotError)
	}

	if gotStderr := errOut.String(); gotStderr != tt.wantStderr {
		t.Errorf("want stderr output: %q, got %q", tt.wantStderr, gotStderr)
	}

	got, want := out.String(), fmt.Sprintf(`[vlt] Password for "%s":`, vaultEnv.vaultPath)+tt.wantOutput
	if diff := gocmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected stdout output (-want +got):\n%s", diff)
	}

	gotClipboardContent, err := os.ReadFile(vaultEnv.clipboardContentPath)
	if err != nil {
		t.Errorf("unexpected error while reading clipboard content containing file: %v", err)
	}

	if diff := gocmp.Diff([]byte(tt.wantClipboardContent), gotClipboardContent, secretWithLabelsComparer); diff != "" {
		t.Errorf("clipboard content mismatch (-want +got):\n%s", diff)
	}

	exported := export(t, vaultEnv.vaultPath, []byte(mockedPromptPassword))

	gotSecrets := make([]vaultdb.SecretWithLabels, 0, len(exported))

	for _, s := range exported {
		gotSecrets = append(gotSecrets, s)
	}

	opts := []gocmp.Option{
		secretWithLabelsComparer,
		cmpopts.SortSlices(func(a, b vaultdb.SecretWithLabels) bool {
			return a.Name < b.Name
		}),
	}
	if diff := gocmp.Diff(tt.wantSecrets, gotSecrets, opts...); diff != "" {
		t.Errorf("secrets mismatch (-want +got):\n%s", diff)
	}
}
