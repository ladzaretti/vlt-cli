package cli

import (
	"context"
	"errors"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/clipboard"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vaulterrors"
	"github.com/ladzaretti/vlt-cli/vlt"

	"github.com/spf13/cobra"
)

type ShowError struct {
	Err error
}

func (e *ShowError) Error() string { return "show: " + e.Err.Error() }

func (e *ShowError) Unwrap() error { return e.Err }

// ShowOptions holds data required to run the command.
type ShowOptions struct {
	vault func() *vlt.Vault

	*genericclioptions.StdioOptions
	search *SearchableOptions

	output bool // output controls whether to print the secret to stdout.
	copy   bool // copy controls whether to copy the secret to the clipboard.
}

var _ genericclioptions.CmdOptions = &ShowOptions{}

// NewShowOptions initializes the options struct.
func NewShowOptions(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *ShowOptions {
	return &ShowOptions{
		StdioOptions: stdio,
		vault:        vault,
		search:       &SearchableOptions{&genericclioptions.SearchOptions{}},
	}
}

// NewCmdShow creates the Show cobra command.
func NewCmdShow(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *cobra.Command {
	o := NewShowOptions(stdio, vault)

	cmd := &cobra.Command{
		Use:     "show",
		Aliases: []string{"get"},
		Short: `Retrieve and display the value of a secret.

Search using --id, --name, or --label. Exactly one secret must match.

Use --output to print to stdout, or --copy-clipboard to copy the value to the clipboard.`,
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	cmd.Flags().IntSliceVarP(&o.search.IDs, "id", "", nil, o.search.Usage(genericclioptions.ID))
	cmd.Flags().StringVarP(&o.search.Name, "name", "", "", o.search.Usage(genericclioptions.NAME))
	cmd.Flags().StringSliceVarP(&o.search.Labels, "label", "", nil, o.search.Usage(genericclioptions.LABELS))
	cmd.Flags().BoolVarP(&o.output, "output", "o", false, "output the secret to stdout (use with caution; intended primarily for piping)")
	cmd.Flags().BoolVarP(&o.copy, "copy-clipboard", "c", false, "copy the secret to clipboard")

	return cmd
}

func (o *ShowOptions) Complete() error {
	return o.search.Complete()
}

func (o *ShowOptions) Validate() error {
	c := 0

	if o.copy {
		c++
	}

	if o.output {
		c++
	}

	if c != 1 {
		return &ShowError{errors.New("either --output or --copy-clipboard must be set (but not both)")}
	}

	return o.search.Validate()
}

// Run performs a secret lookup and outputs the result based on user flags.
func (o *ShowOptions) Run(ctx context.Context) error {
	matchingSecrets, err := o.search.search(ctx, o.vault())
	if err != nil {
		return err
	}

	count := len(matchingSecrets)

	switch count {
	case 1:
		o.Debugf("1 secret matches the search settings.\n")

		s, err := o.vault().Secret(ctx, matchingSecrets[0].id)
		if err != nil {
			return err
		}

		return o.outputSecret(s)
	case 0:
		o.Warnf("No secrets match the search settings.\n")
		return &ShowError{vaulterrors.ErrSearchNoMatch}
	default:
		o.Warnf("Expected exactly one match, but found %d.\n\n", count)
		printTable(o.ErrOut, matchingSecrets)

		return &ShowError{vaulterrors.ErrAmbiguousSecretMatch}
	}
}

func (o *ShowOptions) outputSecret(s string) error {
	if o.output {
		o.Infof("%s\n", s)
		return nil
	}

	if o.copy {
		o.Debugf("Copying secret to clipboard\n")
		return clipboard.Copy(s)
	}

	return nil
}
