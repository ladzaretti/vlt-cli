package cmd

import (
	"io"
	"log"
	"os"

	"github.com/ladzaretti/vlt-cli/pkg/cmd/create"
	"github.com/ladzaretti/vlt-cli/pkg/cmd/login"
	"github.com/ladzaretti/vlt-cli/pkg/genericclioptions"

	"github.com/spf13/cobra"
)

// NewDefaultVltCommand creates the `vlt` command with its sub-commands.
func NewDefaultVltCommand() *cobra.Command {
	opts := genericclioptions.Opts{}
	cmd := &cobra.Command{
		Use:   "vlt",
		Short: "vault CLI for managing secrets",
		Long:  "vlt is a command-line password manager for securely storing and retrieving credentials.",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			log.SetFlags(0)
			log.SetOutput(io.Discard)

			if opts.Verbose {
				log.SetOutput(os.Stderr)
			}
		},
	}

	cmd.PersistentFlags().BoolVarP(&opts.Verbose, "verbose", "v", false,
		"enable verbose output")
	cmd.PersistentFlags().StringVarP(&opts.File, "file", "f", "",
		"path to the SQLite database file where credentials will be stored")

	cmd.AddCommand(login.NewCmdLogin())
	cmd.AddCommand(create.NewCmdCreate(opts))

	return cmd
}
