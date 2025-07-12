package cli

import (
	"bytes"
	"context"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"

	"github.com/spf13/cobra"
)

type FindError struct {
	Err error
}

func (e *FindError) Error() string { return "find: " + e.Err.Error() }

func (e *FindError) Unwrap() error { return e.Err }

// FindOptions holds data required to run the command.
type FindOptions struct {
	*genericclioptions.StdioOptions
	*VaultOptions

	config *ResolvedConfig
	search *SearchableOptions
}

var _ genericclioptions.CmdOptions = &FindOptions{}

// NewFindOptions initializes the options struct.
func NewFindOptions(stdio *genericclioptions.StdioOptions, vaultOptions *VaultOptions, config *ResolvedConfig) *FindOptions {
	return &FindOptions{
		StdioOptions: stdio,
		VaultOptions: vaultOptions,
		config:       config,
		search:       NewSearchableOptions(),
	}
}

func (o *FindOptions) Complete() error { return o.search.Complete() }

func (o *FindOptions) Validate() error { return o.search.Validate() }

func (o *FindOptions) Run(ctx context.Context, args ...string) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = &FindError{retErr}
			return
		}
	}()

	o.search.WildcardFrom(args)

	matchingSecrets, err := o.search.search(ctx, o.vault)
	if err != nil {
		return err
	}

	var buf bytes.Buffer

	printTable(&buf, matchingSecrets)

	_, err = buf.WriteTo(o.Out)

	return err
}

// NewCmdFind creates the find cobra command.
func NewCmdFind(defaults *DefaultVltOptions) *cobra.Command {
	o := NewFindOptions(
		defaults.StdioOptions,
		defaults.vaultOptions,
		defaults.configOptions.resolved,
	)

	cmd := &cobra.Command{
		Use:     "find [glob]",
		Args:    cobra.ArbitraryArgs,
		Aliases: []string{"list", "ls"},
		Short:   "Search for secrets",
		Long: `Search for secrets stored in the vault using various filters.

You may optionally provide a glob pattern to match against secret names or labels.

Filters can be applied using --id, --name, or --label.
Multiple --label flags can be applied and are logically ORed.

Search values support UNIX glob patterns (e.g., "foo*", "*bar*").`,
		Example: `  # Find secrets with names or labels containing "foo"
  vlt find "*foo*"

  # Find secrets matching multiple labels (AND logic)
  vlt find --label foo --label bar

  # List all secrets in the vault
  vlt find`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o, args...))
		},
	}

	cmd.Flags().IntSliceVarP(&o.search.IDs, "id", "", nil, FilterByID.Help())
	cmd.Flags().StringVarP(&o.search.Name, "name", "", "", FilterByName.Help())
	cmd.Flags().StringSliceVarP(&o.search.Labels, "label", "", nil, FilterByLabels.Help())

	return cmd
}
