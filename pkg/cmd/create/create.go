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

type createOptions struct {
	filePath string
	stdin    bool
}

func NewCmdCreate() *cobra.Command {
	o := newCreateOptions()

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Initialize a new vault",
		Long:  "Create a new vault by specifying the SQLite database file where credentials will be stored.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return o.runCreate()
		},
	}

	cmd.Flags().StringVarP(&o.filePath, "file", "f", "",
		"Path to the SQLite database file where credentials will be stored")
	cmd.Flags().BoolVarP(&o.stdin, "input", "i", false,
		"Read password from stdin instead of prompting the user")

	return cmd
}

func newCreateOptions() *createOptions {
	return &createOptions{}
}

func (o *createOptions) runCreate() error {
	vaultPath := o.filePath
	if len(vaultPath) == 0 {
		p, err := defaultVaultPath()
		if err != nil {
			return err
		}

		vaultPath = p
	}

	if _, err := os.Stat(vaultPath); !errors.Is(err, fs.ErrNotExist) {
		fmt.Printf("A file already exists at path: %q. Cannot create a new vault.\n", vaultPath)
		return ErrFileExists
	}

	pass, err := o.readPassword()
	if err != nil {
		log.Printf("Could not read user password: %v\n", err)
		return errors.New("read user password failure")
	}

	// TODO: do something with the password
	_ = pass

	log.Printf("Using database filepath: %q", o.filePath)

	if _, err := vlt.Open(o.filePath); err != nil {
		return fmt.Errorf("create new data store: %w", err)
	}

	return nil
}

func (o *createOptions) readPassword() (string, error) {
	if input.IsPiped() && !o.stdin {
		return "", ErrUnexpectedPipedData
	}

	if o.stdin {
		log.Println("Reading password from stdin")

		pass, err := input.ReadTrimStdin()
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}

		return pass, nil
	}

	pass, err := input.PromptNewPassword()
	if err != nil {
		return "", fmt.Errorf("prompt new password: %w", err)
	}

	return pass, nil
}

func defaultVaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home dir: %w", err)
	}

	return filepath.Join(home, defaultFilename), nil
}
