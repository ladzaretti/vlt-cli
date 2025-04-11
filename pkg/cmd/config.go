package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const (
	// defaultConfigName is the default name of the configuration file
	// expected under the user's home directory.
	defaultConfigName = ".vlt.toml"

	// envConfigPathKey is the environment variable key for overriding
	// the config file path.
	envConfigPathKey = "VLT_CONFIG_PATH"
)

var (
	ErrNoConfigAvailable      = errors.New("no config file available")
	ErrPartialClipboardConfig = errors.New("invalid partial clipboard config")
)

type VaultConfig struct {
	Path string `toml:"path"`
}

type ClipboardConfig struct {
	CopyCmd  string `toml:"copy_cmd"`
	PasteCmd string `toml:"paste_cmd"`
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

// LoadConfig loads the configuration from a file and validates it.
// It returns [ErrNoConfigAvailable] if no config file is found.
func LoadConfig() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, ErrNoConfigAvailable
	}

	path := filepath.Join(home, defaultConfigName)
	if p, ok := os.LookupEnv(envConfigPathKey); ok {
		path = p
	}

	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Config{}, ErrNoConfigAvailable
		}

		return Config{}, fmt.Errorf("stat vault config file: %w", err)
	}

	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Config{}, err
	}

	config := Config{}
	if err := toml.Unmarshal(raw, &config); err != nil {
		return Config{}, err
	}

	return config, config.Validate()
}
