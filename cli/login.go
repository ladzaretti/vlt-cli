package cli

import (
	"context"
	"fmt"

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
	path    func() string
	session *vaultdaemon.SessionClient
}

var _ genericclioptions.CmdOptions = &LoginOptions{}

// NewLoginOptions initializes the options struct.
func NewLoginOptions(stdio *genericclioptions.StdioOptions, path func() string) *LoginOptions {
	return &LoginOptions{
		StdioOptions: stdio,
		path:         path,
	}
}

func (o *LoginOptions) Complete() error {
	s, err := vaultdaemon.NewSessionClient()
	if err != nil {
		return err
	}

	o.session = s

	return nil
}

func (o *LoginOptions) Validate() error {
	if o.NonInteractive {
		return vaulterrors.ErrNonInteractiveUnsupported
	}

	return nil
}

func (o *LoginOptions) Run(ctx context.Context, _ ...string) error {
	path := o.path()

	password, err := input.PromptReadSecure(o.Out, int(o.In.Fd()), "[vlt] Password for %q:", path)
	if err != nil {
		return fmt.Errorf("prompt password: %v", err)
	}

	key, nonce, err := vault.Login(ctx, path, password)
	if err != nil {
		return err
	}

	if err := o.session.Login(ctx, path, key, nonce, "1m"); err != nil {
		return err
	}

	// TODO: session only needs the aesgcm, not cipherdate -> rewrite proto def
	// FIXME: utilize vltd for auth session:
	// 	  create derived keys and store in in the vltd daemon.
	// TODO2: end session in the logout cmd.
	// TODO3: add session duration config opt.
	// TODO1: possible refactor the table render for easier fzf searching
	// 	  also, consider printing the create/update timestamps

	o.Infof("Login successful")

	return nil
}

// NewCmdLogin creates the login cobra command.
func NewCmdLogin(stdio *genericclioptions.StdioOptions, path func() string) *cobra.Command {
	o := NewLoginOptions(stdio, path)

	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate the user with the vault",
		Long:  "Authenticate the user and grant access to the vault for subsequent operations.",
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}
}
