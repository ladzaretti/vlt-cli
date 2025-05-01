package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/input"
	"github.com/ladzaretti/vlt-cli/vaulterrors"
	"github.com/ladzaretti/vlt-cli/vlt"

	"github.com/spf13/cobra"
)

type RemoveError struct {
	Err error
}

func (e *RemoveError) Error() string { return "remove: " + e.Err.Error() }

func (e *RemoveError) Unwrap() error { return e.Err }

// RemoveOptions holds data required to run the command.
type RemoveOptions struct {
	*genericclioptions.StdioOptions

	vault     func() *vlt.Vault
	search    *SearchableOptions
	assumeYes bool
	removeAll bool
}

var _ genericclioptions.CmdOptions = &RemoveOptions{}

// NewRemoveOptions initializes the options struct.
func NewRemoveOptions(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *RemoveOptions {
	return &RemoveOptions{
		StdioOptions: stdio,
		vault:        vault,
		search:       NewSearchableOptions(),
	}
}

func (o *RemoveOptions) Complete() error {
	return o.search.Complete()
}

func (o *RemoveOptions) Validate() error {
	if o.search.IsUnset() {
		return &RemoveError{genericclioptions.ErrNoSearchParams}
	}

	return o.search.Validate()
}

func (o *RemoveOptions) Run(ctx context.Context) error {
	matchingSecrets, err := o.search.search(ctx, o.vault())
	if err != nil {
		return err
	}

	count := len(matchingSecrets)

	if count > 0 {
		printTable(o.Out, matchingSecrets)
	}

	switch count {
	case 1:
		o.Debugf("Found one match.\n")
	case 0:
		o.Warnf("No match found.\n")
		return &RemoveError{vaulterrors.ErrSearchNoMatch}
	default:
		o.Warnf("Found %d matching secrets.\n", count)

		if !o.removeAll {
			return &RemoveError{fmt.Errorf("%d matching secrets found, use --all to delete all", count)}
		}
	}

	if !o.assumeYes {
		yes, err := confirm(o.Out, o.In, "\nDelete %d secrets? (y/N): ", count)
		if err != nil {
			return err
		}

		if !yes {
			return nil
		}

		o.Debugf("Deletion confirmed by the user.\n")
	}

	o.Debugf("Proceeding with deleting secrets.\n")

	n, err := o.vault().DeleteByIDs(ctx, extractIDs(matchingSecrets)...)
	if err != nil {
		return err
	}

	o.Debugf("Deleted %d secrets.\n", n)
	o.Infof("\nOK\n")

	return nil
}

func confirm(out io.Writer, in io.Reader, prompt string, a ...any) (bool, error) {
	response, err := input.PromptRead(out, in, prompt, a...)
	if err != nil {
		return false, err
	}

	normalized := strings.ToLower(strings.TrimSpace(response))

	return slices.Contains([]string{"y", "yes"}, normalized), nil
}

// NewCmdRemove creates the remove cobra command.
func NewCmdRemove(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *cobra.Command {
	o := NewRemoveOptions(stdio, vault)

	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove secrets from the vault",
		Long: `Remove one or more secrets from the vault.

Secrets can be matched for removal using filters such as ID, name, or label.
Use the --yes flag to skip confirmation prompts.`,
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	cmd.Flags().IntSliceVarP(&o.search.IDs, "id", "", nil, o.search.Usage(genericclioptions.ID))
	cmd.Flags().StringVarP(&o.search.Name, "name", "", "", o.search.Usage(genericclioptions.NAME))
	cmd.Flags().StringSliceVarP(&o.search.Labels, "label", "", nil, o.search.Usage(genericclioptions.LABELS))
	cmd.Flags().BoolVarP(&o.assumeYes, "yes", "y", false, "automatically answer yes to all questions")
	cmd.Flags().BoolVar(&o.removeAll, "all", false, "remove all matching secrets")

	return cmd
}
