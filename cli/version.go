package cli

import (
	"errors"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"

	"github.com/spf13/cobra"
)

func newVersionCommand(defaults *DefaultVltOptions) *cobra.Command {
	cmd := cobra.Command{
		Use:                "version",
		Short:              "Show version",
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return clierror.Check(func() error {
				if len(args) > 0 {
					return errors.New("version: command takes no arguments")
				}

				defaults.Printf("%s\n", Version)

				return nil
			}())
		},
	}

	genericclioptions.MarkAllFlagsHidden(&cmd)

	return &cmd
}
