package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vaulterrors"

	"github.com/spf13/cobra"
)

// RotateOptions have the data required to perform the create operation.
type RotateOptions struct {
	*genericclioptions.StdioOptions

	vaultOptions *VaultOptions
}

var _ genericclioptions.CmdOptions = &RotateOptions{}

// NewRotateOptions initializes the options struct.
func NewRotateOptions(stdio *genericclioptions.StdioOptions, vaultOptions *VaultOptions) *RotateOptions {
	return &RotateOptions{
		StdioOptions: stdio,
		vaultOptions: vaultOptions,
	}
}

func (o *RotateOptions) Complete() error {
	return o.vaultOptions.Complete()
}

func (o *RotateOptions) Validate() error {
	if _, err := os.Stat(o.vaultOptions.path); !errors.Is(err, fs.ErrNotExist) {
		return vaulterrors.ErrVaultFileExists
	}

	if o.StdinIsPiped {
		return vaulterrors.ErrNonInteractiveUnsupported
	}

	return nil
}

// TODO: impl.
func (*RotateOptions) Run(_ context.Context, _ ...string) error {
	return nil
}

// NewCmdRotate creates the create cobra command.
func NewCmdRotate(defaults *DefaultVltOptions) *cobra.Command {
	o := NewRotateOptions(defaults.StdioOptions, defaults.vaultOptions)

	return &cobra.Command{
		Use:   "rotate",
		Short: "Rotate the master password",
		Long: fmt.Sprintf(`Securely change the master password of a vault.

The vault will be re-encrypted using the new password.

If no --file path is provided, uses the default path (~/%s).`, defaultDatabaseFilename),
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}
}
