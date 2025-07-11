package cli

import (
	"context"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vaultdaemon"

	"github.com/spf13/cobra"
)

type LogoutOptions struct {
	*genericclioptions.StdioOptions
	*VaultOptions

	sessionClient *vaultdaemon.SessionClient
}

var _ genericclioptions.CmdOptions = &LogoutOptions{}

// NewLogoutOptions initializes the options struct.
func NewLogoutOptions(stdio *genericclioptions.StdioOptions, vaultOptions *VaultOptions) *LogoutOptions {
	return &LogoutOptions{
		StdioOptions: stdio,
		VaultOptions: vaultOptions,
	}
}

func (o *LogoutOptions) Complete() error {
	s, err := vaultdaemon.NewSessionClient()
	if err != nil {
		return err
	}

	o.sessionClient = s

	return nil
}

func (*LogoutOptions) Validate() error { return nil }

func (o *LogoutOptions) Run(ctx context.Context, _ ...string) error {
	defer func() { _ = o.Close() }()

	o.Infof("logging out of %q\n", o.path)

	if err := o.sessionClient.Logout(ctx, o.path); err != nil {
		return err
	}

	o.Infof("success\n")

	return nil
}

func (o *LogoutOptions) Close() error {
	return o.sessionClient.Close()
}

// NewCmdLogout creates the logout cobra command.
func NewCmdLogout(defaults *DefaultVltOptions) *cobra.Command {
	o := NewLogoutOptions(
		defaults.StdioOptions,
		defaults.vaultOptions,
	)

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out of the current session",
		Long:  "Log out of the current session.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	return cmd
}
