package cli

import (
	"context"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"

	"github.com/spf13/cobra"
)

type VacuumOptions struct {
	*genericclioptions.StdioOptions
	*VaultOptions
}

var _ genericclioptions.CmdOptions = &VacuumOptions{}

// NewVacuumOptions initializes the options struct.
func NewVacuumOptions(stdio *genericclioptions.StdioOptions, vaultOptions *VaultOptions) *VacuumOptions {
	return &VacuumOptions{
		StdioOptions: stdio,
		VaultOptions: vaultOptions,
	}
}

func (*VacuumOptions) Complete() error { return nil }

func (*VacuumOptions) Validate() error { return nil }

func (o *VacuumOptions) Run(ctx context.Context, _ ...string) error {
	o.Debugf("vacuuming vault\n")

	if err := o.vault.Vacuum(ctx); err != nil {
		return err
	}

	f := func() error {
		o.Debugf("vacuuming vault container\n")
		return o.vault.VacuumContainer(ctx)
	}

	o.vault.RegisterCleanup(f)

	return nil
}

// NewCmdVacuum creates the vacuum cobra command.
func NewCmdVacuum(defaults *DefaultVltOptions) *cobra.Command {
	o := NewVacuumOptions(
		defaults.StdioOptions,
		defaults.vaultOptions,
	)

	cmd := &cobra.Command{
		Use:   "vacuum",
		Short: "Reclaim unused space in the database",
		Long: `Reclaim unused space in the database.

This is typically unnecessary, as SQLite reuses space internally.  
However, after deleting large blobs, vacuuming can help shrink the database file.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	return cmd
}
