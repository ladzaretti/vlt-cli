package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/clipboard"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/input"
	"github.com/ladzaretti/vlt-cli/randstring"
	"github.com/ladzaretti/vlt-cli/vaulterrors"
	"github.com/ladzaretti/vlt-cli/vlt"

	"github.com/spf13/cobra"
)

var (
	ErrMissingID       = errors.New("no id provided; specify an id with the --id flag")
	ErrNoUpdateArgs    = errors.New("no update arguments provided; specify at least one of --name, --secret, --add-label, or --remove-label")
	ErrNoSecretUpdated = errors.New("no secret was updated")
)

type UpdateError struct {
	Err error
}

func (e *UpdateError) Error() string { return "update: " + e.Err.Error() }

func (e *UpdateError) Unwrap() error { return e.Err }

type UpdateOptions struct {
	*genericclioptions.StdioOptions

	vault func() *vlt.Vault

	id           int
	name         string
	addLabels    []string
	removeLabels []string
}

var _ genericclioptions.CmdOptions = &UpdateOptions{}

// NewUpdateOptions initializes the options struct.
func NewUpdateOptions(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *UpdateOptions {
	return &UpdateOptions{
		StdioOptions: stdio,
		vault:        vault,
	}
}

func (*UpdateOptions) Complete() error {
	return nil
}

func (o *UpdateOptions) Validate() error {
	if o.id == 0 {
		return &UpdateError{ErrMissingID}
	}

	return o.validateUpdateArgs()
}

func (o *UpdateOptions) validateUpdateArgs() error {
	args := 0

	if len(o.name) > 0 {
		args++
	}

	if len(o.addLabels) > 0 {
		args++
	}

	if len(o.removeLabels) > 0 {
		args++
	}

	if args == 0 {
		return &UpdateError{ErrNoUpdateArgs}
	}

	return nil
}

func (*UpdateOptions) Run(context.Context) error {
	// TODO2: update vault: requires a tx based vault methods

	return nil
}

// NewCmdUpdate creates the update cobra command.
func NewCmdUpdate(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *cobra.Command {
	o := NewUpdateOptions(stdio, vault)

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update secret metadata",
		Long: `Update metadata for an existing secret.

To update the secret value, use the 'vlt update secret' command.`,
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	cmd.Flags().IntVarP(&o.id, "id", "", 0, "id of the secret to update")
	cmd.Flags().StringVarP(&o.name, "name", "", "", "new name for the secret")
	cmd.Flags().StringSliceVarP(&o.addLabels, "add-label", "", nil, "label to add to the secret")
	cmd.Flags().StringSliceVarP(&o.removeLabels, "remove-label", "", nil, "label to remove from the secret")

	cmd.AddCommand(NewCmdUpdateSecretValue(stdio, vault))

	return cmd
}

type UpdateSecretValueOptions struct {
	*genericclioptions.StdioOptions
	vault func() *vlt.Vault

	id       int  // id of the secret to be updated.
	generate bool // generate indicates whether to auto-generate a random secret.
	output   bool // output controls whether to print the saved secret to stdout.
	copy     bool // copy controls whether to copy the saved secret to the clipboard.
	paste    bool // paste controls whether to read the secret to save from the clipboard.
}

var _ genericclioptions.CmdOptions = &UpdateSecretValueOptions{}

// NewUpdateSecretValueOptions initializes the options struct.
func NewUpdateSecretValueOptions(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *UpdateSecretValueOptions {
	return &UpdateSecretValueOptions{
		StdioOptions: stdio,
		vault:        vault,
	}
}

func (*UpdateSecretValueOptions) Complete() error {
	return nil
}

func (o *UpdateSecretValueOptions) Validate() error {
	if o.id == 0 {
		return &UpdateError{ErrMissingID}
	}

	return o.validateUpdateSecretArgs()
}

func (o *UpdateSecretValueOptions) validateUpdateSecretArgs() error {
	used := 0
	if o.NonInteractive {
		used++
	}

	if o.generate {
		used++
	}

	if o.paste {
		used++
	}

	if used > 1 {
		return &UpdateError{errors.New("only one of non-interactive input, --generate, or --paste can be used at a time")}
	}

	return nil
}

func (o *UpdateSecretValueOptions) Run(ctx context.Context) (retErr error) {
	secret := ""

	// ensure error is wrapped and output is printed if everything succeeded
	defer func() {
		if retErr != nil {
			retErr = &UpdateError{retErr}
			return
		}

		if len(secret) > 0 {
			if err := o.outputSecret(secret); err != nil {
				retErr = &UpdateError{err}
				return
			}
		}
	}()

	if err := secretExists(ctx, o.vault(), o.id); err != nil {
		return err
	}

	s, err := o.readSecretNonInteractive()
	if err != nil {
		return fmt.Errorf("read secret non-interactive: %w", err)
	}

	interactive := len(s) == 0
	secret = strings.TrimSpace(s)

	if interactive {
		s, err := o.promptReadSecure("Enter new secret value: ")
		if err != nil {
			return err
		}

		secret = s
	}

	if len(secret) == 0 {
		return vaulterrors.ErrEmptySecret
	}

	return o.UpdateSecretValue(ctx, o.id, secret)
}

func (o *UpdateSecretValueOptions) readSecretNonInteractive() (string, error) {
	if o.generate {
		return randstring.NewWithPolicy(randstring.DefaultPasswordPolicy)
	}

	if o.paste {
		o.Debugf("Reading secret from clipboard")
		return clipboard.Paste()
	}

	if o.NonInteractive {
		o.Debugf("Reading non-interactive secret")
		return input.ReadTrim(o.In)
	}

	return "", nil
}

func (o *UpdateSecretValueOptions) outputSecret(s string) error {
	if o.output {
		o.Infof("%s", s)
		return nil
	}

	if o.copy {
		o.Debugf("Copying secret to clipboard\n")
		return clipboard.Copy(s)
	}

	return nil
}

func (o *UpdateSecretValueOptions) promptReadSecure(prompt string, a ...any) (string, error) {
	return input.PromptReadSecure(o.Out, int(o.In.Fd()), prompt, a...)
}

func (o *UpdateSecretValueOptions) UpdateSecretValue(ctx context.Context, id int, secret string) error {
	n, err := o.vault().UpdateSecret(ctx, id, secret)
	if err != nil {
		return err
	}

	if n == 0 {
		return ErrNoSecretInserted
	}

	o.Infof("OK\n")

	return nil
}

func secretExists(ctx context.Context, vault *vlt.Vault, id int) error {
	ids, err := vault.SecretsByIDs(ctx, id)
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		return fmt.Errorf("no secret found with ID: %d", id)
	}

	return nil
}

// NewCmdUpdateSecretValue creates the cobra command.
func NewCmdUpdateSecretValue(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *cobra.Command {
	o := NewUpdateSecretValueOptions(stdio, vault)

	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Update a secret value",
		Long:  "Update a secret value.",
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	cmd.Flags().IntVarP(&o.id, "id", "", 0, "id of the secret to update")
	cmd.Flags().BoolVarP(&o.generate, "generate", "g", false, "generate a random secret")
	cmd.Flags().BoolVarP(&o.output, "output", "o", false, "output the saved secret to stdout (unsafe)")
	cmd.Flags().BoolVarP(&o.copy, "copy", "c", false, "copy the saved secret to the clipboard")
	cmd.Flags().BoolVarP(&o.paste, "paste", "p", false, "read the secret from the clipboard")

	return cmd
}
