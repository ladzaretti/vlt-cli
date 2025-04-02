package cmd

import (
	"io"
	"log"
	"os"

	"github.com/ladzaretti/vlt-cli/pkg/cmd/create"
	"github.com/ladzaretti/vlt-cli/pkg/cmd/login"

	"github.com/spf13/cobra"
)

type vltOptions struct {
	verbose bool
}

func NewDefaultVltCommand() *cobra.Command {
	o := vltOptions{}
	cmd := &cobra.Command{
		Use:   "vlt",
		Short: "Vault CLI for managing secrets",
		Long:  "vlt is a command-line password manager for securely storing and retrieving credentials.",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			log.SetFlags(0)
			log.SetOutput(io.Discard)

			if o.verbose {
				log.SetOutput(os.Stderr)
			}
		},
	}

	cmd.PersistentFlags().BoolVarP(&o.verbose, "verbose", "v", false,
		"Enable verbose output")

	cmd.AddCommand(login.NewCmdLogin())
	cmd.AddCommand(create.NewCmdCreate())

	return cmd
}
