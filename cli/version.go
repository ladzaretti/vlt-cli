package cli

import (
	"github.com/ladzaretti/vlt-cli/genericclioptions"

	"github.com/spf13/cobra"
)

func newVersionCommand(defaults *DefaultVltOptions) *cobra.Command {
	cmd := cobra.Command{
		Use:   "version",
		Short: "Show version",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			defaults.Printf("%s\n", Version)
		},
	}

	genericclioptions.MarkAllFlagsHidden(&cmd)

	return &cmd
}
