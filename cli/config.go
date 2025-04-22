package cli

import (
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

// NewCmdConfig creates the config cobra command.
func NewCmdConfig(o *genericclioptions.StdioOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage vlt configuration",
		Long:  "Utilities for generating and validating vlt's configuration file.",
	}

	cmd.AddCommand(NewGenerateCmd(o), newValidateCmd(o))
	genericclioptions.MarkFlagsHidden(cmd, "file", "verbose", "config")

	return cmd
}

// NewGenerateCmd creates the 'generate' subcommand for generating default config.
func NewGenerateCmd(o *genericclioptions.StdioOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Print a default config file",
		Long: `Generate and print a default configuration file in TOML format.
This command does not take any arguments. It prints the configuration to stdout.`,
		Run: func(_ *cobra.Command, _ []string) {
			out, err := toml.Marshal(&Config{})
			clierror.Check(err)
			o.Infof("%s", string(out))
		},
	}

	genericclioptions.MarkFlagsHidden(cmd, "file", "verbose", "config")

	return cmd
}

// newValidateCmd creates the 'validate' subcommand for validating the config file.
func newValidateCmd(o *genericclioptions.StdioOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Check config validity",
		Long: `Validate the configuration file by loading and checking for common errors.
Defaults to the standard config path if --file is not provided.`,
		Run: func(cmd *cobra.Command, _ []string) {
			file, _ := cmd.Flags().GetString("file")
			if len(file) == 0 {
				f, err := ResolveConfigPath()
				clierror.Check(err)
				file = f
			}
			_, err := LoadConfig(file)
			clierror.Check(err)
			o.Infof("%s: OK\n", file)
		},
	}

	cmd.Flags().StringP("file", "f", "", "config file to validate")
	genericclioptions.MarkFlagsHidden(cmd, "verbose", "config")

	return cmd
}
