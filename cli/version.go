package cli

import "github.com/spf13/cobra"

func newVersionCommand(defaults *DefaultVltOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "show version",
		Run: func(_ *cobra.Command, _ []string) {
			defaults.Infof("%s", Version)
		},
	}
}
