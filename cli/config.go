package cli

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"

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
type ResolvedConfig struct {
	copyCmd         string
	pasteCmd        string
	sessionDuration time.Duration
	vaultPath       string
}

var _ genericclioptions.CmdOptions = &ConfigOptions{}

// NewConfigOptions initializes ConfigOptions with default values.
func NewConfigOptions(stdio *genericclioptions.StdioOptions) *ConfigOptions {
	return &ConfigOptions{
		StdioOptions: stdio,
		fileConfig:   &FileConfig{},
		cliFlags:     &Flags{},
		resolved:     &ResolvedConfig{},
	}
}

func (o *ConfigOptions) Complete() error {
	c, err := LoadFileConfig(o.cliFlags.configPath)
	if err != nil {
		return err
	}

	o.fileConfig = c

	return o.resolve()
}

func (o *ConfigOptions) resolve() error {
	o.resolved.copyCmd = o.fileConfig.Clipboard.CopyCmd
	o.resolved.pasteCmd = o.fileConfig.Clipboard.PasteCmd
	o.resolved.vaultPath = cmp.Or(o.cliFlags.vaultPath, o.fileConfig.Vault.Path)

	if len(o.resolved.vaultPath) == 0 {
		vaultPath, err := defaultVaultPath()
		if err != nil {
			return err
		}

		o.resolved.vaultPath = vaultPath
	}

	sessionDuration := cmp.Or(o.fileConfig.Vault.SessionDuration, defaultSessionDuration)

	t, err := time.ParseDuration(sessionDuration)
	if err != nil {
		return fmt.Errorf("invalid session duration: %w", err)
	}

	o.resolved.sessionDuration = t

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
func NewCmdConfig(vltOpts *DefaultVltOptions) *cobra.Command {
	hiddenFlags := []string{"config"}
	o := NewConfigOptions(vltOpts.StdioOptions)

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Resolve and inspect the active vlt configuration (subcommands available)",
		Long: fmt.Sprintf(`Resolve and display the active vlt configuration.

If --file is not provided, the default config path (~/%s) is used.`, defaultConfigName),
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.RejectDisallowedFlags(cmd, hiddenFlags...))
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))

			if len(o.fileConfig.path) == 0 {
				o.Infof("No config file found; using default values.\n")
				return
			}

			o.Infof("Resolved config at %q:\n\n%s\n", o.fileConfig.path, o.fileConfig)
		},
	}

	cmd.PersistentFlags().StringVarP(&o.cliFlags.configPath, "file", "f", "",
		fmt.Sprintf("path to the configuration file (default: ~/%s)", defaultConfigName))

	cmd.AddCommand(newGenerateConfigCmd(vltOpts))
	cmd.AddCommand(newValidateConfigCmd(vltOpts))

	genericclioptions.MarkFlagsHidden(cmd, hiddenFlags...)

	return cmd
}

type generateConfigOptions struct {
	*genericclioptions.StdioOptions
}

var _ genericclioptions.CmdOptions = &generateConfigOptions{}

// newGenerateConfigOptions initializes the options struct.
func newGenerateConfigOptions(vltOpts *DefaultVltOptions) *generateConfigOptions {
	return &generateConfigOptions{
		StdioOptions: vltOpts.StdioOptions,
	}
}

func (*generateConfigOptions) Complete() error { return nil }

func (*generateConfigOptions) Validate() error { return nil }

func (o *generateConfigOptions) Run(context.Context, ...string) error {
	out, err := toml.Marshal(&FileConfig{})
	clierror.Check(err)

	o.Infof("%s", string(out))

	return nil
}

// newGenerateConfigCmd creates the 'generate' subcommand for generating default config.
func newGenerateConfigCmd(vltOpts *DefaultVltOptions) *cobra.Command {
	hiddenFlags := []string{"file", "config"}
	o := newGenerateConfigOptions(vltOpts)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Print a default config file",
		Long: `Outputs the default configuration in TOML format to stdout.

This command does not accept any arguments.`,
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
		o.Infof("No config file found; Nothing to validate.\n")
		return nil
	}

	o.Infof("%s: OK\n", c.path)

	return nil
}

// newValidateConfigCmd creates the 'validate' subcommand for validating the config file.
func newValidateConfigCmd(vltOpts *DefaultVltOptions) *cobra.Command {
	hiddenFlags := []string{"config"}
	o := newValidateConfigOptions(vltOpts.StdioOptions)

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
