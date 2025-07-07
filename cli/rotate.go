package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/input"
	"github.com/ladzaretti/vlt-cli/vault"
	"github.com/ladzaretti/vlt-cli/vaulterrors"

	"github.com/spf13/cobra"
)

type RotateError struct {
	Err error
}

func (e *RotateError) Error() string { return "rotate: " + e.Err.Error() }

func (e *RotateError) Unwrap() error { return e.Err }

// RotateOptions have the data required to perform the create operation.
type RotateOptions struct {
	*genericclioptions.StdioOptions

	vaultOptions *VaultOptions
}

var _ genericclioptions.CmdOptions = &RotateOptions{}

// NewRotateOptions initializes the options struct.
func NewRotateOptions(stdio *genericclioptions.StdioOptions, vaultOptions *VaultOptions) *RotateOptions {
	return &RotateOptions{
		StdioOptions: stdio,
		vaultOptions: vaultOptions,
	}
}

func (o *RotateOptions) Complete() error {
	return o.vaultOptions.Complete()
}

func (o *RotateOptions) Validate() error {
	if o.StdinIsPiped {
		return vaulterrors.ErrNonInteractiveUnsupported
	}

	return nil
}

func (o *RotateOptions) Run(ctx context.Context, _ ...string) (retErr error) {
	defer func() {
		if retErr != nil {
			retErr = &RotateError{retErr}
			return
		}
	}()

	srcVault, err := o.openSrcVault(ctx)
	if err != nil {
		return err
	}

	secrets, err := srcVault.ExportSecrets(ctx)
	if err != nil {
		return err
	}

	err = srcVault.Close()
	if err != nil {
		return err
	}

	dir, err := os.MkdirTemp(filepath.Dir(srcVault.Path), "vlt_rotate_")
	if err != nil {
		return err
	}

	o.Debugf("created temporary rotation directory: %s", dir)

	defer func() {
		o.Debugf("removing temporary directory: %s", dir)

		if err := os.RemoveAll(dir); err != nil {
			o.Errorf("failure removing dir: %v", err)
		}
	}()

	destVault, err := o.openDestVault(ctx, filepath.Join(dir, ".vlt.tmp"))
	if err != nil {
		return err
	}
	defer func() { //nolint:wsl
		retErr = errors.Join(retErr, destVault.Close())
	}()

	i := 0
	for id, s := range secrets {
		_, err := destVault.InsertNewSecret(ctx, s.Name, s.Value, s.Labels, vault.InsertWithID(id))
		if err != nil {
			return err
		}

		i++
	}

	o.Debugf("number of secrets rotated: %d", i)

	if err := destVault.Seal(ctx); err != nil {
		return fmt.Errorf("create: %w", err)
	}

	if err := destVault.Close(); err != nil {
		return err
	}

	o.Debugf("rotating vault: from %q to %q", srcVault.Path, destVault.Path)

	if err := os.Rename(destVault.Path, srcVault.Path); err != nil {
		return err
	}

	o.Infof("vault rotated successfully\n")

	return nil
}

func (o *RotateOptions) openSrcVault(ctx context.Context) (*vault.Vault, error) {
	path := o.vaultOptions.path

	password, err := input.PromptReadSecure(o.Out, int(o.In.Fd()), "[vlt] Password for %q:", path)
	if err != nil {
		return nil, fmt.Errorf("prompt password: %v", err)
	}
	defer clear(password)

	if len(password) == 0 {
		return nil, vaulterrors.ErrEmptyPassword
	}

	key, nonce, err := vault.Login(ctx, path, password)
	if err != nil {
		return nil, err
	}

	if err := o.vaultOptions.postLoginHook(ctx, o.StdioOptions); err != nil {
		return nil, err
	}

	return vault.Open(ctx, path, vault.WithSessionKey(key, nonce))
}

func (o *RotateOptions) openDestVault(ctx context.Context, path string) (*vault.Vault, error) {
	password, err := input.PromptNewPassword(o.Out, int(o.In.Fd()), masterPasswordMinLen)
	if err != nil {
		return nil, fmt.Errorf("create: %w", err)
	}
	defer clear(password)

	return vault.New(ctx, path, password, vault.WithMaxHistorySnapshots(o.vaultOptions.maxHistorySnapshots))
}

// NewCmdRotate creates the create cobra command.
func NewCmdRotate(defaults *DefaultVltOptions) *cobra.Command {
	hiddenFlags := []string{"no-login-prompt"}
	o := NewRotateOptions(defaults.StdioOptions, defaults.vaultOptions)

	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "Rotate the master password",
		Long: fmt.Sprintf(`Securely change the master password of a vault.

The vault will be re-encrypted using the new password.

If no --file path is provided, uses the default path (~/%s).`, defaultDatabaseFilename),
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.RejectDisallowedFlags(cmd, hiddenFlags...))
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	genericclioptions.MarkFlagsHidden(cmd, hiddenFlags...)

	return cmd
}
