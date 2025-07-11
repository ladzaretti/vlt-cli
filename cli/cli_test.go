package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/ladzaretti/vlt-cli/cli"
	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/input"
)

func newTestConfig(t *testing.T) (configPath string) {
	t.Helper()
	dir := t.TempDir()

	f, err := os.CreateTemp(dir, ".vlt.*.toml")
	if err != nil {
		t.Fatalf("failed to create temp config file: %v", err)
	}
	defer func() { //nolint:wsl
		_ = f.Close()
	}()

	configPath, vaultPath := f.Name(), path.Join(dir, ".vlt")

	content := fmt.Sprintf(`
		[vault]
		path = '%s'
		session_duration = '%s'
	`, vaultPath, "0m")

	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write config content: %v", err)
	}

	return configPath
}

func setupIOStreams(t *testing.T, in []byte) (ioStreams *genericclioptions.IOStreams, out *bytes.Buffer, errOut *bytes.Buffer) {
	t.Helper()

	buf := bytes.NewBuffer(in)
	stdinReader := genericclioptions.NewTestFdReader(buf, 0, newTTYFileInfo("stdin", len(in)))

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

func mustInitializeVault(t *testing.T) string {
	t.Helper()

	configPath := newTestConfig(t)

	ioStreams, _, errOut := setupIOStreams(t, nil)

	input.SetDefaultReadPassword(func(_ int) ([]byte, error) {
		return []byte("mocked_password_input"), nil
	})

	cmd := cli.NewDefaultVltCommand(ioStreams, []string{
		"create", "--config", configPath,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("create command failed: %v\nstderr: %q", err, errOut.String())
	}

	return configPath
}

func TestConfigCommand_WithValidConfig(t *testing.T) {
	configPath := newTestConfig(t)

	stdin := genericclioptions.NewTestFdReader(bytes.NewBuffer(nil), 0, newTTYFileInfo("stdin", 0))
	ioStreams, _, out, errOut := genericclioptions.NewTestIOStreams(stdin)

	cmd := cli.NewDefaultVltCommand(ioStreams, []string{
		"config", "--file", configPath,
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
		t.Errorf("got Parsed.SessionDuration %q, want %q", got, want)
	}

	if got := config.Parsed.Vault.Path; got == "" {
		t.Error("got empty Parsed.Vault.Path, want non-empty path")
	}

	if got, want := config.Resolved.SessionDuration, cli.Duration(0); got != want {
		t.Errorf("got Resolved.SessionDuration %v, want %v", got, want)
	}

	if got := config.Resolved.VaultPath; got == "" {
		t.Error("got empty Resolved.VaultPath, want non-empty path")
	}
}

func TestCreateCommand_WithPrompt(t *testing.T) {
	configPath := mustInitializeVault(t)

	ioStreams, _, errOut := setupIOStreams(t, nil)
	cmd := cli.NewDefaultVltCommand(ioStreams, []string{
		"create", "--config", configPath,
	})

	if gotError, wantError := cmd.Execute(), "vault file already exists"; gotError == nil || gotError.Error() != wantError {
		t.Errorf("got error %v, want %q", gotError, wantError)
	}

	if gotError, wantError := errOut.String(), "vlt: vault file already exists"; !strings.Contains(gotError, wantError) {
		t.Errorf("got stderr %q, want it to contain %q", errOut.String(), wantError)
	}
}
