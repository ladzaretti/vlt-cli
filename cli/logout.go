package cli

import (
	"context"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vlt"

	"github.com/spf13/cobra"
)

type LogoutOptions struct {
	*genericclioptions.StdioOptions
	vault func() *vlt.Vault
}

var _ genericclioptions.CmdOptions = &LogoutOptions{}

// NewLogoutOptions initializes the options struct.
func NewLogoutOptions(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *LogoutOptions {
	return &LogoutOptions{
		StdioOptions: stdio,
		vault:        vault,
	}
}

func (*LogoutOptions) Complete() error {
	return nil
}

func (*LogoutOptions) Validate() error {
	return nil
}

func (*LogoutOptions) Run(context.Context) error {
	return nil
}

// NewCmdLogout creates the logout cobra command.
func NewCmdLogout(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *cobra.Command {
	o := NewLogoutOptions(stdio, vault)

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logs the user out of the current session",
		Long:  "Logs the user out of the current session.",
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	return cmd
}
