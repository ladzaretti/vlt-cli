package cmd

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose bool

	rootCmd = &cobra.Command{
		Use:   "vlt",
		Short: "Vault CLI for managing secrets",
		Long:  "vlt is a command-line password manager for securely storing and retrieving credentials.",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			setupLogging()
		},
	}
)

func MustInitialize() error {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"Enable verbose output")

	rootCmd.AddCommand(loginCmd)

	createCmd, err := newCreateCmd()
	if err != nil {
		return fmt.Errorf("failed to initialize create command: %w", err)
	}

	rootCmd.AddCommand(createCmd.cmd)

	return nil
}

func Execute() error {
	//nolint:wrapcheck
	return rootCmd.Execute()
}

func setupLogging() {
	log.SetFlags(0)
	log.SetOutput(io.Discard)

	if verbose {
		log.SetOutput(os.Stderr)
	}
}
