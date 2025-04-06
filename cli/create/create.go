package create

import (
	"fmt"

	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/input"
	"github.com/ladzaretti/vlt-cli/vlt"

	"github.com/spf13/cobra"
)

// CreateOptions have the data required to perform the create operation.
type CreateOptions struct {
	vault *vlt.Vault

	genericclioptions.StdioOptions
}

var _ genericclioptions.CmdOptions = &CreateOptions{}

// NewCreateOptions initializes the options struct.
func NewCreateOptions(iostreams genericclioptions.IOStreams, vault *vlt.Vault) *CreateOptions {
	return &CreateOptions{
		StdioOptions: genericclioptions.StdioOptions{IOStreams: iostreams},
		vault:        vault,
	}
}

// NewCmdCreate creates a new create command.
func NewCmdCreate(iostreams genericclioptions.IOStreams, vault *vlt.Vault) *cobra.Command {
	o := NewCreateOptions(iostreams, vault)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Initialize a new vault",
		Long:  "Create a new vault by specifying the SQLite database file where credentials will be stored.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return genericclioptions.ExecuteCommand(o)
		},
	}

	cmd.Flags().BoolVarP(&o.Stdin, "input", "i", false,
		"read password from stdin (useful with pipes or file redirects)")

	return cmd
}

func (o *CreateOptions) Complete() error {
	return o.StdioOptions.Complete()
}

func (o *CreateOptions) Validate() error {
	return o.StdioOptions.Validate()
}

func (o *CreateOptions) Run() error {
	mk, err := o.readNewMasterKey()
	if err != nil {
		o.Debugf("Could not read user password: %v\n", err)
		return fmt.Errorf("read user password: %v", err)
	}

	if err := o.vault.SetMasterKey(mk); err != nil {
		o.Debugf("Failure setting vault master key: %v\n", err)
		return fmt.Errorf("set master key: %v", err)
	}

	return nil
}

func (o *CreateOptions) readNewMasterKey() (string, error) {
	if o.Stdin {
		o.Debugf("Reading password from stdin")

		pass, err := input.ReadTrim(o.In)
		if err != nil {
			return "", fmt.Errorf("read password: %v", err)
		}

		return pass, nil
	}

	pass, err := input.PromptNewPassword(int(o.In.Fd()))
	if err != nil {
		return "", fmt.Errorf("prompt new password: %v", err)
	}

	return pass, nil
}
