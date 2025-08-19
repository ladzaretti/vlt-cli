package cli

import "github.com/spf13/cobra"

func newVersionCommand(defaults *DefaultVltOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version",
		RunE: func(_ *cobra.Command, _ []string) error {
			defaults.Printf("%s\n", Version)
			return nil
		},
	}
}
