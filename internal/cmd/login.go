package cmd

import (
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate and set the database file",
	Long:  "Login by specifying the SQLite database file where credentials will be stored.",
	Run: func(cmd *cobra.Command, _ []string) {
		v, _ := cmd.Flags().GetBool("verbose")
		c, _ := cmd.Flags().GetString("config")

		_, _ = v, c
	},
}
