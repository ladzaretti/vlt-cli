package genericclioptions

import "github.com/spf13/cobra"

func MarkFlagsHidden(sub *cobra.Command, names ...string) {
	f := sub.HelpFunc()
	sub.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		for _, n := range names {
			flag := cmd.Flags().Lookup(n)
			if flag != nil {
				flag.Hidden = true
			}
		}

		f(cmd, args)
	})
}
