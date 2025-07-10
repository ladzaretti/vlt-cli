package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ladzaretti/vlt-cli/cli"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
)

func writeTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	f, err := os.CreateTemp(dir, ".vlt.toml")
	if err != nil {
		t.Fatalf("failed to create temp config file: %v", err)
	}
	defer func() { //nolint:wsl
		_ = f.Close()
	}()

	path := f.Name()

	content := fmt.Sprintf(`
		[vault]
		path = '%s'
		session_duration = '%s'
	`, path, "0m")

	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("failed to write config content: %v", err)
	}

	return path
}

func newTTYFileInfo(name string, size int64) os.FileInfo {
	return genericclioptions.NewMockFileInfo(name, size, os.ModeCharDevice, false, time.Now())
}

func TestConfigCommand_WithValidConfig(t *testing.T) {
	t.Helper()

	configPath := writeTestConfig(t)

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

	if config.Parsed.Vault.SessionDuration != "0m" {
		t.Errorf("Parsed.SessionDuration want '0m', got %v", config.Parsed.Vault.SessionDuration)
	}

	if config.Parsed.Vault.Path == "" {
		t.Error("expected parsed vault path to be set, got empty string")
	}

	if config.Resolved.SessionDuration != 0 {
		t.Errorf("SessionDuration want 0, got %v", config.Resolved.SessionDuration)
	}

	if config.Resolved.VaultPath == "" {
		t.Error("expected resolved vault path to be set, got empty string")
	}
}
