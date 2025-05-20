package cli

import (
	"context"
	"crypto/subtle"
	"fmt"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/input"
	"github.com/ladzaretti/vlt-cli/vault"
	"github.com/ladzaretti/vlt-cli/vaulterrors"

	"github.com/spf13/cobra"
)

// LoginOptions holds data required to run the command.
type LoginOptions struct {
	*genericclioptions.StdioOptions
	vault func() *vault.Vault
}

var _ genericclioptions.CmdOptions = &LoginOptions{}

// NewLoginOptions initializes the options struct.
func NewLoginOptions(stdio *genericclioptions.StdioOptions, vault func() *vault.Vault) *LoginOptions {
	return &LoginOptions{
		StdioOptions: stdio,
		vault:        vault,
	}
}

func (*LoginOptions) Complete() error {
	return nil
}

func (o *LoginOptions) Validate() error {
	if o.NonInteractive {
		return vaulterrors.ErrNonInteractiveUnsupported
	}

	return nil
}

func (o *LoginOptions) Run(context.Context, ...string) error {
	v := o.vault()

	usrKey, err := input.PromptReadSecure(o.Out, int(o.In.Fd()), "Password for vault at %q:", v.Path)
	if err != nil {
		return fmt.Errorf("prompt password: %v", err)
	}

	// TODO: session only needs the aesgcm, not cipherdate -> rewrite proto def
	// FIXME: utilize vltd for auth session:
	// 	  create derived keys and store in in the vltd daemon.
	// TODO2: end session in the logout cmd.
	// TODO3: add session duration config opt.
	// TODO1: possible refactor the table render for easier fzf searching
	// 	  also, consider printing the create/update timestamps
	dbKey, err := "", nil
	if err != nil {
		return fmt.Errorf("get master key: %v", err)
	}

	if subtle.ConstantTimeCompare([]byte(usrKey), []byte(dbKey)) == 0 {
		return vaulterrors.ErrWrongPassword
	}

	o.Infof("Login successful")

	return nil
}

// NewCmdLogin creates the login cobra command.
func NewCmdLogin(stdio *genericclioptions.StdioOptions, vault func() *vault.Vault) *cobra.Command {
	o := NewLoginOptions(stdio, vault)

	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate the user with the vault",
		Long:  "Authenticate the user and grant access to the vault for subsequent operations.",
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}
}
