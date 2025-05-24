package cli

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vault"

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

	vault  func() *vault.Vault
	config func() *ResolvedConfig
	search *SearchableOptions

	pipe    bool
	pipeCmd string
}

var _ genericclioptions.CmdOptions = &FindOptions{}

// NewFindOptions initializes the options struct.
func NewFindOptions(stdio *genericclioptions.StdioOptions, vault func() *vault.Vault, config func() *ResolvedConfig) *FindOptions {
	return &FindOptions{
		StdioOptions: stdio,
		vault:        vault,
		config:       config,
		search:       NewSearchableOptions(),
	}
}

func (o *FindOptions) Complete() error {
	return o.search.Complete()
}

func (o *FindOptions) Validate() error {
	if err := o.search.Validate(); err != nil {
		return err
	}

	if len(o.pipeCmd) > 0 {
		o.pipe = true
	}

	if len(o.pipeCmd) == 0 && o.pipe && len(o.config().PipeFindCmd) == 0 {
		return errors.New("cannot use --pipe: 'pipe_find_cmd' is not configured")
	}

	return nil
}

func (o *FindOptions) Run(ctx context.Context, args ...string) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = &FindError{retErr}
			return
		}
	}()

	o.search.WildcardFrom(args)

	matchingSecrets, err := o.search.search(ctx, o.vault())
	if err != nil {
		return err
	}

	var buf bytes.Buffer

	printTable(&buf, matchingSecrets)

	if o.pipe {
		cmd := cmp.Or(o.pipeCmd, o.config().PipeFindCmd)
		return o.runWithPipe(ctx, cmd, buf.String())
	}

	_, err = buf.WriteTo(o.Out)

	return err
}

func (o *FindOptions) runWithPipe(ctx context.Context, pipeCmd string, input string) error {
	if strings.TrimSpace(pipeCmd) == "" {
		return errors.New("no pipeline command provided")
	}

	shell := o.config().Shell

	//nolint:gosec //G204 - intentional use of shell for user-configured pipeline
	cmd := exec.CommandContext(ctx, shell, "-c", pipeCmd)

	cmd.Stdin = strings.NewReader(input)
	cmd.Stdout = o.Out
	cmd.Stderr = o.ErrOut

	return cmd.Run()
}

// NewCmdFind creates the find cobra command.
func NewCmdFind(vltOpts *DefaultVltOptions) *cobra.Command {
	o := NewFindOptions(
		vltOpts.StdioOptions,
		vltOpts.vaultOptions.Vault,
		vltOpts.configOptions.Resolved,
	)

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
	cmd.Flags().BoolVarP(&o.pipe, "pipe", "p", false, "pipe output using 'pipe_find_cmd' if configured")
	cmd.Flags().StringVarP(
		&o.pipeCmd,
		"pipe-cmd", "P",
		"",
		"override 'pipe_find_cmd' with a custom pipeline command",
	)

	return cmd
}
