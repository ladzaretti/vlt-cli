package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"time"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/clipboard"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/input"
	cmdutil "github.com/ladzaretti/vlt-cli/util"
	"github.com/ladzaretti/vlt-cli/vault"
	"github.com/ladzaretti/vlt-cli/vaultdaemon"
	"github.com/ladzaretti/vlt-cli/vaulterrors"

	"github.com/spf13/cobra"
)

var Version = "0.0.0"

const (
	// defaultDatabaseFilename is the default vault file name.
	defaultDatabaseFilename = ".vlt"

	// defaultConfigName is the default configuration file name.
	defaultConfigName = ".vlt.toml"

	// defaultSessionDuration is the fallback for session duration.
	defaultSessionDuration = "1m"

	// defaultMaxHistorySnapshots is the default number of vault snapshots to keep.
	defaultMaxHistorySnapshots = 3
)

var (
	cobraCompletionCommands = []string{"completion", "bash", "fish", "powershell", "zsh"}

	// preRunSkipCommands are commands that skips the pre-run execution.
	preRunSkipCommands = append(
		[]string{"config", "validate", "version"},
		cobraCompletionCommands...,
	)

	// preRunPartialCommands are commands that require partial pre-run execution without vault opening.
	preRunPartialCommands = []string{"create", "generate", "login", "logout", "rotate"}

	// postRunSkipCommands are commands that skips the post-run execution.
	postRunSkipCommands = append(
		cmdutil.SliceWithout(preRunPartialCommands, "rotate"),
		preRunSkipCommands...,
	)

	// persistRequiredCommands lists commands that modify the in-memory vault state,
	// requiring subsequent persistence to the on-disk vault container.
	persistRequiredCommands = []string{
		"import",
		"remove",
		"save",
		"update",
		"secret", // vlt update secret
	}
)

type vaultHooks struct {
	postLogin []string
	postWrite []string
}

type VaultOptions struct {
	path                string
	vault               *vault.Vault
	hooks               vaultHooks
	disableHooks        bool
	nonInteractive      bool
	sessionDuration     time.Duration
	maxHistorySnapshots int
}

var _ genericclioptions.BaseOptions = &VaultOptions{}

type VaultOptionsOpts func(*VaultOptions)

// NewVaultOptions creates a new VaultOptions with provided configurations.
// It will open an existing vault or create a new one if [WithLazyLoad] is enabled.
func NewVaultOptions(opts ...VaultOptionsOpts) *VaultOptions {
	o := &VaultOptions{}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

func (*VaultOptions) Complete() error { return nil }

func (*VaultOptions) Validate() error { return nil }

// Run initializes the Vault object from the specified existing file.
func (o *VaultOptions) Open(ctx context.Context, io *genericclioptions.StdioOptions, sessionClient *vaultdaemon.SessionClient) error {
	exists, err := o.vaultExists()
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("%w: %s", vaulterrors.ErrVaultFileNotFound, o.path)
	}

	opts := []vault.Option{vault.WithMaxHistorySnapshots(o.maxHistorySnapshots)}

	// nil-safe: sessionClient methods handle nil receivers safely.
	key, nonce, err := sessionClient.GetSessionKey(ctx, o.path)
	if err != nil {
		io.Debugf("vlt: no session found, falling back to password: %v\n", err)
	}

	if key == nil || nonce == nil {
		if o.nonInteractive {
			return vaulterrors.ErrInteractiveLoginDisabled
		}

		password, err := o.login(ctx, io, sessionClient)
		if err != nil {
			return err
		}
		defer clear(password)

		opts = append(opts, vault.WithPassword(password))
	} else {
		opts = append(opts, vault.WithSessionKey(key, nonce))
	}

	v, err := vault.Open(ctx, o.path, opts...)
	if err != nil {
		return err
	}

	o.vault = v

	return nil
}

func (o *VaultOptions) login(ctx context.Context, io *genericclioptions.StdioOptions, sessionClient *vaultdaemon.SessionClient) ([]byte, error) {
	password, err := input.PromptReadSecure(io.Out, int(io.In.Fd()), "[vlt] Password for %q:", o.path)
	if err != nil {
		return nil, fmt.Errorf("prompt password: %v", err)
	}

	if len(password) == 0 {
		return nil, vaulterrors.ErrEmptyPassword
	}

	key, nonce, err := vault.Login(ctx, o.path, password, vault.WithMaxHistorySnapshots(o.maxHistorySnapshots))
	if err != nil {
		return nil, err
	}

	_ = sessionClient.Login(ctx, o.path, key, nonce, o.sessionDuration)

	if err := o.postLoginHook(ctx, io); err != nil {
		return nil, fmt.Errorf("post-login hook: %w", err)
	}

	return password, nil
}

func (o *VaultOptions) vaultExists() (bool, error) {
	_, err := os.Stat(o.path)
	if err == nil {
		return true, nil
	}

	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	return false, fmt.Errorf("stat vault file: %w", err)
}

func (o *VaultOptions) postLoginHook(ctx context.Context, io *genericclioptions.StdioOptions) error {
	if o.disableHooks {
		io.Debugf("post-login hook skipped\n")
		return nil
	}

	return genericclioptions.RunHook(ctx, io, "post-login", o.hooks.postLogin)
}

func (o *VaultOptions) postWriteHook(ctx context.Context, io *genericclioptions.StdioOptions) error {
	if o.disableHooks {
		io.Debugf("post-write hook skipped\n")
		return nil
	}

	return genericclioptions.RunHook(ctx, io, "post-write", o.hooks.postWrite)
}

type DefaultVltOptions struct {
	*genericclioptions.StdioOptions

	vaultOptions  *VaultOptions
	configOptions *ConfigOptions

	// sessionClient is used for daemon communication,
	// it is lazily initialized in [DefaultVltOptions.Run].
	sessionClient *vaultdaemon.SessionClient
}

var _ genericclioptions.CmdOptions = &DefaultVltOptions{}

func NewDefaultVltOptions(iostreams *genericclioptions.IOStreams, vaultOptions *VaultOptions) *DefaultVltOptions {
	stdio := &genericclioptions.StdioOptions{IOStreams: iostreams}

	return &DefaultVltOptions{
		configOptions: NewConfigOptions(stdio),
		StdioOptions:  stdio,
		vaultOptions:  vaultOptions,
	}
}

func (o *DefaultVltOptions) Complete() error {
	if err := o.StdioOptions.Complete(); err != nil {
		return err
	}

	if err := o.configOptions.Complete(); err != nil {
		return err
	}

	if err := o.vaultOptions.Complete(); err != nil {
		return err
	}

	return o.complete()
}

//nolint:revive // allow internal complete() alongside public Complete()
func (o *DefaultVltOptions) complete() error {
	copyCmd, pasteCmd := o.configOptions.resolved.CopyCmd, o.configOptions.resolved.PasteCmd

	var opts []clipboard.Opt
	if len(copyCmd) > 0 {
		opts = append(opts, clipboard.WithCopyCmd(copyCmd))
	}

	if len(pasteCmd) > 0 {
		opts = append(opts, clipboard.WithPasteCmd(pasteCmd))
	}

	if len(opts) > 0 {
		clipboard.SetDefault(clipboard.New(opts...))
	}

	o.vaultOptions.maxHistorySnapshots = o.configOptions.resolved.MaxHistorySnapshots
	o.vaultOptions.sessionDuration = time.Duration(o.configOptions.resolved.SessionDuration)
	o.vaultOptions.path = o.configOptions.resolved.VaultPath

	o.vaultOptions.hooks = vaultHooks{
		postLogin: o.configOptions.resolved.PostLoginCmd,
		postWrite: o.configOptions.resolved.PostWriteCmd,
	}

	return nil
}

func (o *DefaultVltOptions) Validate() error {
	if err := o.StdioOptions.Validate(); err != nil {
		return err
	}

	if err := o.configOptions.Validate(); err != nil {
		return err
	}

	return o.vaultOptions.Validate()
}

func (o *DefaultVltOptions) Run(ctx context.Context, args ...string) error {
	if err := o.configOptions.Run(ctx); err != nil {
		return err
	}

	cmd := ""
	if len(args) == 1 {
		cmd = args[0]
	}

	if slices.Contains(preRunPartialCommands, cmd) {
		return nil
	}

	if o.configOptions.resolved.enableSession {
		c, err := vaultdaemon.NewSessionClient()
		if err != nil {
			o.Infof("vlt: daemon unavailable, continuing without session support\nTo enable session support, make sure the 'vltd' daemon is running.\n\n")
		}

		o.sessionClient = c
	}

	return o.vaultOptions.Open(ctx, o.StdioOptions, o.sessionClient)
}

func (o *DefaultVltOptions) postRun(ctx context.Context, cmd string) (retErr error) {
	if slices.Contains(postRunSkipCommands, cmd) {
		return nil
	}

	defer func() {
		if err := o.vaultOptions.vault.Close(); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("post-run: %w", err))
		}

		if err := o.sessionClient.Close(); err != nil {
			o.Errorf("post-run: session client close failed: %v", err)
		}
	}()

	if !slices.Contains(persistRequiredCommands, cmd) {
		return nil
	}

	nonce, err := o.vaultOptions.vault.Seal(ctx)
	if err != nil {
		return fmt.Errorf("post-run: %w", err)
	}

	if err := o.sessionClient.UpdateSession(ctx, o.vaultOptions.path, nonce); err != nil {
		o.Errorf("post-run: session nonce update failed: %v", err)
	}

	if err := o.vaultOptions.postWriteHook(ctx, o.StdioOptions); err != nil {
		o.Errorf("post-write hook failed: %v", err)
	}

	return nil
}

// NewDefaultVltCommand creates the `vlt` command with its sub-commands.
func NewDefaultVltCommand(iostreams *genericclioptions.IOStreams, args []string) *cobra.Command {
	o := NewDefaultVltOptions(iostreams, NewVaultOptions())

	cmd := &cobra.Command{
		Use: "vlt",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Long: `Name:
  vlt - A secure command-line tool for managing secrets in your terminal.

Description:
  User secrets are stored in a fully encrypted, serialized SQLite vault on disk.

  The encrypted vault is only decrypted into memory during a session, and individual secrets
  are decrypted on demand. Decrypted values are ephemeral and cleared from memory immediately after use.

  The vault file is never written to disk in plaintext. 

Environment Variables:
  VLT_CONFIG_PATH - overrides the default config path: "~/.vlt.toml".`,
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if slices.Contains(preRunSkipCommands, cmd.Name()) {
				return nil
			}

			return clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o, cmd.Name()))
		},
		PersistentPostRunE: func(cmd *cobra.Command, _ []string) error {
			return clierror.Check(o.postRun(cmd.Context(), cmd.Name()))
		},
	}

	cmd.SetArgs(args)

	cmd.PersistentFlags().BoolVarP(&o.Verbose, "verbose", "v", false, "enable verbose output")
	cmd.PersistentFlags().BoolVarP(&o.vaultOptions.disableHooks, "no-hooks", "H", false, "disable hook execution")
	cmd.PersistentFlags().BoolVarP(
		&o.vaultOptions.nonInteractive,
		"no-login-prompt",
		"P",
		false,
		"do not prompt for login; use existing session or fail",
	)
	cmd.PersistentFlags().StringVarP(&o.configOptions.cliFlags.vaultPath, "file", "f", "",
		fmt.Sprintf("database file path (default: ~/%s)", defaultDatabaseFilename))
	cmd.PersistentFlags().StringVarP(
		&o.configOptions.cliFlags.configPath,
		"config",
		"",
		"",
		fmt.Sprintf("configuration file path (default: ~/%s)", defaultConfigName),
	)

	hiddenFlags := []string{"config", "verbose", "file", "no-hooks", "no-login-prompt"}
	genericclioptions.MarkFlagsHidden(cmd, hiddenFlags...)

	cmd.AddCommand(newVersionCommand(o))
	cmd.AddCommand(NewCmdGenerate(o))
	cmd.AddCommand(NewCmdConfig(o))
	cmd.AddCommand(NewCmdLogout(o))
	cmd.AddCommand(NewCmdCreate(o))
	cmd.AddCommand(NewCmdRotate(o))
	cmd.AddCommand(NewCmdRemove(o))
	cmd.AddCommand(NewCmdUpdate(o))
	cmd.AddCommand(NewCmdImport(o))
	cmd.AddCommand(NewCmdExport(o))
	cmd.AddCommand(NewCmdVacuum(o))
	cmd.AddCommand(NewCmdLogin(o))
	cmd.AddCommand(NewCmdSave(o))
	cmd.AddCommand(NewCmdFind(o))
	cmd.AddCommand(NewCmdShow(o))

	return cmd
}
