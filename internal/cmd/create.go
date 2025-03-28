package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ladzaretti/vlt-cli/input"
	vlt "github.com/ladzaretti/vlt-cli/vlt"
)

const (
	defaultFilename = ".vlt"
)

var ErrUnexpectedPipedData = errors.New("unexpected piped data")

type createCmd struct {
	cmd *cobra.Command

	filePath string
	stdin    bool
}

func newCreateCmd() (*createCmd, error) {
	createCmd := &createCmd{}
	createCmd.cmd = &cobra.Command{
		Use:   "create",
		Short: "Initialize a new vault",
		Long:  "Create a new vault by specifying the SQLite database file where credentials will be stored.",
		RunE:  createCmd.run,
	}

	if err := createCmd.init(); err != nil {
		return nil, fmt.Errorf("cmd init: %w", err)
	}

	return createCmd, nil
}

func (c *createCmd) init() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("user home dir: %w", err)
	}

	defaultFilepath := filepath.Join(home, defaultFilename)

	c.cmd.Flags().StringVarP(&c.filePath, "file", "f", defaultFilepath,
		"Path to the SQLite database file where credentials will be stored")

	c.cmd.Flags().BoolVarP(&c.stdin, "input", "i", false,
		"Read password from stdin instead of prompting the user")

	return nil
}

func (c *createCmd) run(_ *cobra.Command, _ []string) error {
	pass, err := c.readPassword()
	if err != nil {
		log.Printf("Could not read user password: %v\n", err)
		return errors.New("read user password failure")
	}

	// TODO: do something with the password
	_ = pass

	log.Printf("Using database filepath: %q", c.filePath)

	if _, err := vlt.Open(c.filePath); err != nil {
		return fmt.Errorf("create new data store: %w", err)
	}

	return nil
}

func (c *createCmd) readPassword() (string, error) {
	if input.IsPiped() && !c.stdin {
		return "", ErrUnexpectedPipedData
	}

	if c.stdin {
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
