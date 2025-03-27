package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var root = &cobra.Command{
	Use:   "vlt",
	Short: "Vault CLI for managing secrets",
	Long:  "vlt is a command-line password manager for securely storing and retrieving credentials.",
}

func logAndExit(err error, msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s: %v\n\n", msg, err)
	os.Exit(1)
}

func MustInitialize() error {
	root.PersistentFlags().BoolP("verbose", "v", false,
		"Enable verbose output")

	root.AddCommand(login) // Attach login command

	createCmd, err := newCreateCmd()
	if err != nil {
		logAndExit(err, "failed to initialize create command")
	}

	root.AddCommand(createCmd.cmd)

	return nil
}

func Execute() error {
	return root.Execute()
}
