package login // replace with your actual package name

import (
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vlt"

	"github.com/spf13/cobra"
)

// LoginOptions holds data required to run the command.
type LoginOptions struct {
	vault *vlt.Vault

	genericclioptions.StdioOptions
}

var _ genericclioptions.CmdOptions = &LoginOptions{}

// NewLoginOptions initializes the options struct.
func NewLoginOptions(iostreams genericclioptions.IOStreams, vault *vlt.Vault) *LoginOptions {
	return &LoginOptions{
		StdioOptions: genericclioptions.StdioOptions{IOStreams: iostreams},
		vault:        vault,
	}
}

// NewCmdLogin creates the cobra command.
func NewCmdLogin(iostreams genericclioptions.IOStreams, vault *vlt.Vault) *cobra.Command {
	o := NewLoginOptions(iostreams, vault)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against the specified vault database",
		Long:  "This command authenticates the user and grants access to the vault for subsequent operations.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return genericclioptions.ExecuteCommand(o)
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

func (*LoginOptions) Run() error {
	// read current password
	// compare to database
	return nil
}
