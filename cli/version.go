package cli

import (
	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"

	"github.com/spf13/cobra"
)

func newVersionCommand(defaults *DefaultVltOptions) *cobra.Command {
	cmd := cobra.Command{
		Use:   "version",
		Short: "Show version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			return clierror.Check(func() error {
				defaults.Printf("%s\n", Version)

				return nil
			}())
		},
	}

	genericclioptions.MarkAllFlagsHidden(&cmd)

	return &cmd
}
