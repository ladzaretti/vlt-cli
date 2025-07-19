package cli_test

import (
	"bytes"
	"cmp"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/ladzaretti/vlt-cli/cli"
	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vault"
	"github.com/ladzaretti/vlt-cli/vault/sqlite/vaultdb"
	"github.com/ladzaretti/vlt-cli/vaulterrors"

	gocmp "github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

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

func TestConfigGenerateCommand(t *testing.T) {
	stdin := genericclioptions.NewTestFdReader(bytes.NewBuffer(nil), 0, newTTYFileInfo("stdin", 0))
	ioStreams, _, out, errOut := genericclioptions.NewTestIOStreams(stdin)

	cmd := cli.NewDefaultVltCommand(ioStreams, []string{
		"config", "generate",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("config command failed: %v\nstderr: %s", err, errOut.String())
	}

	gotStdout, wantStdout := out.String(), `[vault]
# Vlt database path (default: '~/.vlt' if not set)
# path = ''
# How long a session lasts before requiring login again (default: '1m')
# session_duration = ''
# Maximum number of historical vault snapshots to keep (default: 3, 0 disables history)
# max_history_snapshots = 3

# Clipboard configuration: Both copy and paste commands must be either both set or both unset.
[clipboard]
# The command used for copying to the clipboard (default: ['xsel', '-ib'] if not set)
# copy_cmd = []
# The command used for pasting from the clipboard (default: ['xsel', '-ob'] if not set)
# paste_cmd = []

# Optional lifecycle hooks for vault events
[hooks]
# Command to run after a successful login
# post_login_cmd = []
# Command to run after any vault write (e.g., create, update, delete)
# post_write_cmd = []
`

	if errOut.Len() > 0 {
		t.Errorf("unexpected stderr output: %s", errOut.String())
	}

	if diff := gocmp.Diff(wantStdout, gotStdout); diff != "" {
		t.Errorf("secrets mismatch (-want +got):\n%s", diff)
	}
}

func TestConfigValidateCommand(t *testing.T) {
	vaultEnv := setupTestVaultEnv(t)
	validConfig := `
[vault]
path = "/tmp/vault.db"
session_duration = "10m"
max_history_snapshots = 2
`

	f, err := os.CreateTemp(vaultEnv.tempDir, "import.csv")
	if err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}
	t.Cleanup(func() { //nolint:wsl
		_ = f.Close()
	})

	defaultConfigPath := f.Name()

	if _, err := f.WriteString(validConfig); err != nil {
		t.Fatalf("failed to write config file content: %v", err)
	}

	t.Setenv("VLT_CONFIG_PATH", defaultConfigPath)

	stdin := genericclioptions.NewTestFdReader(bytes.NewBuffer(nil), 0, newTTYFileInfo("stdin", 0))
	ioStreams, _, out, errOut := genericclioptions.NewTestIOStreams(stdin)

	cmd := cli.NewDefaultVltCommand(ioStreams, []string{
		"config", "validate",
	})

	if err := cmd.Execute(); err != nil {
		t.Errorf("validate command failed: %v\nstderr: %s", err, errOut.String())
	}

	if errOut.String() != "" {
		t.Errorf("unexpected stderr: %s", errOut.String())
	}

	gotStdout, wantStdout := out.String(), fmt.Sprintf(`INFO %s:`, defaultConfigPath)+" OK\n"
	if diff := gocmp.Diff(wantStdout, gotStdout); diff != "" {
		t.Errorf("unexpected stdout output (-want +got):\n%s", diff)
	}

	out.Reset()
	errOut.Reset()

	cmd = cli.NewDefaultVltCommand(ioStreams, []string{
		"config", "validate", "--file", defaultConfigPath,
	})

	t.Setenv("VLT_CONFIG_PATH", "")

	if err := cmd.Execute(); err != nil {
		t.Errorf("validate command failed: %v\nstderr: %s", err, errOut.String())
	}

	if errOut.Len() > 0 {
		t.Errorf("unexpected stderr: %s", errOut.String())
	}

	gotStdout, wantStdout = out.String(), fmt.Sprintf(`INFO %s:`, defaultConfigPath)+" OK\n"
	if diff := gocmp.Diff(wantStdout, gotStdout); diff != "" {
		t.Errorf("unexpected stdout output (-want +got):\n%s", diff)
	}
}

func TestCreateCommand_WithPrompt(t *testing.T) {
	vaultEnv := setupTestVaultEnv(t)

	mustInitializeVault(t, vaultEnv.configPath, mockedPromptPassword)

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
	testCases := []commandTestCase{
		{
			name:        "full prompt",
			stdinData:   []byte(secret1.Name + "\n" + secret1.Labels[0] + "\n"),
			stdinInfoFn: newTTYFileInfo,
			args:        []string{"save", "-c"},
			wantOutput:  `Enter name: Enter secret for name "name_1": Enter labels (comma-separated), or press Enter to skip: `,
			wantSecrets: []vaultdb.SecretWithLabels{
				{
					Name:   secret1.Name,
					Value:  []byte(mockedPromptPassword),
					Labels: secret1.Labels,
				},
			},
			wantClipboardContent: mockedPromptPassword,
		},
		{
			name:        "prompt password only, metadata via cli flags",
			stdinData:   secret2.Value,
			stdinInfoFn: newNonTTYFileInfo,
			args: []string{
				"save",
				"--name", secret2.Name,
				"--label", secret2.Labels[0],
				"-c",
			},
			wantSecrets: []vaultdb.SecretWithLabels{
				secret2,
			},
			wantClipboardContent: string(secret2.Value),
		},
		{
			name:        "paste password only, metadata via cli flags",
			stdinData:   nil,
			stdinInfoFn: newTTYFileInfo,
			args: []string{
				"save",
				"--name", secret3.Name,
				"--label", secret3.Labels[0],
				"-p",
				"-c",
			},
			wantSecrets: []vaultdb.SecretWithLabels{
				{
					Name:   secret3.Name,
					Value:  []byte(mockedPastedPassword),
					Labels: secret3.Labels,
				},
			},
			wantClipboardContent: mockedPastedPassword,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, tt.run)
	}
}

const (
	vltExportHeader      = "name,secret,labels"
	firefoxImportHeader  = "url,username,password,httpRealm,formActionOrigin,guid,timeCreated,timeLastUsed,timePasswordChanged"
	chromiumImportHeader = "name,url,username,password,note"
	customImportHeader   = "password,username,label_1,label_2"
)

func vltImportRecord(data vaultdb.SecretWithLabels) string {
	return fmt.Sprintf(
		"%s,%s,%q",
		data.Name,                      // name
		hex.EncodeToString(data.Value), // password
		data.Labels[0],                 // labels
	)
}

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

func customImportRecord(data vaultdb.SecretWithLabels) string {
	return fmt.Sprintf(
		"%s,%s,,%s",
		data.Value,     // password
		data.Name,      // username
		data.Labels[0], // label_1
	)
}

func TestImportCommand(t *testing.T) { //nolint:revive
	testCases := []struct {
		name        string
		importData  string
		wantSecrets map[int]vaultdb.SecretWithLabels
		extraArgs   []string
	}{
		{
			name: "vault import",
			importData: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
				vltImportRecord(secret3),
				vltImportRecord(secret4),
			}, "\n"),
			wantSecrets: map[int]vaultdb.SecretWithLabels{
				1: secret1,
				2: secret2,
				3: secret3,
				4: secret4,
			},
		},
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
		{
			name: "custom import",
			importData: strings.Join([]string{
				customImportHeader,
				customImportRecord(secret1),
				customImportRecord(secret2),
				customImportRecord(secret3),
				customImportRecord(secret4),
			}, "\n"),
			wantSecrets: map[int]vaultdb.SecretWithLabels{
				1: secret1,
				2: secret2,
				3: secret3,
				4: secret4,
			},
			extraArgs: []string{
				"--indexes", `{"name":1,"secret":0,"labels":[2,3]}`,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			vaultEnv := setupTestVaultEnv(t)
			mustInitializeVault(t, vaultEnv.configPath, mockedPromptPassword)

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

			args := []string{"import", "--config", vaultEnv.configPath, f.Name()}
			args = append(args, tt.extraArgs...)

			cmd := cli.NewDefaultVltCommand(ioStreams, args)

			if err := cmd.Execute(); err != nil {
				t.Errorf("unexpected error from import command: %v", err)
			}

			if got := errOut.String(); got != "" {
				t.Errorf("unexpected stderr output: %q", got)
			}

			v, err := vault.Open(t.Context(), vaultEnv.vaultPath, vault.WithPassword([]byte(mockedPromptPassword)))
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

			if diff := gocmp.Diff(tt.wantSecrets, gotSecrets, secretWithLabelsComparer); diff != "" {
				t.Errorf("secrets mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestExportCommand(t *testing.T) {
	vaultEnv := setupTestVaultEnv(t)
	mustInitializeVault(t, vaultEnv.configPath, mockedPromptPassword)
	seedSecrets(t, vaultEnv, strings.Join([]string{
		vltExportHeader,
		vltImportRecord(secret1),
		vltImportRecord(secret2),
		vltImportRecord(secret3),
		vltImportRecord(secret4),
	}, "\n"))

	exportFile := path.Join(vaultEnv.tempDir, "export.csv")
	ioStreams, _, errOut := setupIOStreams(t, nil, newTTYFileInfo)
	cmd := cli.NewDefaultVltCommand(ioStreams, []string{
		"export",
		"--config", vaultEnv.configPath,
		"-o", exportFile,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("export command failed: %v\nstderr: %s", err, errOut.String())
	}

	// create a new vault to import into.

	anotherVaultEnv := setupTestVaultEnv(t)
	mustInitializeVault(t, anotherVaultEnv.configPath, mockedPromptPassword)

	cmd = cli.NewDefaultVltCommand(ioStreams, []string{
		"import",
		"--config", anotherVaultEnv.configPath,
		exportFile,
	},
	)

	if err := cmd.Execute(); err != nil {
		t.Errorf("unexpected error from import command: %v", err)
	}

	exported := export(t, vaultEnv.vaultPath, []byte(mockedPromptPassword))

	gotSecrets := make([]vaultdb.SecretWithLabels, 0, len(exported))

	for _, s := range exported {
		gotSecrets = append(gotSecrets, s)
	}

	wantSecrets := []vaultdb.SecretWithLabels{secret1, secret2, secret3, secret4}

	opts := []gocmp.Option{
		secretWithLabelsComparer,
		cmpopts.SortSlices(func(a, b vaultdb.SecretWithLabels) bool {
			return a.Name < b.Name
		}),
	}
	if diff := gocmp.Diff(wantSecrets, gotSecrets, opts...); diff != "" {
		t.Errorf("secrets mismatch (-want +got):\n%s", diff)
	}
}

func TestFindCommand(t *testing.T) { //nolint:revive
	testCases := []commandTestCase{
		{
			name:        "list all secrets",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
				vltImportRecord(secret3),
				vltImportRecord(secret4),
			}, "\n"),
			args: []string{"find"},
			wantOutput: `ID     NAME       LABELS
4      name_4     label_4
3      name_3     label_3
2      name_2     label_2
1      name_1     label_1

`,
			wantSecrets: []vaultdb.SecretWithLabels{secret1, secret2, secret3, secret4},
		},
		{
			name:        "find by glob match in name or label",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
				vltImportRecord(secret3),
				vltImportRecord(secret4),
			}, "\n"),
			args: []string{"find", "*3"},
			wantOutput: `ID     NAME       LABELS
3      name_3     label_3

`,
			wantSecrets: []vaultdb.SecretWithLabels{secret1, secret2, secret3, secret4},
		},
		{
			name:        "find by name",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
			}, "\n"),
			args: []string{"find", "--name", "name_2"},
			wantOutput: `ID     NAME       LABELS
2      name_2     label_2

`,
			wantSecrets: []vaultdb.SecretWithLabels{secret1, secret2},
		},
		{
			name:        "find by id",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
				vltImportRecord(secret3),
			}, "\n"),
			args: []string{"find", "--id", "1", "--id", "3"},
			wantOutput: `ID     NAME       LABELS
3      name_3     label_3
1      name_1     label_1

`,
			wantSecrets: []vaultdb.SecretWithLabels{secret1, secret2, secret3},
		},
		{
			name:        "find by multiple labels",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
				vltImportRecord(secret3),
			}, "\n"),
			args: []string{"find", "--label", "label_1", "--label", "label_3"},
			wantOutput: `ID     NAME       LABELS
3      name_3     label_3
1      name_1     label_1

`,
			wantSecrets: []vaultdb.SecretWithLabels{secret1, secret2, secret3},
		},
		{
			name:        "no results found",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
			}, "\n"),
			args: []string{"find", "--name", "nonexistent"},
			wantOutput: `ID     NAME     LABELS

`,
			wantSecrets: []vaultdb.SecretWithLabels{secret1, secret2},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, tt.run)
	}
}

func TestShowCommand(t *testing.T) { //nolint:revive
	testCases := []commandTestCase{
		{
			name:        "by name and output to stdout",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
			}, "\n"),
			args:        []string{"show", "--name", secret1.Name, "--stdout"},
			wantOutput:  string(secret1.Value),
			wantSecrets: []vaultdb.SecretWithLabels{secret1},
		},

		{
			name:        "by name and copy to clipboard",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
			}, "\n"),
			args:                 []string{"show", "--name", secret1.Name, "-c"},
			wantOutput:           "",
			wantClipboardContent: string(secret1.Value),
			wantSecrets:          []vaultdb.SecretWithLabels{secret1},
		},
		{
			name:        "by wildcard and output to stdout",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
			}, "\n"),
			args:        []string{"show", "*", "--stdout"},
			wantSecrets: []vaultdb.SecretWithLabels{secret1},
			wantOutput:  string(secret1.Value),
		},
		{
			name:        "by wildcard and copy to clipboard",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
			}, "\n"),
			args:                 []string{"show", "*", "-c"},
			wantOutput:           "",
			wantSecrets:          []vaultdb.SecretWithLabels{secret1},
			wantClipboardContent: string(secret1.Value),
		},
		{
			name:        "by id and output to stdout",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
			}, "\n"),
			args:        []string{"show", "--id", "1", "--stdout"},
			wantSecrets: []vaultdb.SecretWithLabels{secret1},
			wantOutput:  string(secret1.Value),
		},
		{
			name:        "by id and copy to clipboard",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
			}, "\n"),
			args:                 []string{"show", "--id", "1", "-c"},
			wantOutput:           "",
			wantSecrets:          []vaultdb.SecretWithLabels{secret1},
			wantClipboardContent: string(secret1.Value),
		},
		{
			name:        "by label and output to stdout",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
			}, "\n"),
			args:        []string{"show", "--label", secret1.Labels[0], "--stdout"},
			wantSecrets: []vaultdb.SecretWithLabels{secret1},
			wantOutput:  string(secret1.Value),
		},
		{
			name:        "by label and copy to clipboard",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
			}, "\n"),
			args:                 []string{"show", "--label", secret1.Labels[0], "-c"},
			wantOutput:           "",
			wantSecrets:          []vaultdb.SecretWithLabels{secret1},
			wantClipboardContent: string(secret1.Value),
		},
		{
			name:        "by name and label and output to stdout",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
				vltImportRecord(secret3),
			}, "\n"),
			args:        []string{"show", "--name", secret1.Name, "--label", secret1.Labels[0], "--stdout"},
			wantOutput:  string(secret1.Value),
			wantSecrets: []vaultdb.SecretWithLabels{secret1, secret2, secret3},
		},
		{
			name:        "by name and label and copy to clipboard",
			stdinInfoFn: newTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
				vltImportRecord(secret3),
			}, "\n"),
			args:                 []string{"show", "--name", secret1.Name, "--label", secret1.Labels[0], "-c"},
			wantOutput:           "",
			wantSecrets:          []vaultdb.SecretWithLabels{secret1, secret2, secret3},
			wantClipboardContent: string(secret1.Value),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, tt.run)
	}

	t.Run("ambiguous match fails with error and stderr", func(t *testing.T) {
		vaultEnv := setupTestVaultEnv(t)
		mustInitializeVault(t, vaultEnv.configPath, mockedPromptPassword)

		input := strings.Join([]string{
			vltExportHeader,
			vltImportRecord(secret1),
			vltImportRecord(secret2),
			vltImportRecord(secret3),
		}, "\n")
		seedSecrets(t, vaultEnv, input)

		ioStreams, out, errOut := setupIOStreams(t, nil, newTTYFileInfo)

		args := []string{
			"show",
			"--config", vaultEnv.configPath,
			"*",
			"--stdout",
		}

		cmd := cli.NewDefaultVltCommand(ioStreams, args)

		gotErr, wantErr := cmd.Execute(), vaulterrors.ErrAmbiguousSecretMatch
		if gotErr == nil {
			t.Fatal("expected error due to ambiguous match, got nil")
		}

		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("want error %q, got %q", wantErr, gotErr)
		}

		if got := out.String(); got != fmt.Sprintf(`[vlt] Password for "%s":`, vaultEnv.vaultPath) {
			t.Errorf("unexpected stdout: %q", got)
		}

		if got := errOut.String(); got == "" {
			t.Error("want stderr message due to ambiguous match, got empty string")
		} else if !strings.Contains(got, "multiple secrets match") {
			t.Errorf("unexpected stderr content: %q", got)
		}
	})
}

func TestGenerateCommand(t *testing.T) { //nolint:revive,gocognit,cyclop
	type passwordRequirements struct {
		minLen  int
		upper   int
		lower   int
		digit   int
		special int
	}

	tests := []struct {
		name string
		args []string
		want passwordRequirements
	}{
		{
			name: "default policy",
			args: []string{"generate", "-c"},
			want: passwordRequirements{
				minLen:  12,
				upper:   2,
				lower:   2,
				digit:   2,
				special: 2,
			},
		},
		{
			name: "numeric 4 and length 16",
			args: []string{"generate", "--numeric", "4", "--min-length", "16"},
			want: passwordRequirements{
				minLen: 16,
				digit:  4,
			},
		},
		{
			name: "no special characters",
			args: []string{"generate", "--special", "0"},
			want: passwordRequirements{
				minLen:  12,
				special: 0,
			},
		},
		{
			name: "custom mix",
			args: []string{"generate", "-u3", "-l3", "-d3", "-s3"},
			want: passwordRequirements{
				minLen:  12,
				upper:   3,
				lower:   3,
				digit:   3,
				special: 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vaultEnv := setupTestVaultEnv(t)

			stdin := genericclioptions.NewTestFdReader(nil, 0, newTTYFileInfo("stdin", 0))
			ioStreams, _, out, errOut := genericclioptions.NewTestIOStreams(stdin)

			clierror.SetErrorHandler(clierror.PrintErrHandler)
			clierror.SetErrWriter(ioStreams.ErrOut)

			args := []string{"--config", vaultEnv.configPath}
			tt.args = append(tt.args, args...)

			cmd := cli.NewDefaultVltCommand(ioStreams, tt.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("generate command failed: %v\nstderr: %s", err, errOut.String())
			}

			if got := errOut.String(); got != "" {
				t.Errorf("unexpected stderr output: %q", got)
			}

			stdout := strings.TrimSpace(out.String())
			clipboard := ""

			if len(stdout) == 0 {
				maxAttempts := 5
				clipboard = pollFile(t, vaultEnv.clipboardContentPath, maxAttempts)
			}

			output := cmp.Or(stdout, clipboard)

			got := passwordRequirements{
				minLen: len(output),
			}
			for _, r := range output {
				switch {
				case unicode.IsUpper(r):
					got.upper++
				case unicode.IsLower(r):
					got.lower++
				case unicode.IsDigit(r):
					got.digit++
				case unicode.IsPunct(r) || unicode.IsSymbol(r):
					got.special++
				}
			}

			if got.minLen < tt.want.minLen {
				t.Errorf("want: password length ≥ %d, got: %d", tt.want.minLen, len(stdout))
			}

			if got.upper < tt.want.upper {
				t.Errorf("want: >= %d uppercase letters, got: %d", tt.want.upper, got.upper)
			}

			if got.lower < tt.want.lower {
				t.Errorf("want: >= %d lowercase letters, got: %d", tt.want.lower, got.lower)
			}

			if got.digit < tt.want.digit {
				t.Errorf("want: >= %d digits, got: %d", tt.want.digit, got.digit)
			}

			if got.special < tt.want.special {
				t.Errorf("want: >= %d special characters, got: %d", tt.want.special, got.special)
			}
		})
	}
}

func pollFile(t *testing.T, path string, maxAttempts int) (content string) {
	t.Helper()

	ticker := time.NewTicker(time.Millisecond * 200)
	defer ticker.Stop()

	for i := range maxAttempts {
		c, err := os.ReadFile(filepath.Clean(path))
		if err != nil {
			t.Logf("clipboard read attempt %d failed: %v", i+1, err)
			continue
		}

		content = string(c)

		if len(content) > 0 {
			break
		}

		<-ticker.C
	}

	return content
}

func TestRemoveCommand(t *testing.T) { //nolint:revive
	testCases := []commandTestCase{
		{
			name:        "force remove by id",
			stdinInfoFn: newNonTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
				vltImportRecord(secret3),
			}, "\n"),
			args:        []string{"remove", "--id", "1", "--yes"},
			wantSecrets: []vaultdb.SecretWithLabels{secret2, secret3},
			wantOutput:  "INFO successfully deleted 1 secrets.\n",
		},
		{
			name: "remove by name with confirmation",
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
				vltImportRecord(secret3),
			}, "\n"),
			stdinData:   []byte("y\n"),
			stdinInfoFn: newTTYFileInfo,
			args:        []string{"remove", "--name", secret1.Name},
			wantSecrets: []vaultdb.SecretWithLabels{secret2, secret3},
			wantOutput: `ID     NAME       LABELS
1      name_1     label_1

Delete 1 secrets? (y/N): INFO successfully deleted 1 secrets.
`,
		},
		{
			name: "abort remove by prompt",
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
				vltImportRecord(secret3),
			}, "\n"),
			stdinData:   []byte("\n"),
			stdinInfoFn: newTTYFileInfo,
			args:        []string{"remove", "--name", secret1.Name},
			wantSecrets: []vaultdb.SecretWithLabels{secret1, secret2, secret3},
			wantOutput: `ID     NAME       LABELS
1      name_1     label_1

Delete 1 secrets? (y/N): `,
		},
		{
			name:        "force remove by label",
			stdinInfoFn: newNonTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
				vltImportRecord(secret3),
			}, "\n"),
			args:        []string{"remove", "--label", "label_1", "--yes"},
			wantSecrets: []vaultdb.SecretWithLabels{secret2, secret3},
			wantOutput:  "INFO successfully deleted 1 secrets.\n",
		},
		{
			name:        "require confirmation when multiple match",
			stdinInfoFn: newNonTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
				vltImportRecord(secret3),
			}, "\n"),
			args:        []string{"remove", "--label", "label_[12]", "--yes"},
			wantErrorAs: &cli.RemoveError{},
			wantSecrets: []vaultdb.SecretWithLabels{secret1, secret2, secret3},
			wantOutput:  "",
			wantStderr:  "WARN found 2 matching secrets.\nvlt: remove: 2 matching secrets found, use --all to delete all\n",
		},

		{
			name:        "force remove by label glob",
			stdinInfoFn: newNonTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
				vltImportRecord(secret3),
			}, "\n"),
			args:        []string{"remove", "--label", "label_[12]", "--yes", "--all"},
			wantSecrets: []vaultdb.SecretWithLabels{secret3},
			wantOutput:  "INFO successfully deleted 2 secrets.\n",
			wantStderr:  "WARN found 2 matching secrets.\n",
		},
		{
			name:        "no matching secrets",
			stdinInfoFn: newNonTTYFileInfo,
			seed: strings.Join([]string{
				vltExportHeader,
				vltImportRecord(secret1),
				vltImportRecord(secret2),
			}, "\n"),
			args:        []string{"remove", "--name", "does-not-exist", "--yes"},
			wantErrorAs: &cli.RemoveError{},
			wantSecrets: []vaultdb.SecretWithLabels{secret1, secret2},
			wantOutput:  "",
			wantStderr:  "WARN no match found.\nvlt: remove: no match found\n",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, tt.run)
	}
}
