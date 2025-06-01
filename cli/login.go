package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/input"
	"github.com/ladzaretti/vlt-cli/vault"
	"github.com/ladzaretti/vlt-cli/vaultdaemon"
	"github.com/ladzaretti/vlt-cli/vaulterrors"

	"github.com/spf13/cobra"
)

// LoginOptions holds data required to run the command.
type LoginOptions struct {
	*genericclioptions.StdioOptions
	*VaultOptions

	config        *ResolvedConfig
	sessionClient *vaultdaemon.SessionClient
}

var _ genericclioptions.CmdOptions = &LoginOptions{}

// NewLoginOptions initializes the options struct.
func NewLoginOptions(stdio *genericclioptions.StdioOptions, vaultOptions *VaultOptions, config *ResolvedConfig) *LoginOptions {
	return &LoginOptions{
		StdioOptions: stdio,
		VaultOptions: vaultOptions,
		config:       config,
	}
}

func (o *LoginOptions) Complete() error {
	s, err := vaultdaemon.NewSessionClient()
	if err != nil {
		return err
	}

	o.sessionClient = s

	return nil
}

func (o *LoginOptions) Validate() error {
	exists, err := o.vaultExists()
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("%w: %s", vaulterrors.ErrVaultFileNotFound, o.path)
	}

	if o.StdinIsPiped {
		return vaulterrors.ErrNonInteractiveUnsupported
	}

	return nil
}

func (o *LoginOptions) Close() error {
	return o.sessionClient.Close()
}

func (o *LoginOptions) Run(ctx context.Context, _ ...string) error {
	defer func() { _ = o.Close() }()

	path := o.path

	password, err := input.PromptReadSecure(o.Out, int(o.In.Fd()), "[vlt] Password for %q:", path)
	if err != nil {
		return fmt.Errorf("prompt password: %v", err)
	}

	if len(password) == 0 {
		return vaulterrors.ErrEmptyPassword
	}

	key, nonce, err := vault.Login(ctx, path, password)
	if err != nil {
		return err
	}

	sessionDuration := time.Duration(o.config.SessionDuration)
	if err := o.sessionClient.Login(ctx, path, key, nonce, sessionDuration); err != nil {
		return err
	}

	o.Infof("login successful\n")

	if err := o.postLoginHook(ctx, o.StdioOptions); err != nil {
		return fmt.Errorf("post-login hook: %w", err)
	}

	return nil
}

// NewCmdLogin creates the login cobra command.
func NewCmdLogin(defaults *DefaultVltOptions) *cobra.Command {
	o := NewLoginOptions(
		defaults.StdioOptions,
		defaults.vaultOptions,
		defaults.configOptions.resolved,
	)

	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate the user",
		Long:  "Authenticate the user and grant access to the vault for subsequent operations.",
		Run: func(cmd *cobra.Command, _ []string) {
			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}
}
