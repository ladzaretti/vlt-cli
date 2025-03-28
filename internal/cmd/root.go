package cmd

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "vlt",
		Short: "Vault CLI for managing secrets",
		Long:  "vlt is a command-line password manager for securely storing and retrieving credentials.",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			setupLogging(verbose)
		},
	}

	verbose bool
)

func logAndExit(err error, msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s: %v\n\n", msg, err)
	os.Exit(1)
}

func MustInitialize() error {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false,
		"Enable verbose output")

	rootCmd.AddCommand(loginCmd)

	createCmd, err := newCreateCmd()
	if err != nil {
		logAndExit(err, "failed to initialize create command")
	}

	rootCmd.AddCommand(createCmd.cmd)

	return nil
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func setupLogging(enabled bool) {
	log.SetFlags(0)

	if enabled {
		log.SetOutput(os.Stderr)
	} else {
		log.SetOutput(io.Discard)
	}
}
