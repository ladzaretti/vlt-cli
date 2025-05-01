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
	Path string `toml:"path,commented" comment:"Vlt database path (default: '~/.vlt' if not set)"`
}

//nolint:tagalign
type ClipboardConfig struct {
	CopyCmd  string `toml:"copy_cmd,commented"  comment:"The command used for copying to the clipboard (default: 'xsel -ib' if not set)"`
	PasteCmd string `toml:"paste_cmd,commented" comment:"The command used for pasting from the clipboard (default: 'xsel -ob' if not set)"`
}

//nolint:tagalign
type Config struct {
	Vault     VaultConfig     `toml:"vault"`
	Clipboard ClipboardConfig `toml:"clipboard,commented" comment:"Clipboard configuration: both copy and paste commands must be provided."`

	path string // path is the resolved file path from which this config was loaded
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

func defaultConfigPath() (string, error) {
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

// LoadConfig reads the configuration from the given file path.
// If no path is provided, it uses the default config path (~/.vlt.toml).
//
// Returns an empty Config if no config file is found and no path was explicitly given.
func LoadConfig(userPath string) (Config, error) {
	path := userPath
	userProvided := len(userPath) > 0

	if !userProvided {
		f, err := defaultConfigPath()
		if err != nil {
			return Config{}, err
		}

		path = f
	}

	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if !userProvided {
				return Config{}, nil
			}

			return Config{}, fmt.Errorf("config: no config file found at %q", path)
		}

		return Config{}, fmt.Errorf("config: stat file: %w", err)
	}

	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Config{}, err
	}

	config := Config{path: path}
	if err := toml.Unmarshal(raw, &config); err != nil {
		return Config{}, fmt.Errorf("config: parse file: %w", err)
	}

	return config, config.Validate()
}

type ConfigOptions struct {
	*genericclioptions.StdioOptions

	Config
	userPath string // userPath is the config file path explicitly provided by the user, if any.
}

var _ genericclioptions.CmdOptions = &ConfigOptions{}

// NewConfigOptions initializes the options struct.
func NewConfigOptions(stdio *genericclioptions.StdioOptions) *ConfigOptions {
	return &ConfigOptions{
		StdioOptions: stdio,
	}
}

func (*ConfigOptions) Complete() error {
	return nil
}

func (*ConfigOptions) Validate() error {
	return nil
}

func (o *ConfigOptions) Run(context.Context) error {
	c, err := LoadConfig(o.userPath)
	if err != nil {
		return err
	}

	o.Config = c

	return nil
}

// NewCmdConfig creates the cobra config command tree.
func NewCmdConfig(stdio *genericclioptions.StdioOptions) *cobra.Command {
	hiddenFlags := []string{"config"}
	o := NewConfigOptions(stdio)

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Resolve validate and output vlt configuration (subcommands available)",
		Long:  "Resolve validate and output vlt configuration.",
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.RejectGlobalFlags(cmd, hiddenFlags...))
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))

			if len(o.path) == 0 {
				o.Infof("No config file found; using default values.\n")
				return
			}

			o.Infof("Resolved config at %q:\n\n%s\n", o.path, o.Config)
		},
	}

	cmd.PersistentFlags().StringVarP(&o.userPath, "file", "f", "",
		fmt.Sprintf("path to the configuration file (default: ~/%s)", defaultConfigName))

	cmd.AddCommand(newGenerateConfigCmd(stdio))
	cmd.AddCommand(newValidateConfigCmd(stdio))

	genericclioptions.MarkFlagsHidden(cmd, hiddenFlags...)

	return cmd
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

func (*generateConfigOptions) Complete() error {
	return nil
}

func (*generateConfigOptions) Validate() error {
	return nil
}

func (o *generateConfigOptions) Run(context.Context) error {
	out, err := toml.Marshal(&Config{})
	clierror.Check(err)

	o.Infof("%s", string(out))

	return nil
}

// newGenerateConfigCmd creates the 'generate' subcommand for generating default config.
func newGenerateConfigCmd(stdio *genericclioptions.StdioOptions) *cobra.Command {
	hiddenFlags := []string{"file", "config"}
	o := newGenerateConfigOptions(stdio)

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

func (*validateConfigOptions) Complete() error {
	return nil
}

func (*validateConfigOptions) Validate() error {
	return nil
}

func (o *validateConfigOptions) Run(context.Context) error {
	c, err := LoadConfig(o.configPath)
	clierror.Check(err)

	if len(c.path) == 0 {
		o.Infof("No config file found; Nothing to validate.\n")
		return nil
	}

	o.Infof("%s: OK\n", c.path)

	return nil
}

// newValidateConfigCmd creates the 'validate' subcommand for validating the config file.
func newValidateConfigCmd(stdio *genericclioptions.StdioOptions) *cobra.Command {
	hiddenFlags := []string{"config"}
	o := newValidateConfigOptions(stdio)

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
