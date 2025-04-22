package cli

import (
	"context"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vlt"

	"github.com/spf13/cobra"
)

// FindOptions holds data required to run the command.
type FindOptions struct {
	vault func() *vlt.Vault

	*genericclioptions.StdioOptions
	search *SearchableOptions
}

var _ genericclioptions.CmdOptions = &FindOptions{}

// NewFindOptions initializes the options struct.
func NewFindOptions(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *FindOptions {
	return &FindOptions{
		StdioOptions: stdio,
		vault:        vault,
		search:       &SearchableOptions{&genericclioptions.SearchOptions{}},
	}
}

// NewCmdFind creates the find cobra command.
func NewCmdFind(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *cobra.Command {
	o := NewFindOptions(stdio, vault)

	cmd := &cobra.Command{
		Use:     "find",
		Aliases: []string{"list", "ls"},
		Short:   "Find secrets by ID, name, or labels",
		Long: `Find secrets stored in the vault using various filters.

Supports filtering by secret ID, name, or one or more labels.
Multiple label filters are matched using a logical OR.
Name and label values accept UNIX glob patterns (e.g., "foo*", "*bar*").`,
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	cmd.Flags().IntSliceVarP(&o.search.IDs, "id", "", nil, o.search.Usage(genericclioptions.ID))
	cmd.Flags().StringVarP(&o.search.Name, "name", "", "", o.search.Usage(genericclioptions.NAME))
	cmd.Flags().StringSliceVarP(&o.search.Labels, "label", "", nil, o.search.Usage(genericclioptions.LABELS))

	return cmd
}

func (o *FindOptions) Complete() error {
	return o.search.Complete()
}

func (o *FindOptions) Validate() error {
	return o.search.Validate()
}

func (o *FindOptions) Run(ctx context.Context) error {
	matchingSecrets, err := o.search.search(ctx, o.vault())
	if err != nil {
		return err
	}

	printTable(o.Out, matchingSecrets)

	return nil
}
