package cli

import (
	"context"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vault"

	"github.com/spf13/cobra"
)

// FindOptions holds data required to run the command.
type FindOptions struct {
	*genericclioptions.StdioOptions

	vault  func() *vault.Vault
	search *SearchableOptions
}

var _ genericclioptions.CmdOptions = &FindOptions{}

// NewFindOptions initializes the options struct.
func NewFindOptions(stdio *genericclioptions.StdioOptions, vault func() *vault.Vault) *FindOptions {
	return &FindOptions{
		StdioOptions: stdio,
		vault:        vault,
		search:       NewSearchableOptions(WithStrict(false)),
	}
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

// NewCmdFind creates the find cobra command.
func NewCmdFind(stdio *genericclioptions.StdioOptions, vault func() *vault.Vault) *cobra.Command {
	o := NewFindOptions(stdio, vault)

	cmd := &cobra.Command{
		Use:     "find",
		Aliases: []string{"list", "ls"},
		Short:   "Search for secrets in the vault",
		Long: `Search for secrets stored in the vault using various filters.

Filters can be applied using --id, --name, or --label.
Multiple --label flags can be applied and are logically ORed.

Name and label values support UNIX glob patterns (e.g., "foo*", "*bar*").`,
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	cmd.Flags().IntSliceVarP(&o.search.IDs, "id", "", nil, o.search.Usage(genericclioptions.ID))
	cmd.Flags().StringVarP(&o.search.Name, "name", "", "", o.search.Usage(genericclioptions.NAME))
	cmd.Flags().StringSliceVarP(&o.search.Labels, "label", "", nil, o.search.Usage(genericclioptions.LABELS))

	return cmd
}
