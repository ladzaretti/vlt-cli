package create

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ladzaretti/vlt-cli/pkg/input"
	"github.com/ladzaretti/vlt-cli/vlt"
)

const (
	defaultFilename = ".vlt"
)

var (
	ErrUnexpectedPipedData = errors.New("unexpected piped data")
	ErrFileExists          = errors.New("vault file path already exists")
)

// createOptions have the data required to perform the create operation.
type createOptions struct {
	filePath string
	stdin    bool
}

// newCreateOptions creates the options for create.
func newCreateOptions() *createOptions {
	return &createOptions{}
}

// NewCmdCreate creates a new create command.
func NewCmdCreate() *cobra.Command {
	o := newCreateOptions()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Initialize a new vault",
		Long:  "Create a new vault by specifying the SQLite database file where credentials will be stored.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return o.run()
		},
	}

	cmd.Flags().StringVarP(&o.filePath, "file", "f", "",
		"Path to the SQLite database file where credentials will be stored")
	cmd.Flags().BoolVarP(&o.stdin, "input", "i", false,
		"Read password from stdin instead of prompting the user")

	return cmd
}

func (o *createOptions) run() error {
	if err := o.resolveFilePath(); err != nil {
		return err
	}

	log.Printf("Using database filepath: %q", o.filePath)

	mk, err := o.readNewMasterKey()
	if err != nil {
		log.Printf("Could not read user password: %v\n", err)
		return fmt.Errorf("read user password: %v", err)
	}

	vault, err := vlt.Open(o.filePath)
	if err != nil {
		return fmt.Errorf("create new vault: %v", err)
	}

	if err := vault.SetMasterKey(mk); err != nil {
		log.Printf("Failure setting vault master key: %v\n", err)
		return fmt.Errorf("set master key: %v", err)
	}

	return nil
}

// resolveFilePath ensures the file path is set and checks for conflicts.
func (o *createOptions) resolveFilePath() error {
	if len(o.filePath) == 0 {
		p, err := defaultVaultPath()
		if err != nil {
			return err
		}

		o.filePath = p
	}

	if _, err := os.Stat(o.filePath); !errors.Is(err, fs.ErrNotExist) {
		fmt.Printf("A file already exists at path: %q. Cannot create a new vault.\n", o.filePath)
		return ErrFileExists
	}

	return nil
}

func (o *createOptions) readNewMasterKey() (string, error) {
	if input.IsPiped() && !o.stdin {
		return "", ErrUnexpectedPipedData
	}

	if o.stdin {
		log.Println("Reading password from stdin")

		pass, err := input.ReadTrimStdin()
		if err != nil {
			return "", fmt.Errorf("read password: %v", err)
		}

		return pass, nil
	}

	pass, err := input.PromptNewPassword()
	if err != nil {
		return "", fmt.Errorf("prompt new password: %v", err)
	}

	return pass, nil
}

func defaultVaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %v", err)
	}

	return filepath.Join(home, defaultFilename), nil
}
