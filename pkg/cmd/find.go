package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/ladzaretti/vlt-cli/pkg/genericclioptions"
	"github.com/ladzaretti/vlt-cli/pkg/vlt"

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

func (o *FindOptions) Complete() error {
	return o.search.Complete()
}

func (o *FindOptions) Validate() error {
	return o.search.Validate()
}

func (o *FindOptions) Run(ctx context.Context) error {
	labeledSecrets, err := o.search.search(ctx, o.vault())
	if err != nil {
		return err
	}

	printTable(o.Out, labeledSecrets)

	return nil
}

func printTable(w io.Writer, labeledSecrets []labeledSecretPair) {
	tw := tabwriter.NewWriter(w, 0, 0, 5, ' ', 0)
	defer func() { _ = tw.Flush() }()

	fmt.Fprintln(tw, "ID\tNAME\tLABELS")

	for _, ls := range labeledSecrets {
		fmt.Fprintf(tw, "%d\t%s\t%s\n", ls.id, ls.secret.Name, strings.Join(ls.secret.Labels, ","))
	}
}
