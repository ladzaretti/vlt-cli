package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/input"
	"github.com/ladzaretti/vlt-cli/vault"
	"github.com/ladzaretti/vlt-cli/vaulterrors"

	"github.com/spf13/cobra"
)

const (
	masterKeyMinLen = 8
)

// CreateOptions have the data required to perform the create operation.
type CreateOptions struct {
	*genericclioptions.StdioOptions

	vaultOptions *VaultOptions
}

var _ genericclioptions.CmdOptions = &CreateOptions{}

// NewCreateOptions initializes the options struct.
func NewCreateOptions(stdio *genericclioptions.StdioOptions, vaultOptions *VaultOptions) *CreateOptions {
	return &CreateOptions{
		StdioOptions: stdio,
		vaultOptions: vaultOptions,
	}
}

func (o *CreateOptions) Complete() error {
	return o.vaultOptions.Complete()
}

func (o *CreateOptions) Validate() error {
	if _, err := os.Stat(o.vaultOptions.Path); !errors.Is(err, fs.ErrNotExist) {
		return vaulterrors.ErrVaultFileExists
	}

	if o.NonInteractive {
		return vaulterrors.ErrNonInteractiveUnsupported
	}

	return nil
}

func (o *CreateOptions) Run(ctx context.Context) error {
	mk, err := input.PromptNewPassword(o.Out, int(o.In.Fd()), masterKeyMinLen)
	if err != nil {
		return fmt.Errorf("read new master key: %w", err)
	}

	_, err = vault.New(ctx, mk, o.vaultOptions.Path)
	if err != nil {
		return fmt.Errorf("create vault: %w", err)
	}

	o.Infof("New vault successfully created at %q\n", o.vaultOptions.Path)

	return nil
}

// NewCmdCreate creates the create cobra command.
func NewCmdCreate(stdio *genericclioptions.StdioOptions, vaultOptions *VaultOptions) *cobra.Command {
	o := NewCreateOptions(stdio, vaultOptions)

	return &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Initialize a new vault",
		Long: fmt.Sprintf(`Create a new vault at the specified path. 

If no --file path is provided, uses the default path (~/%s).`, defaultDatabaseFilename),
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}
}
