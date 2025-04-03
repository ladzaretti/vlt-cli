package genericclioptions

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrFileExists = errors.New("vault file path already exists")

const (
	defaultFilename = ".vlt"
)

type Opts struct {
	Verbose bool
	File    string
}

// ResolveFilePath ensures the file path is set and checks for conflicts.
func (o *Opts) ResolveFilePath() error {
	if len(o.File) == 0 {
		p, err := defaultVaultPath()
		if err != nil {
			return err
		}

		o.File = p
	}

	if _, err := os.Stat(o.File); !errors.Is(err, fs.ErrNotExist) {
		fmt.Printf("A file already exists at path: %q. Cannot create a new vault.\n", o.File)
		return ErrFileExists
	}

	return nil
}

func defaultVaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %v", err)
	}

	return filepath.Join(home, defaultFilename), nil
}
