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
		search:       NewSearchableOptions(),
	}
}

func (o *FindOptions) Complete() error {
	return o.search.Complete()
}

func (o *FindOptions) Validate() error {
	return o.search.Validate()
}

func (o *FindOptions) Run(ctx context.Context, args ...string) error {
	o.search.WildcardFrom(args)

	matchingSecrets, err := o.search.search(ctx, o.vault())
	if err != nil {
		return err
	}

	printTable(o.Out, matchingSecrets)

	return nil
}

// NewCmdFind creates the find cobra command.
func NewCmdFind(vltOpts *DefaultVltOptions) *cobra.Command {
	o := NewFindOptions(vltOpts.StdioOptions, vltOpts.vaultOptions.Vault)

	cmd := &cobra.Command{
		Use:     "find [glob]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"list", "ls"},
		Short:   "Search for secrets in the vault",
		Long: `Search for secrets stored in the vault using various filters.

You may optionally provide a glob pattern to match against secret names or labels.

Filters can be applied using --id, --name, or --label.
Multiple --label flags can be applied and are logically ORed.

Name and label values support UNIX glob patterns (e.g., "foo*", "*bar*").`,
		Run: func(cmd *cobra.Command, args []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o, args...))
		},
	}

	cmd.Flags().IntSliceVarP(&o.search.IDs, "id", "", nil, FilterByID.Help())
	cmd.Flags().StringVarP(&o.search.Name, "name", "", "", FilterByName.Help())
	cmd.Flags().StringSliceVarP(&o.search.Labels, "label", "", nil, FilterByLabels.Help())

	return cmd
}
