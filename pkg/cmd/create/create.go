package create

import (
	"fmt"

	"github.com/ladzaretti/vlt-cli/pkg/genericclioptions"
	cmdutil "github.com/ladzaretti/vlt-cli/pkg/util"
	"github.com/ladzaretti/vlt-cli/pkg/util/input"
	"github.com/ladzaretti/vlt-cli/pkg/vlt"

	"github.com/spf13/cobra"
)

// CreateOptions have the data required to perform the create operation.
type CreateOptions struct {
	vaultPath func() string

	*genericclioptions.StdioOptions
}

var _ genericclioptions.CmdOptions = &CreateOptions{}

// NewCreateOptions initializes the options struct.
func NewCreateOptions(stdio *genericclioptions.StdioOptions, vaultPath func() string) *CreateOptions {
	return &CreateOptions{
		StdioOptions: stdio,
		vaultPath:    vaultPath,
	}
}

// NewCmdCreate creates a new create command.
func NewCmdCreate(stdio *genericclioptions.StdioOptions, path func() string) *cobra.Command {
	o := NewCreateOptions(stdio, path)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Initialize a new vault",
		Long:  "Create a new vault by specifying the SQLite database file where credentials will be stored.",
		Run: func(_ *cobra.Command, _ []string) {
			cmdutil.CheckErr(genericclioptions.ExecuteCommand(o))
		},
	}

	cmd.Flags().BoolVarP(&o.Stdin, "input", "i", false,
		"read password from stdin (useful with pipes or file redirects)")

	return cmd
}

func (*CreateOptions) Complete() error {
	return nil
}

func (*CreateOptions) Validate() error {
	return nil
}

func (o *CreateOptions) Run() error {
	mk, err := o.readNewMasterKey()
	if err != nil {
		o.Debugf("Could not read user password: %v\n", err)
		return fmt.Errorf("read user password: %v", err)
	}

	vault, err := vlt.New(o.vaultPath())
	if err != nil {
		return err
	}

	if err := vault.SetMasterKey(mk); err != nil {
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
