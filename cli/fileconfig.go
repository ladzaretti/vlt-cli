package cli

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const (
	// envConfigPathKey is the environment variable key for overriding
	// the config file path.
	envConfigPathKey = "VLT_CONFIG_PATH"
)

type ConfigError struct {
	Opt string
	Err error
}

func (e *ConfigError) Error() string { return "config: " + e.Opt + ": " + e.Err.Error() }

func (e *ConfigError) Unwrap() error { return e.Err }

// FileConfig represents the full structure of the configuration file,
//
//nolint:tagalign
type FileConfig struct {
	Vault     VaultConfig      `toml:"vault" json:"vault"`
	Clipboard *ClipboardConfig `toml:"clipboard,commented" comment:"Clipboard configuration: Both copy and paste commands must be either both set or both unset." json:"clipboard"`
	Pipeline  *PipelineConfig  `toml:"pipeline,commented" comment:"Pipeline configuration for vault search commands (e.g., 'vlt find')"`

	path string // path to the loaded config file. Empty if no config file was used.
}

func newFileConfig() FileConfig {
	return FileConfig{
		Clipboard: &ClipboardConfig{},
		Pipeline:  &PipelineConfig{},
	}
}

// VaultConfig holds vault-related configuration.
//
//nolint:tagalign,tagliatelle
type VaultConfig struct {
	Path            string `toml:"path,commented" comment:"Vlt database path (default: '~/.vlt' if not set)" json:"path,omitempty"`
	SessionDuration string `toml:"session_duration,commented" comment:"How long a session lasts before requiring login again (default: '1m')" json:"session_duration,omitempty"`
}

// ClipboardConfig defines commands for clipboard ops.
//
//nolint:tagalign,tagliatelle
type ClipboardConfig struct {
	CopyCmd  string `toml:"copy_cmd,commented"  comment:"The command used for copying to the clipboard (default: 'xsel -ib' if not set)" json:"copy_cmd,omitempty"`
	PasteCmd string `toml:"paste_cmd,commented" comment:"The command used for pasting from the clipboard (default: 'xsel -ob' if not set)" json:"paste_cmd,omitempty"`
}

// Pipeline configuration for vault search commands.
//
//nolint:tagalign,tagliatelle
type PipelineConfig struct {
	FindPipeCmd []string `toml:"find_pipe_cmd,commented" comment:"Optional command to pipe 'vlt find' output through (e.g. [\"sh\", \"-c\", \"fzf\"])" json:"find_pipe_cmd,omitempty"`
}

// LoadFileConfig loads the config from the given or default path.
func LoadFileConfig(path string) (*FileConfig, error) {
	defaultPath, err := defaultConfigPath()
	if err != nil {
		return nil, err
	}

	configPath := cmp.Or(path, defaultPath)

	c, err := parseFileConfig(configPath)
	if err != nil {
		// config file not found at default location; fallback to empty config
		if len(path) == 0 && errors.Is(err, fs.ErrNotExist) { //nolint:revive // clearer with explicit fallback logic
			c = &FileConfig{}
		} else {
			return nil, err
		}
	} else {
		c.path = configPath
	}

	return c, c.validate()
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

func parseFileConfig(path string) (*FileConfig, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("config: stat file: %w", err)
	}

	raw, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	config := FileConfig{}
	if err := toml.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("config: parse file: %w", err)
	}

	return &config, nil
}

func (c *FileConfig) validate() error {
	if c.hasPartialClipboard() {
		return &ConfigError{Opt: "clipboard", Err: errors.New("both 'copy_cmd' and 'paste_cmd' must be set or unset together")}
	}

	if c.Pipeline.FindPipeCmd != nil && len(c.Pipeline.FindPipeCmd) == 0 {
		return &ConfigError{Opt: "find_pipe_cmd", Err: errors.New("defined but contains no values")}
	}

	return nil
}

// hasPartialClipboard checks if only one of the clipboard commands is set.
func (c *FileConfig) hasPartialClipboard() bool {
	return (c.Clipboard.CopyCmd == "") != (c.Clipboard.PasteCmd == "")
}
