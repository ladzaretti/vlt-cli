package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/ladzaretti/vlt-cli/store"
)

const defaultFilename = ".vlt"

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
		// TODO: use stdlib logger and set the log level based on the verbose flag
		fmt.Printf("Could not read user password: %v\n", err)
		return errors.New("read user password failure")
	}

	_ = pass

	if _, err := store.New(c.filePath); err != nil {
		return fmt.Errorf("create new data store: %w", err)
	}

	return nil
}

func (c *createCmd) readPassword() (string, error) {
	if c.stdin {
		fmt.Println("reading from stdin")

		pass, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read from stdin: %w", err)
		}

		return strings.TrimSpace(string(pass)), nil
	}

	return askUserPassword(create)
}

type passwordType int

const (
	_ passwordType = iota
	create
	current
)

func promptPassword(pt passwordType) string {
	switch pt {
	case create:
		return "Enter new password: "
	case current:
		fallthrough
	default:
		return "Enter password: "
	}
}

func askUserPassword(pt passwordType) (string, error) {
	// FIXME: enforce pass len
	fmt.Println(promptPassword(pt))

	pb, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	pass := string(pb)

	if pt != create {
		return pass, nil
	}

	fmt.Println("Retype password: ")

	pb2, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}

	pass2 := string(pb2)

	if pass2 != pass {
		fmt.Println("Passwords do not match. Please try again.")
		return "", errors.New("user password mismatch")
	}

	return pass, nil
}
