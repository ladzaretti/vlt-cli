package login // replace with your actual package name

import (
	"context"
	"crypto/subtle"
	"fmt"

	"github.com/ladzaretti/vlt-cli/pkg/genericclioptions"
	cmdutil "github.com/ladzaretti/vlt-cli/pkg/util"
	"github.com/ladzaretti/vlt-cli/pkg/util/input"
	"github.com/ladzaretti/vlt-cli/pkg/vaulterrors"
	"github.com/ladzaretti/vlt-cli/pkg/vlt"

	"github.com/spf13/cobra"
)

// LoginOptions holds data required to run the command.
type LoginOptions struct {
	vault func() *vlt.Vault

	*genericclioptions.StdioOptions
}

var _ genericclioptions.CmdOptions = &LoginOptions{}

// NewLoginOptions initializes the options struct.
func NewLoginOptions(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *LoginOptions {
	return &LoginOptions{
		StdioOptions: stdio,
		vault:        vault,
	}
}

// NewCmdLogin creates the cobra command.
func NewCmdLogin(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *cobra.Command {
	o := NewLoginOptions(stdio, vault)

	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate against the specified vault database",
		Long:  "This command authenticates the user and grants access to the vault for subsequent operations.",
		Run: func(_ *cobra.Command, _ []string) {
			cmdutil.CheckErr(genericclioptions.ExecuteCommand(context.Background(), o))
		},
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

func (o *LoginOptions) Run(ctx context.Context) error {
	v := o.vault()

	usrKey, err := input.PromptReadSecure(o.Out, int(o.In.Fd()), "Password for vault at %q:", v.Path)
	if err != nil {
		return fmt.Errorf("prompt password: %v", err)
	}

	dbKey, err := v.GetMasterKey(ctx)
	if err != nil {
		return fmt.Errorf("get master key: %v", err)
	}

	if subtle.ConstantTimeCompare([]byte(usrKey), []byte(dbKey)) == 0 {
		return vaulterrors.ErrWrongPassword
	}

	o.Infof("Login successful")

	return nil
}
