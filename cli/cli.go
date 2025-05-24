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
	"github.com/ladzaretti/vlt-cli/vault"
	"github.com/ladzaretti/vlt-cli/vaultdaemon"
	"github.com/ladzaretti/vlt-cli/vaulterrors"

	"github.com/spf13/cobra"
)

const (
	// defaultDatabaseFilename is the default name for the vault file,
	// created under the user's home directory.
	defaultDatabaseFilename = ".vlt"

	// defaultConfigName is the default name of the configuration file
	// expected under the user's home directory.
	defaultConfigName = ".vlt.toml"

	// defaultSessionDuration is the fallback when no session duration is set.
	defaultSessionDuration = "1m"
)

var (
	// preRunSkipCommands lists command names that should
	// bypass the persistent pre-run logic.
	preRunSkipCommands = []string{"config", "generate", "validate"}

	// preRunPartialCommands lists commands that require partial
	// preRunPartialCommands run setup like path resolution, but skip vault opening.
	preRunPartialCommands = []string{"create", "login", "logout"}

	// postRunSkipCommands lists command names that should
	// bypass the persistent post-run logic.
	postRunSkipCommands = []string{"config", "generate", "validate", "create", "login", "logout"}
)

type VaultOptions struct {
	path  string
	vault *vault.Vault

	// TODO1: post update for import,remove,save & update [secret]
	// TODO2: post login for login command and a general one for cli
	// TODO3: hooks should probably be done via some kind of a struct
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
func (o *VaultOptions) Open(ctx context.Context, sessionClient *vaultdaemon.SessionClient, io *genericclioptions.StdioOptions, sessionDuration time.Duration) error {
	exists, err := o.vaultExists()
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("%w: %s", vaulterrors.ErrVaultFileNotFound, o.path)
	}

	opts := []vault.Option{}

	// nil-safe: sessionClient methods handle nil receivers safely.
	key, nonce, err := sessionClient.GetSessionKey(ctx, o.path)
	if err != nil {
		io.Debugf("vlt: no session found, falling back to password: %v\n", err)
	}

	if key == nil || nonce == nil {
		password, err := input.PromptReadSecure(io.Out, int(io.In.Fd()), "[vlt] Password for %q:", o.path)
		if err != nil {
			return fmt.Errorf("prompt password: %v", err)
		}

		key, nonce, err := vault.Login(ctx, o.path, password)
		if err != nil {
			io.Debugf("%v", err)
		} else {
			_ = sessionClient.Login(ctx, o.path, key, nonce, sessionDuration)
		}

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

type DefaultVltOptions struct {
	*genericclioptions.StdioOptions

	vaultOptions  *VaultOptions
	configOptions *ConfigOptions

	// sessionClient is used for daemon communication,
	// it is lazily initialized in [DefaultVltOptions.Run].
	sessionClient *vaultdaemon.SessionClient
}

var _ genericclioptions.CmdOptions = &DefaultVltOptions{}

func NewDefaultVltOptions(iostreams *genericclioptions.IOStreams, vaultOptions *VaultOptions) (*DefaultVltOptions, error) {
	stdio := &genericclioptions.StdioOptions{IOStreams: iostreams}

	return &DefaultVltOptions{
		configOptions: NewConfigOptions(stdio),
		StdioOptions:  stdio,
		vaultOptions:  vaultOptions,
	}, nil
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

	o.vaultOptions.path = o.configOptions.resolved.VaultPath

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

	c, err := vaultdaemon.NewSessionClient()
	if err != nil {
		o.Infof("vlt: daemon unavailable, continuing without session support\nTo enable session support, make sure the 'vltd' daemon is running.\n\n")
	}

	o.sessionClient = c
	sessionDuration := time.Duration(o.configOptions.resolved.SessionDuration)

	return o.vaultOptions.Open(ctx, o.sessionClient, o.StdioOptions, sessionDuration)
}

// NewDefaultVltCommand creates the `vlt` command with its sub-commands.
func NewDefaultVltCommand(iostreams *genericclioptions.IOStreams, args []string) *cobra.Command {
	o, err := NewDefaultVltOptions(iostreams, NewVaultOptions())
	clierror.Check(err)

	cmd := &cobra.Command{
		Use:   "vlt",
		Short: "Command-line in-memory secret manager",
		Long: `vlt is an encrypted in-memory command-line secret manager.

Environment Variables:
    VLT_CONFIG_PATH: overrides the default config path: "~/.vlt.toml".`,
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			if slices.Contains(preRunSkipCommands, cmd.Name()) {
				return
			}

			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o, cmd.Name()))
		},
		PersistentPostRun: func(cmd *cobra.Command, _ []string) {
			if slices.Contains(postRunSkipCommands, cmd.Name()) {
				return
			}

			clierror.Check(errors.Join(
				o.vaultOptions.vault.Close(cmd.Context()),
				o.sessionClient.Close(),
			))
		},
	}

	cmd.SetArgs(args)

	cmd.PersistentFlags().BoolVarP(&o.Verbose, "verbose", "v", false, "enable verbose output")
	cmd.PersistentFlags().StringVarP(&o.configOptions.cliFlags.vaultPath, "file", "f", "",
		fmt.Sprintf("database file path (default: ~/%s)", defaultDatabaseFilename))
	cmd.PersistentFlags().StringVarP(
		&o.configOptions.cliFlags.configPath,
		"config",
		"",
		"",
		fmt.Sprintf("configuration file path (default: ~/%s)", defaultConfigName),
	)

	cmd.AddCommand(NewCmdGenerate(o))
	cmd.AddCommand(NewCmdConfig(o))
	cmd.AddCommand(NewCmdLogout(o))
	cmd.AddCommand(NewCmdCreate(o))
	cmd.AddCommand(NewCmdRemove(o))
	cmd.AddCommand(NewCmdUpdate(o))
	cmd.AddCommand(NewCmdImport(o))
	cmd.AddCommand(NewCmdExport(o))
	cmd.AddCommand(NewCmdLogin(o))
	cmd.AddCommand(NewCmdSave(o))
	cmd.AddCommand(NewCmdFind(o))
	cmd.AddCommand(NewCmdShow(o))

	return cmd
}
