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
	*VaultOptions

	search    *SearchableOptions
	assumeYes bool
	removeAll bool
}

var _ genericclioptions.CmdOptions = &RemoveOptions{}

// NewRemoveOptions initializes the options struct.
func NewRemoveOptions(stdio *genericclioptions.StdioOptions, vaultOptions *VaultOptions) *RemoveOptions {
	return &RemoveOptions{
		StdioOptions: stdio,
		VaultOptions: vaultOptions,
		search:       NewSearchableOptions(),
	}
}

func (o *RemoveOptions) Complete() error {
	return o.search.Complete()
}

func (o *RemoveOptions) Validate() error {
	return o.search.Validate()
}

func (o *RemoveOptions) Run(ctx context.Context, args ...string) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = &RemoveError{retErr}
			return
		}
	}()

	o.search.WildcardFrom(args)

	matchingSecrets, err := o.search.search(ctx, o.vault)
	if err != nil {
		return err
	}

	count := len(matchingSecrets)

	if count > 0 && !o.assumeYes {
		printTable(o.Out, matchingSecrets)
	}

	switch count {
	case 1:
		o.Debugf("found one match.\n")
	case 0:
		o.Errorf("no match found.\n")
		return vaulterrors.ErrSearchNoMatch
	default:
		o.Errorf("found %d matching secrets.\n", count)

		if !o.removeAll {
			return fmt.Errorf("%d matching secrets found, use --all to delete all", count)
		}
	}

	if !o.assumeYes {
		yes, err := confirm(o.Out, o.In, "Delete %d secrets? (y/N): ", count)
		if err != nil {
			return err
		}

		if !yes {
			return nil
		}

		o.Debugf("deletion confirmed by the user.\n")
	}

	o.Debugf("proceeding with deleting secrets.\n")

	n, err := o.vault.DeleteSecretsByIDs(ctx, extractIDs(matchingSecrets)...)
	if err != nil {
		return err
	}

	o.Infof("successfully deleted %d secrets.\n", n)

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
func NewCmdRemove(defaults *DefaultVltOptions) *cobra.Command {
	o := NewRemoveOptions(
		defaults.StdioOptions,
		defaults.vaultOptions,
	)

	cmd := &cobra.Command{
		Use:     "remove [glob]",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove secrets",
		Long: `Remove one or more secrets from the vault.

You may optionally provide a glob pattern to match against secret names or labels.

Use --id, --name, or --label to select which secrets to remove.
Multiple --label flags can be applied and are logically ORed.

Search values support UNIX glob patterns (e.g., "foo*", "*bar*").
`,
		Example: `  # Remove a secret by ID
  vlt remove --id 42

  # Remove all secrets whose name or label matches the given glob pattern
  vlt remove "*foo*" --all

  # Remove all secrets matching any of the given labels
  vlt remove --label foo --label bar --all

  # Remove a secret by name without confirmation
  vlt remove --name api-key --yes`,
		Run: func(cmd *cobra.Command, args []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o, args...))
		},
	}

	cmd.Flags().IntSliceVarP(&o.search.IDs, "id", "", nil, FilterByID.Help())
	cmd.Flags().StringVarP(&o.search.Name, "name", "", "", FilterByName.Help())
	cmd.Flags().StringSliceVarP(&o.search.Labels, "label", "", nil, FilterByName.Help())
	cmd.Flags().BoolVarP(&o.assumeYes, "yes", "y", false, "skip confirmation prompts")
	cmd.Flags().BoolVar(&o.removeAll, "all", false, "remove all matching secrets")

	return cmd
}
