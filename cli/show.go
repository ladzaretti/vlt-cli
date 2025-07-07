package cli

import (
	"context"
	"errors"
	"os"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/clipboard"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vaulterrors"

	"github.com/spf13/cobra"
)

type ShowError struct {
	Err error
}

func (e *ShowError) Error() string { return "show: " + e.Err.Error() }

func (e *ShowError) Unwrap() error { return e.Err }

// ShowOptions holds data required to run the command.
type ShowOptions struct {
	*genericclioptions.StdioOptions
	*VaultOptions

	search *SearchableOptions
	stdout bool   // stdout controls whether to print the secret to stdout.
	copy   bool   // copy controls whether to copy the secret to the clipboard.
	output string // output controls whether to write secret to a given file.
}

var _ genericclioptions.CmdOptions = &ShowOptions{}

// NewShowOptions initializes the options struct.
func NewShowOptions(stdio *genericclioptions.StdioOptions, vaultOptions *VaultOptions) *ShowOptions {
	return &ShowOptions{
		StdioOptions: stdio,
		VaultOptions: vaultOptions,
		search:       NewSearchableOptions(),
	}
}

func (o *ShowOptions) Complete() error {
	return o.search.Complete()
}

func (o *ShowOptions) Validate() error {
	if err := o.search.Validate(); err != nil {
		return &ShowError{err}
	}

	if err := o.validateConfigOptions(); err != nil {
		return err
	}

	return o.search.Validate()
}

func (o *ShowOptions) validateConfigOptions() error {
	c := 0

	if o.copy {
		c++
	}

	if o.stdout {
		c++
	}

	if len(o.output) > 0 {
		c++
	}

	if c != 1 {
		return &ShowError{errors.New("exactly one of --stdout, --output, or --copy-clipboard must be set")}
	}

	return nil
}

// Run performs a secret lookup and outputs the result based on user flags.
func (o *ShowOptions) Run(ctx context.Context, args ...string) error {
	o.search.WildcardFrom(args)

	matchingSecrets, err := o.search.search(ctx, o.vault)
	if err != nil {
		return err
	}

	count := len(matchingSecrets)

	switch count {
	case 1:
		o.Debugf("found one match.\n")

		s, err := o.vault.ShowSecret(ctx, matchingSecrets[0].id)
		if err != nil {
			return err
		}

		return o.outputSecret(s)
	case 0:
		o.Errorf("no match found.\n")
		return &ShowError{vaulterrors.ErrSearchNoMatch}
	default:
		o.Errorf("expecting exactly one match, but found %d.\n\n", count)
		printTable(o.ErrOut, matchingSecrets)

		return &ShowError{vaulterrors.ErrAmbiguousSecretMatch}
	}
}

func (o *ShowOptions) outputSecret(s []byte) error {
	defer clear(s)

	if o.stdout {
		o.Printf("%s", s)
		return nil
	}

	if o.copy {
		o.Debugf("copying secret to clipboard\n")
		return clipboard.Copy(s)
	}

	if len(o.output) > 0 {
		f, err := os.Create(o.output)
		if err != nil {
			return err
		}
		defer func() { //nolint:wsl
			_ = f.Close()
		}()

		if _, err := f.Write(s); err != nil {
			return err
		}
	}

	return nil
}

// NewCmdShow creates the Show cobra command.
func NewCmdShow(defaults *DefaultVltOptions) *cobra.Command {
	o := NewShowOptions(
		defaults.StdioOptions,
		defaults.vaultOptions,
	)

	cmd := &cobra.Command{
		Use:     "show [glob]",
		Aliases: []string{"get"},
		Short:   "Retrieve a secret value",
		Long: `Retrieve and display a secret value from the vault.

The secret value will be displayed only if there is exactly one match for the given search criteria.

Search values support UNIX glob patterns (e.g., "foo*", "*bar*").

Use --stdout to print to stdout (unsafe), or --copy-clipboard to copy the value to the clipboard.`,
		Example: `  # Show a secret by matching its name or label, output to stdout (unsafe)
  vlt show foo --stdout

  # Show a secret by matching its ID, copy the value to the clipboard
  vlt show --id 42 --copy-clipboard

  # Show a secret by ID and write its value to a file
  vlt show --id 42 --output secret.file

  # Use glob pattern and label filter
  vlt show "*foo*" --label "*bar*" --stdout`,
		Run: func(cmd *cobra.Command, args []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o, args...))
		},
	}

	cmd.Flags().IntVarP(&o.search.ID, "id", "", 0, FilterByID.Help())
	cmd.Flags().StringVarP(&o.search.Name, "name", "", "", FilterByName.Help())
	cmd.Flags().StringSliceVarP(&o.search.Labels, "label", "", nil, FilterByLabels.Help())
	cmd.Flags().BoolVarP(&o.stdout, "stdout", "", false, "output the secret to stdout (unsafe)")
	cmd.Flags().BoolVarP(&o.copy, "copy-clipboard", "c", false, "copy the secret to the clipboard")
	cmd.Flags().StringVarP(&o.output, "output", "o", "", "export secrets to the specified file path")

	return cmd
}
