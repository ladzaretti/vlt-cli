package cli

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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

func (e *ConfigError) Error() string {
	return "config: " + strings.Join([]string{e.Opt, e.Err.Error()}, ":")
}

func (e *ConfigError) Unwrap() error { return e.Err }

// FileConfig represents the full structure of the configuration file,
//
//nolint:tagalign
type FileConfig struct {
	Vault     VaultConfig      `toml:"vault" json:"vault"`
	Clipboard *ClipboardConfig `toml:"clipboard" comment:"Clipboard configuration: Both copy and paste commands must be either both set or both unset." json:"clipboard"`
	Hooks     *HooksConfig     `toml:"hooks" comment:"Optional lifecycle hooks for vault events" json:"hooks"`

	path string // path to the loaded config file. Empty if no config file was used.
}

func newFileConfig() *FileConfig {
	return &FileConfig{
		Clipboard: &ClipboardConfig{},
		Hooks:     &HooksConfig{},
	}
}

// VaultConfig holds vault-related configuration.
//
//nolint:tagalign,tagliatelle
type VaultConfig struct {
	Path                string `toml:"path,commented" comment:"Vlt database path (default: '~/.vlt' if not set)" json:"path,omitempty"`
	SessionDuration     string `toml:"session_duration,commented" comment:"How long a session lasts before requiring login again (default: '1m')" json:"session_duration,omitempty"`
	MaxHistorySnapshots *int   `toml:"max_history_snapshots,commented" comment:"Maximum number of historical vault snapshots to keep (default: 3, 0 disables history)" json:"max_history_snapshots,omitempty"`
}

// ClipboardConfig defines commands for clipboard ops.
//
//nolint:tagalign,tagliatelle
type ClipboardConfig struct {
	CopyCmd  []string `toml:"copy_cmd,commented"  comment:"The command used for copying to the clipboard (default: ['xsel', '-ib'] if not set)" json:"copy_cmd,omitempty"`
	PasteCmd []string `toml:"paste_cmd,commented" comment:"The command used for pasting from the clipboard (default: ['xsel', '-ob'] if not set)" json:"paste_cmd,omitempty"`
}

// HooksConfig defines optional lifecycle hooks triggered by vault events.
//
//nolint:tagalign,tagliatelle
type HooksConfig struct {
	PostLoginCmd []string `toml:"post_login_cmd,commented" comment:"Command to run after a successful login" json:"post_login_cmd"`
	PostWriteCmd []string `toml:"post_write_cmd,commented" comment:"Command to run after any vault write (e.g., create, update, delete)" json:"post_write_cmd"`
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
			c = newFileConfig()
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

	config := newFileConfig()
	if err := toml.Unmarshal(raw, config); err != nil {
		return nil, fmt.Errorf("config: parse file: %w", err)
	}

	return config, nil
}

func (c *FileConfig) validate() error {
	if c == nil {
		return &ConfigError{Err: errors.New("cannot validate a nil config")}
	}

	if c.hasPartialClipboard() {
		return &ConfigError{Opt: "clipboard", Err: errors.New("both 'copy_cmd' and 'paste_cmd' must be set or unset together")}
	}

	if c.Hooks.PostLoginCmd != nil && len(c.Hooks.PostLoginCmd) == 0 {
		return &ConfigError{Opt: "hooks.post_login_cmd", Err: errors.New("defined but contains no values")}
	}

	if c.Hooks.PostWriteCmd != nil && len(c.Hooks.PostWriteCmd) == 0 {
		return &ConfigError{Opt: "hooks.post_write_cmd", Err: errors.New("defined but contains no values")}
	}

	if c.Vault.MaxHistorySnapshots != nil && *c.Vault.MaxHistorySnapshots < 0 {
		return &ConfigError{Opt: "vault.max_history_snapshots", Err: errors.New("must be zero or a positive integer")}
	}

	return nil
}

// hasPartialClipboard checks if only one of the clipboard commands is set.
func (c *FileConfig) hasPartialClipboard() bool {
	return (len(c.Clipboard.CopyCmd) == 0) != (len(c.Clipboard.PasteCmd) == 0)
}
