package save // replace with your actual package name

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ladzaretti/vlt-cli/pkg/clipboard"
	"github.com/ladzaretti/vlt-cli/pkg/genericclioptions"
	cmdutil "github.com/ladzaretti/vlt-cli/pkg/util"
	"github.com/ladzaretti/vlt-cli/pkg/util/input"
	"github.com/ladzaretti/vlt-cli/pkg/util/randstring"
	"github.com/ladzaretti/vlt-cli/pkg/vaulterrors"

	"github.com/ladzaretti/vlt-cli/pkg/vlt"

	"github.com/spf13/cobra"
)

var (
	ErrNoSecretUpdated  = errors.New("no secret was updated")
	ErrNoSecretInserted = errors.New("no secret was inserted")
)

type SaveError struct {
	Err error
}

func (e *SaveError) Error() string { return "save: " + e.Err.Error() }

func (e *SaveError) Unwrap() error { return e.Err }

// SaveOptions holds data required to run the command.
type SaveOptions struct {
	vault func() *vlt.Vault

	*genericclioptions.StdioOptions

	key      string // key is the name of the secret to save in the vault.
	generate bool   // generate indicates whether to auto-generate a random secret.
	update   bool   // update determines whether to overwrite an existing secret.
	output   bool   // output controls whether to print the saved secret to stdout.
	copy     bool   // copy controls whether to copy the saved secret to the clipboard.
	paste    bool   // paste controls whether to read the secret to save from the clipboard.
}

var _ genericclioptions.CmdOptions = &SaveOptions{}

// NewSaveOptions initializes the options struct.
func NewSaveOptions(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *SaveOptions {
	return &SaveOptions{
		StdioOptions: stdio,
		vault:        vault,
	}
}

// NewCmdSave creates the cobra command.
func NewCmdSave(stdio *genericclioptions.StdioOptions, vault func() *vlt.Vault) *cobra.Command {
	o := NewSaveOptions(stdio, vault)

	cmd := &cobra.Command{
		Use:   "save",
		Short: "Save a new secret to the vault",
		Long: `Save a new key-value pair to the vault.

If the specified key already exists, the operation will fail unless the --update flag is set.
Use --update to overwrite the existing value for a given key.`,
		Run: func(_ *cobra.Command, _ []string) {
			cmdutil.CheckErr(genericclioptions.ExecuteCommand(o))
		},
	}

	cmd.Flags().BoolVarP(&o.generate, "generate", "g", false, "generate a random secret")
	cmd.Flags().BoolVarP(&o.Stdin, "input", "i", false, "read password from stdin (useful with pipes or file redirects)")
	cmd.Flags().BoolVarP(&o.update, "update", "u", false, "update the value of an existing key in the vault")
	cmd.Flags().BoolVarP(&o.output, "output", "o", false, "output the saved secret to stdout (use with caution, intended primarily for piping)")
	cmd.Flags().BoolVarP(&o.copy, "copy-clipboard", "c", false, "copy the saved secret to the clipboard")
	cmd.Flags().BoolVarP(&o.paste, "paste-clipboard", "p", false, "read the password from the clipboard")

	cmd.Flags().StringVarP(&o.key, "key", "", "", "The key to associate with the secret value (required)")
	_ = cmd.MarkFlagRequired("key")

	return cmd
}

func (*SaveOptions) Complete() error {
	return nil
}

func (o *SaveOptions) Validate() error {
	if len(o.key) == 0 {
		return &SaveError{vaulterrors.ErrEmptyKey}
	}

	if strings.HasPrefix(o.key, "-") {
		return fmt.Errorf("invalid --key value %q (must not start with '-')", o.key)
	}

	return o.validateInputSource()
}

func (o *SaveOptions) Run() (retErr error) {
	secret, err := o.secret()
	if err != nil {
		return err
	}

	defer func() {
		if retErr != nil {
			retErr = &SaveError{retErr}
			return
		}

		retErr = o.outputSecret(secret)
	}()

	if o.update {
		retErr = o.updateSecret(secret)
		return
	}

	retErr = o.insertNewSecret(secret)

	return
}

func (o *SaveOptions) secret() (string, error) {
	if o.generate {
		return randstring.New(20)
	}

	return o.readSecret()
}

func (o *SaveOptions) readSecret() (string, error) {
	if o.paste {
		o.Debugf("Reading secret from clipboard")
		return clipboard.Paste()
	}

	if o.Stdin {
		o.Debugf("Reading secret from stdin")

		pass, err := input.ReadTrim(o.In)
		if err != nil {
			return "", fmt.Errorf("read secret: %v", err)
		}

		return pass, nil
	}

	pass, err := input.ReadSecure(int(o.In.Fd()), "Enter secret for key %q: ", o.key)
	if err != nil {
		return "", fmt.Errorf("prompt new secret: %v", err)
	}

	return pass, nil
}

func (o *SaveOptions) updateSecret(s string) error {
	n, err := o.vault().UpsertSecret(o.key, s)
	if err != nil {
		return err
	}

	if n == 0 {
		return ErrNoSecretUpdated
	}

	return nil
}

func (o *SaveOptions) insertNewSecret(s string) error {
	n, err := o.vault().InsertNewSecret(o.key, s)
	if err != nil {
		return err
	}

	if n == 0 {
		return ErrNoSecretInserted
	}

	return nil
}

func (o *SaveOptions) outputSecret(s string) error {
	if o.output {
		o.Infof("%s\n", s)
		return nil
	}

	if o.copy {
		o.Debugf("Coping secret to clipboard")
		return clipboard.Copy(s)
	}

	return nil
}

func (o *SaveOptions) validateInputSource() error {
	used := 0
	if o.generate {
		used++
	}

	if o.Stdin {
		used++
	}

	if o.paste {
		used++
	}

	if used > 1 {
		return &SaveError{errors.New("only one of --generate, --input, or --clipboard-paste can be used at a time")}
	}

	return nil
}
