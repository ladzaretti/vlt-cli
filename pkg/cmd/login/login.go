package login

import (
	"github.com/spf13/cobra"
)

func NewCmdLogin() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "authenticate and set the database file",
		Long:  "login by specifying the SQLite database file where credentials will be stored.",
		Run: func(cmd *cobra.Command, _ []string) {
			v, _ := cmd.Flags().GetBool("verbose")
			c, _ := cmd.Flags().GetString("config")

			_, _ = v, c
		},
	}
}
