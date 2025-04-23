package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

const (
	// defaultConfigName is the default name of the configuration file
	// expected under the user's home directory.
	defaultConfigName = ".vlt.toml"

	// envConfigPathKey is the environment variable key for overriding
	// the config file path.
	envConfigPathKey = "VLT_CONFIG_PATH"
)

var ErrPartialClipboardConfig = errors.New("invalid partial clipboard config")

//nolint:tagalign
type VaultConfig struct {
	Path string `toml:"path,commented" comment:"Default path to look for the SQLite backing database"`
}

//nolint:tagalign
type ClipboardConfig struct {
	CopyCmd  string `toml:"copy_cmd,commented"  comment:"The command used for copying to the clipboard"`
	PasteCmd string `toml:"paste_cmd,commented" comment:"The command used for pasting from the clipboard"`
}

type Config struct {
	Vault     VaultConfig     `toml:"vault"`
	Clipboard ClipboardConfig `toml:"clipboard"`
}

func (c Config) String() string {
	return fmt.Sprintf(`Config{
  Vault: {
    Path: %q
  },
  Clipboard: {
    CopyCmd:  %q,
    PasteCmd: %q
  }
}`, c.Vault.Path, c.Clipboard.CopyCmd, c.Clipboard.PasteCmd)
}

// hasPartialClipboard checks if only one of the clipboard commands is set.
func (c Config) hasPartialClipboard() bool {
	return (c.Clipboard.CopyCmd == "") != (c.Clipboard.PasteCmd == "")
}

func (c Config) Validate() error {
	if c.hasPartialClipboard() {
		return ErrPartialClipboardConfig
	}

	return nil
}

func ResolveConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: user home dir: %w", err)
	}

	path := filepath.Join(home, defaultConfigName)
	if p, ok := os.LookupEnv(envConfigPathKey); ok {
		path = p
	}

	return path, nil
}

// LoadConfig loads the configuration from a file and validates it.
func LoadConfig(path string) (Config, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Config{}, fmt.Errorf("config: no config file found at %q", path)
		}

		return Config{}, fmt.Errorf("config: stat file: %w", err)
	}

	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Config{}, err
	}

	config := Config{}
	if err := toml.Unmarshal(raw, &config); err != nil {
		return Config{}, fmt.Errorf("config: parse file: %w", err)
	}

	return config, config.Validate()
}

type ConfigOptions struct {
	*genericclioptions.StdioOptions

	configPath string

	config Config
}

var _ genericclioptions.CmdOptions = &ConfigOptions{}

// NewConfigOptions initializes the options struct.
func NewConfigOptions(stdio *genericclioptions.StdioOptions) *ConfigOptions {
	return &ConfigOptions{
		StdioOptions: stdio,
	}
}

func (o *ConfigOptions) Complete() error {
	if len(o.configPath) == 0 {
		f, err := ResolveConfigPath()
		if err != nil {
			return err
		}

		o.configPath = f
	}

	return nil
}

func (o *ConfigOptions) Validate() error {
	if len(o.configPath) == 0 {
		return errors.New("config options: missing path")
	}

	return nil
}

func (o *ConfigOptions) Run(context.Context) error {
	c, err := LoadConfig(o.configPath)
	if err != nil {
		return err
	}

	o.config = c

	return nil
}

// NewCmdConfig creates the cobra config command tree.
func NewCmdConfig(stdio *genericclioptions.StdioOptions) *cobra.Command {
	hiddenFlags := []string{"file"}
	o := NewConfigOptions(stdio)

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage vlt configuration",
		Long:  "Utilities for generating and validating vlt's configuration file.",
		Run: func(cmd *cobra.Command, _ []string) {
			o.configPath, _ = cmd.InheritedFlags().GetString("config")

			clierror.Check(genericclioptions.RejectGlobalFlags(cmd, hiddenFlags...))
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))

			o.Infof("Resolved config at %q:\n\n%s\n", o.configPath, o.config)
		},
	}

	cmd.AddCommand(newGenerateCmd(stdio))
	cmd.AddCommand(newValidateCmd(stdio))

	genericclioptions.MarkFlagsHidden(cmd, hiddenFlags...)

	return cmd
}

type generateOptions struct {
	*genericclioptions.StdioOptions
}

var _ genericclioptions.CmdOptions = &generateOptions{}

// newGenerateOptions initializes the options struct.
func newGenerateOptions(stdio *genericclioptions.StdioOptions) *generateOptions {
	return &generateOptions{
		StdioOptions: stdio,
	}
}

func (*generateOptions) Complete() error {
	return nil
}

func (*generateOptions) Validate() error {
	return nil
}

func (o *generateOptions) Run(context.Context) error {
	out, err := toml.Marshal(&Config{})
	clierror.Check(err)

	o.Infof("%s", string(out))

	return nil
}

// newGenerateCmd creates the 'generate' subcommand for generating default config.
func newGenerateCmd(stdio *genericclioptions.StdioOptions) *cobra.Command {
	hiddenFlags := []string{"file", "config"}
	o := newGenerateOptions(stdio)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Print a default config file",
		Long: `Generate and print a default configuration file in TOML format.
This command does not take any arguments. It prints the configuration to stdout.`,
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.RejectGlobalFlags(cmd, hiddenFlags...))
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	genericclioptions.MarkFlagsHidden(cmd, hiddenFlags...)

	return cmd
}

type validateOptions struct {
	*genericclioptions.StdioOptions

	configPath string
}

var _ genericclioptions.CmdOptions = &validateOptions{}

// newValidateOptions initializes the options struct.
func newValidateOptions(stdio *genericclioptions.StdioOptions) *validateOptions {
	return &validateOptions{
		StdioOptions: stdio,
	}
}

func (o *validateOptions) Complete() error {
	if len(o.configPath) == 0 {
		f, err := ResolveConfigPath()
		if err != nil {
			return err
		}

		o.configPath = f
	}

	return nil
}

func (*validateOptions) Validate() error {
	return nil
}

func (o *validateOptions) Run(context.Context) error {
	_, err := LoadConfig(o.configPath)
	clierror.Check(err)

	o.Infof("%s: OK\n", o.configPath)

	return nil
}

// newValidateCmd creates the 'validate' subcommand for validating the config file.
func newValidateCmd(stdio *genericclioptions.StdioOptions) *cobra.Command {
	hiddenFlags := []string{"file"}
	o := newValidateOptions(stdio)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Check config validity",
		Long: `Validate the configuration file by loading and checking for common errors.
Defaults to the standard config path if --file is not provided.`,
		Run: func(cmd *cobra.Command, _ []string) {
			o.configPath, _ = cmd.InheritedFlags().GetString("config")

			clierror.Check(genericclioptions.RejectGlobalFlags(cmd, hiddenFlags...))
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	genericclioptions.MarkFlagsHidden(cmd, hiddenFlags...)

	return cmd
}
