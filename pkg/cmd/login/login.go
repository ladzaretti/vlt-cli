package login // replace with your actual package name

import (
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

	genericclioptions.StdioOptions
}

var _ genericclioptions.CmdOptions = &LoginOptions{}

// NewLoginOptions initializes the options struct.
func NewLoginOptions(iostreams genericclioptions.IOStreams, vault func() *vlt.Vault) *LoginOptions {
	return &LoginOptions{
		StdioOptions: genericclioptions.StdioOptions{IOStreams: iostreams},
		vault:        vault,
	}
}

// NewCmdLogin creates the cobra command.
func NewCmdLogin(iostreams genericclioptions.IOStreams, vault func() *vlt.Vault) *cobra.Command {
	o := NewLoginOptions(iostreams, vault)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against the specified vault database",
		Long:  "This command authenticates the user and grants access to the vault for subsequent operations.",
		Run: func(_ *cobra.Command, _ []string) {
			cmdutil.CheckErr(genericclioptions.ExecuteCommand(o))
		},
	}

	cmd.Flags().BoolVarP(&o.Stdin, "input", "i", false,
		"read password from stdin (useful with pipes or file redirects)")

	return cmd
}

func (o *LoginOptions) Complete() error {
	return o.StdioOptions.Complete()
}

func (o *LoginOptions) Validate() error {
	return o.StdioOptions.Validate()
}

func (o *LoginOptions) Run() error {
	v := o.vault()

	usrKey, err := input.ReadSecure(int(o.In.Fd()), "Password for %s:", v.Path)
	if err != nil {
		return fmt.Errorf("prompt password: %v", err)
	}

	dbKey, err := v.GetMasterKey()
	if err != nil {
		return fmt.Errorf("get master key: %v", err)
	}

	if subtle.ConstantTimeCompare([]byte(usrKey), []byte(dbKey)) == 0 {
		return vaulterrors.ErrWrongPassword
	}

	o.Debugf("Login successful\n")

	return nil
}
