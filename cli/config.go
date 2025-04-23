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

// NewCmdConfig creates the cobra config command tree.
func NewCmdConfig(o *genericclioptions.StdioOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage vlt configuration",
		Long:  "Utilities for generating and validating vlt's configuration file.",
	}

	cmd.AddCommand(NewGenerateCmd(o), NewValidateCmd(o))
	genericclioptions.MarkFlagsHidden(cmd, "file", "verbose", "config")

	return cmd
}

type GenerateOptions struct {
	*genericclioptions.StdioOptions
}

var _ genericclioptions.CmdOptions = &GenerateOptions{}

// NewGenerateOptions initializes the options struct.
func NewGenerateOptions(stdio *genericclioptions.StdioOptions) *GenerateOptions {
	return &GenerateOptions{
		StdioOptions: stdio,
	}
}

func (*GenerateOptions) Complete() error {
	return nil
}

func (*GenerateOptions) Validate() error {
	return nil
}

func (o *GenerateOptions) Run(context.Context) error {
	out, err := toml.Marshal(&Config{})
	clierror.Check(err)

	o.Infof("%s", string(out))

	return nil
}

// NewGenerateCmd creates the 'generate' subcommand for generating default config.
func NewGenerateCmd(stdio *genericclioptions.StdioOptions) *cobra.Command {
	o := NewGenerateOptions(stdio)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Print a default config file",
		Long: `Generate and print a default configuration file in TOML format.
This command does not take any arguments. It prints the configuration to stdout.`,
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	genericclioptions.MarkFlagsHidden(cmd, "file", "verbose", "config")

	return cmd
}

type ValidateOptions struct {
	*genericclioptions.StdioOptions

	file string
}

var _ genericclioptions.CmdOptions = &ValidateOptions{}

// NewValidateOptions initializes the options struct.
func NewValidateOptions(stdio *genericclioptions.StdioOptions) *ValidateOptions {
	return &ValidateOptions{
		StdioOptions: stdio,
	}
}

func (o *ValidateOptions) Complete() error {
	if len(o.file) == 0 {
		f, err := ResolveConfigPath()
		clierror.Check(err)

		o.file = f
	}

	return nil
}

func (*ValidateOptions) Validate() error {
	return nil
}

func (o *ValidateOptions) Run(context.Context) error {
	_, err := LoadConfig(o.file)
	clierror.Check(err)

	o.Infof("%s: OK\n", o.file)

	return nil
}

// NewValidateCmd creates the 'validate' subcommand for validating the config file.
func NewValidateCmd(stdio *genericclioptions.StdioOptions) *cobra.Command {
	o := NewValidateOptions(stdio)

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Check config validity",
		Long: `Validate the configuration file by loading and checking for common errors.
Defaults to the standard config path if --file is not provided.`,
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	cmd.Flags().StringP("file", "f", "", "config file to validate")
	genericclioptions.MarkFlagsHidden(cmd, "verbose", "config")

	return cmd
}
