package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	login = &cobra.Command{
		Use:   "login",
		Short: "Authenticate and set the database file",
		Long:  "Login by specifying the SQLite database file where credentials will be stored.",
		Run: func(cmd *cobra.Command, _ []string) {
			v, _ := cmd.Flags().GetBool("verbose")
			c, _ := cmd.Flags().GetString("config")
			fmt.Printf("verbose: %v; config: %v\n", v, c)
			fmt.Println("hello from login")
		},
	}
)
