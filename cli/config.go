package cli

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/util"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// ConfigOptions holds cli, file, and resolved global configuration.
type ConfigOptions struct {
	*genericclioptions.StdioOptions

	fileConfig *FileConfig
	cliFlags   *Flags

	resolved *ResolvedConfig
}

// Flags holds cli overrides for configuration.
type Flags struct {
	configPath string
	vaultPath  string
}

// ResolvedConfig contains the final merged configuration.
// cli flags take precedence over config file values.
//
//nolint:tagliatelle
type ResolvedConfig struct {
	SessionDuration     Duration `json:"session_duration,omitempty"`
	VaultPath           string   `json:"vault_path,omitempty"`
	MaxHistorySnapshots int      `json:"max_history_snapshots"`
	CopyCmd             []string `json:"copy_cmd,omitempty"`
	PasteCmd            []string `json:"paste_cmd,omitempty"`
	PostLoginCmd        []string `json:"post_login_cmd,omitempty"`
	PostWriteCmd        []string `json:"post_write_cmd,omitempty"`

	enabledSession bool
}

type Duration time.Duration

func (d Duration) String() string { return time.Duration(d).String() }

func (d Duration) MarshalJSON() ([]byte, error) { return json.Marshal(d.String()) }

var _ genericclioptions.CmdOptions = &ConfigOptions{}

// NewConfigOptions initializes ConfigOptions with default values.
func NewConfigOptions(stdio *genericclioptions.StdioOptions) *ConfigOptions {
	return &ConfigOptions{
		StdioOptions: stdio,
		fileConfig:   newFileConfig(),
		cliFlags:     &Flags{},
		resolved:     &ResolvedConfig{},
	}
}

func (o *ConfigOptions) Resolved() *ResolvedConfig { return o.resolved }

func (o *ConfigOptions) Complete() error {
	c, err := LoadFileConfig(o.cliFlags.configPath)
	if err != nil {
		return err
	}

	o.fileConfig = c

	return o.resolve()
}

func (o *ConfigOptions) resolve() error {
	o.resolved.CopyCmd = o.fileConfig.Clipboard.CopyCmd
	o.resolved.PasteCmd = o.fileConfig.Clipboard.PasteCmd
	o.resolved.PostLoginCmd = o.fileConfig.Hooks.PostLoginCmd
	o.resolved.PostWriteCmd = o.fileConfig.Hooks.PostWriteCmd
	o.resolved.VaultPath = cmp.Or(o.cliFlags.vaultPath, o.fileConfig.Vault.Path)

	o.resolved.MaxHistorySnapshots = defaultMaxHistorySnapshots
	if o.fileConfig.Vault.MaxHistorySnapshots != nil {
		o.resolved.MaxHistorySnapshots = *o.fileConfig.Vault.MaxHistorySnapshots
	}

	if len(o.resolved.VaultPath) == 0 {
		vaultPath, err := defaultVaultPath()
		if err != nil {
			return err
		}

		o.resolved.VaultPath = vaultPath
	}

	sessionDuration := cmp.Or(o.fileConfig.Vault.SessionDuration, defaultSessionDuration)

	t, err := time.ParseDuration(sessionDuration)
	if err != nil {
		return fmt.Errorf("invalid session duration: %w", err)
	}

	o.resolved.SessionDuration = Duration(t)

	if o.resolved.SessionDuration > 0 {
		o.resolved.enabledSession = true
	}

	return nil
}

func defaultVaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, defaultDatabaseFilename), nil
}
func (*ConfigOptions) Validate() error { return nil }

func (*ConfigOptions) Run(context.Context, ...string) error { return nil }

// NewCmdConfig creates the cobra config command tree.
func NewCmdConfig(defaults *DefaultVltOptions) *cobra.Command {
	hiddenFlags := []string{"config", "no-hooks", "no-login-prompt"}
	o := NewConfigOptions(defaults.StdioOptions)

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Resolve and inspect the active vlt configuration (subcommands available)",
		Long: fmt.Sprintf(`Resolve and display the active vlt configuration.

If --file is not provided, the default config path (~/%s) is used.`, defaultConfigName),
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.RejectDisallowedFlags(cmd, hiddenFlags...))
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))

			if len(o.fileConfig.path) == 0 {
				o.Infof("no config file found; using default values.\n")
				return
			}

			c := struct {
				Path     string `json:"path"`
				Parsed   any    `json:"parsed_config"`   //nolint:tagliatelle
				Resolved any    `json:"resolved_config"` //nolint:tagliatelle
			}{
				Path:     o.fileConfig.path,
				Parsed:   o.fileConfig,
				Resolved: o.resolved,
			}

			o.Printf("%s", stringifyPretty(c))
		},
	}

	cmd.PersistentFlags().StringVarP(&o.cliFlags.configPath, "file", "f", "",
		fmt.Sprintf("path to the configuration file (default: ~/%s)", defaultConfigName))

	cmd.AddCommand(newGenerateConfigCmd(defaults))
	cmd.AddCommand(newValidateConfigCmd(defaults))

	genericclioptions.MarkFlagsHidden(cmd, hiddenFlags...)

	return cmd
}

// stringifyPretty returns the pretty-printed JSON representation of v.
// If marshalling fails, it returns the error message instead.
func stringifyPretty(v any) string {
	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)

	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")

	if err := enc.Encode(v); err != nil {
		return fmt.Sprintf("stringify error: %v", err)
	}

	return buf.String()
}

type generateConfigOptions struct {
	*genericclioptions.StdioOptions
}

var _ genericclioptions.CmdOptions = &generateConfigOptions{}

// newGenerateConfigOptions initializes the options struct.
func newGenerateConfigOptions(stdio *genericclioptions.StdioOptions) *generateConfigOptions {
	return &generateConfigOptions{
		StdioOptions: stdio,
	}
}

func (*generateConfigOptions) Complete() error { return nil }

func (*generateConfigOptions) Validate() error { return nil }

func (o *generateConfigOptions) Run(context.Context, ...string) error {
	c := newFileConfig()
	c.Vault.MaxHistorySnapshots = util.Ptr(defaultMaxHistorySnapshots)

	out, err := toml.Marshal(c)
	clierror.Check(err)

	o.Printf("%s", string(out))

	return nil
}

// newGenerateConfigCmd creates the 'generate' subcommand for generating default config.
func newGenerateConfigCmd(defaults *DefaultVltOptions) *cobra.Command {
	hiddenFlags := []string{"config", "file", "no-hooks", "no-login-prompt", "verbose"}
	o := newGenerateConfigOptions(defaults.StdioOptions)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Print a default config file",
		Long:  `Outputs the default configuration in TOML format to stdout.`,
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.RejectDisallowedFlags(cmd, hiddenFlags...))
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	genericclioptions.MarkFlagsHidden(cmd, hiddenFlags...)

	return cmd
}

type validateConfigOptions struct {
	*genericclioptions.StdioOptions

	configPath string
}

var _ genericclioptions.CmdOptions = &validateConfigOptions{}

// newValidateConfigOptions initializes the options struct.
func newValidateConfigOptions(stdio *genericclioptions.StdioOptions) *validateConfigOptions {
	return &validateConfigOptions{
		StdioOptions: stdio,
	}
}

func (*validateConfigOptions) Complete() error { return nil }

func (*validateConfigOptions) Validate() error { return nil }

func (o *validateConfigOptions) Run(context.Context, ...string) error {
	c, err := LoadFileConfig(o.configPath)
	clierror.Check(err)

	if len(c.path) == 0 {
		o.Infof("no config file found; Nothing to validate.\n")
		return nil
	}

	o.Infof("%s: OK\n", c.path)

	return nil
}

// newValidateConfigCmd creates the 'validate' subcommand for validating the config file.
func newValidateConfigCmd(defaults *DefaultVltOptions) *cobra.Command {
	hiddenFlags := []string{"config", "no-hooks", "no-login-prompt"}
	o := newValidateConfigOptions(defaults.StdioOptions)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Check config validity",
		Long: fmt.Sprintf(`Loads the configuration file and checks for common errors.

If --file is not provided, the default config path (~/%s) is used.`, defaultConfigName),
		Run: func(cmd *cobra.Command, _ []string) {
			o.configPath, _ = cmd.InheritedFlags().GetString("file")

			clierror.Check(genericclioptions.RejectDisallowedFlags(cmd, hiddenFlags...))
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	genericclioptions.MarkFlagsHidden(cmd, hiddenFlags...)

	return cmd
}
