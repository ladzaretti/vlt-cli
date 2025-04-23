package genericclioptions

import (
	"fmt"

	"github.com/spf13/cobra"
)

func MarkFlagsHidden(sub *cobra.Command, hidden ...string) {
	f := sub.HelpFunc()
	sub.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		for _, n := range hidden {
			flag := cmd.Flags().Lookup(n)
			if flag != nil {
				flag.Hidden = true
			}
		}

		f(cmd, args)
	})
}

func RejectGlobalFlags(cmd *cobra.Command, disallowed ...string) error {
	for _, name := range disallowed {
		if cmd.Flags().Changed(name) {
			return fmt.Errorf("flag --%s is not allowed with '%s' command", name, cmd.Name())
		}
	}

	return nil
}
