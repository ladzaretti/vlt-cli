package find

import (
	"context"

	"github.com/ladzaretti/vlt-cli/pkg/genericclioptions"
	"github.com/ladzaretti/vlt-cli/pkg/vlt"
	"github.com/ladzaretti/vlt-cli/pkg/vlt/store"

	"github.com/spf13/cobra"
)

// FindOptions holds data required to run the command.
type FindOptions struct {
	vault func() *vlt.Vault
	*genericclioptions.StdioOptions
	search *genericclioptions.SearchOptions
}

var _ genericclioptions.CmdOptions = &FindOptions{}

// NewFindOptions initializes the options struct.
func NewFindOptions(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *FindOptions {
	return &FindOptions{
		StdioOptions: stdio,
		vault:        vault,
		search:       &genericclioptions.SearchOptions{},
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

You can search by secret ID, name, or one or more labels.
Multiple label filters are matched using a logical OR.
Name and label values support UNIX glob patterns (e.g., "foo*", "*bar*").`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return genericclioptions.ExecuteCommand(cmd.Context(), o)
		},
	}

	cmd.Flags().IntSliceVarP(&o.search.IDs, "id", "", nil, o.search.Usage(genericclioptions.ID))
	cmd.Flags().StringVarP(&o.search.Name, "name", "", "", o.search.Usage(genericclioptions.NAME))
	cmd.Flags().StringSliceVarP(&o.search.Labels, "label", "", nil, o.search.Usage(genericclioptions.LABELS))

	return cmd
}

func (*FindOptions) Complete() error {
	return nil
}

func (*FindOptions) Validate() error {
	return nil
}

func (o *FindOptions) Run(ctx context.Context) error {
	var (
		m   map[int]store.LabeledSecret
		err error
	)

	switch {
	case len(o.search.IDs) > 0:
		m, err = o.vault().SecretsByIDs(ctx, o.search.IDs)

	case len(o.search.Name) > 0 && len(o.search.Labels) > 0:
		m, err = o.vault().SecretsByLabelsAndName(ctx, o.search.Name, o.search.Labels...)

	case len(o.search.Name) > 0:
		m, err = o.vault().SecretsByName(ctx, o.search.Name)

	case len(o.search.Labels) > 0:
		m, err = o.vault().SecretsByLabels(ctx, o.search.Labels...)

	default:
		m, err = o.vault().SecretsWithLabels(ctx)
	}

	if err != nil {
		return err
	}

	o.Infof("%v", m)

	return nil
}
