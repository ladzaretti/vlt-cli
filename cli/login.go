package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/input"
	"github.com/ladzaretti/vlt-cli/vault"
	"github.com/ladzaretti/vlt-cli/vaultdaemon"
	"github.com/ladzaretti/vlt-cli/vaulterrors"

	"github.com/spf13/cobra"
)

// LoginOptions holds data required to run the command.
type LoginOptions struct {
	*genericclioptions.StdioOptions
	path          func() string
	config        func() *ResolvedConfig
	sessionClient *vaultdaemon.SessionClient
}

var _ genericclioptions.CmdOptions = &LoginOptions{}

// NewLoginOptions initializes the options struct.
func NewLoginOptions(stdio *genericclioptions.StdioOptions, path func() string, config func() *ResolvedConfig) *LoginOptions {
	return &LoginOptions{
		StdioOptions: stdio,
		config:       config,
		path:         path,
	}
}

func (o *LoginOptions) Complete() error {
	s, err := vaultdaemon.NewSessionClient()
	if err != nil {
		return err
	}

	o.sessionClient = s

	return nil
}

func (o *LoginOptions) Validate() error {
	if o.NonInteractive {
		return vaulterrors.ErrNonInteractiveUnsupported
	}

	return nil
}

func (o *LoginOptions) Close() error {
	return o.sessionClient.Close()
}

func (o *LoginOptions) Run(ctx context.Context, _ ...string) error {
	defer func() { _ = o.Close() }()

	path := o.path()

	password, err := input.PromptReadSecure(o.Out, int(o.In.Fd()), "[vlt] Password for %q:", path)
	if err != nil {
		return fmt.Errorf("prompt password: %v", err)
	}

	key, nonce, err := vault.Login(ctx, path, password)
	if err != nil {
		return err
	}

	sessionDuration := time.Duration(o.config().SessionDuration)
	if err := o.sessionClient.Login(ctx, path, key, nonce, sessionDuration); err != nil {
		return err
	}

	// TODO1: possible refactor the table render for easier fzf searching
	// TODO2: consider printing the create/update timestamps
	// FIXME: remote history table ? maybe restrict snapshot count
	// FIXME2: remove highlight; its complex and to be fair redundant with fzf

	o.Infof("Login successful")

	return nil
}

// NewCmdLogin creates the login cobra command.
func NewCmdLogin(vltOpts *DefaultVltOptions) *cobra.Command {
	o := NewLoginOptions(vltOpts.StdioOptions, vltOpts.vaultOptions.Path, vltOpts.configOptions.Resolved)

	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate the user with the vault",
		Long:  "Authenticate the user and grant access to the vault for subsequent operations.",
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}
}
