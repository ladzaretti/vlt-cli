package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/clipboard"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/input"
	"github.com/ladzaretti/vlt-cli/vault"
	"github.com/ladzaretti/vlt-cli/vaulterrors"

	"github.com/spf13/cobra"
)

const (
	// defaultDatabaseFilename is the default name for the vault file,
	// created under the user's home directory.
	defaultDatabaseFilename = ".vlt"
)

var (
	// preRunSkipCommands lists command names that should
	// bypass the persistent pre-run logic.
	preRunSkipCommands = []string{"config", "generate", "validate", "create"}

	// postRunSkipCommands lists command names that should
	// bypass the persistent post-run logic.
	postRunSkipCommands = []string{"config", "generate", "validate", "create"}
)

type VaultOptions struct {
	Path  string
	Vault *vault.Vault
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

// Complete sets the default vault file path if not provided.
func (o *VaultOptions) Complete() error {
	if len(o.Path) == 0 {
		p, err := defaultVaultPath()
		if err != nil {
			return err
		}

		o.Path = p
	}

	return nil
}

// Validate validates the vault options based on whether it's a new or existing vault.
func (o *VaultOptions) Validate() error {
	return o.validateExistingVault()
}

// Run initializes the Vault object from the specified existing file.
func (o *VaultOptions) Open(ctx context.Context, io *genericclioptions.StdioOptions) error {
	password, err := input.PromptReadSecure(io.Out, int(io.In.Fd()), "[vlt] Password for %q:", o.Path)
	if err != nil {
		return fmt.Errorf("prompt password: %v", err)
	}

	v, err := vault.Open(ctx, password, o.Path)
	if err != nil {
		return err
	}

	o.Vault = v

	return nil
}

func (o *VaultOptions) validateExistingVault() error {
	if _, err := os.Stat(o.Path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return vaulterrors.ErrVaultFileNotFound
		}

		return fmt.Errorf("stat vault file: %w", err)
	}

	return nil
}

func (o *VaultOptions) VaultFunc() *vault.Vault {
	return o.Vault
}

func defaultVaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, defaultDatabaseFilename), nil
}

type DefaultVltOptions struct {
	*genericclioptions.StdioOptions

	vaultOptions  *VaultOptions
	configOptions *ConfigOptions
}

var _ genericclioptions.CmdOptions = &DefaultVltOptions{}

func NewDefaultVltOptions(iostreams *genericclioptions.IOStreams, vaultOptions *VaultOptions) (*DefaultVltOptions, error) {
	return &DefaultVltOptions{
		configOptions: &ConfigOptions{},
		StdioOptions:  &genericclioptions.StdioOptions{IOStreams: iostreams},
		vaultOptions:  vaultOptions,
	}, nil
}

func (o *DefaultVltOptions) Complete() error {
	copyCmd, pasteCmd := o.configOptions.Clipboard.CopyCmd, o.configOptions.Clipboard.PasteCmd

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

	if err := o.StdioOptions.Complete(); err != nil {
		return err
	}

	if err := o.configOptions.Complete(); err != nil {
		return err
	}

	return o.vaultOptions.Complete()
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

func (o *DefaultVltOptions) Run(ctx context.Context) error {
	if err := o.configOptions.Run(ctx); err != nil {
		return err
	}

	p := o.configOptions.Vault.Path
	if len(p) > 0 {
		o.vaultOptions.Path = p
	}

	return o.vaultOptions.Open(ctx, o.StdioOptions)
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

			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
		PersistentPostRun: func(cmd *cobra.Command, _ []string) {
			if slices.Contains(postRunSkipCommands, cmd.Name()) {
				return
			}

			clierror.Check(o.vaultOptions.Vault.Close(cmd.Context()))
		},
	}

	cmd.SetArgs(args)

	cmd.PersistentFlags().BoolVarP(&o.Verbose, "verbose", "v", false, "enable verbose output")
	cmd.PersistentFlags().StringVarP(&o.vaultOptions.Path, "file", "f", "",
		fmt.Sprintf("database file path (default: ~/%s)", defaultDatabaseFilename))
	cmd.PersistentFlags().StringVarP(
		&o.configOptions.userPath,
		"config",
		"",
		"",
		fmt.Sprintf("configuration file path (default: ~/%s)", defaultConfigName),
	)

	cmd.AddCommand(NewCmdConfig(o.StdioOptions))
	cmd.AddCommand(NewCmdGenerate(o.StdioOptions))

	cmd.AddCommand(NewCmdCreate(o.StdioOptions, o.vaultOptions))
	cmd.AddCommand(NewCmdLogin(o.StdioOptions, o.vaultOptions.VaultFunc))
	cmd.AddCommand(NewCmdLogout(o.StdioOptions, o.vaultOptions.VaultFunc))
	cmd.AddCommand(NewCmdSave(o.StdioOptions, o.vaultOptions.VaultFunc))
	cmd.AddCommand(NewCmdFind(o.StdioOptions, o.vaultOptions.VaultFunc))
	cmd.AddCommand(NewCmdShow(o.StdioOptions, o.vaultOptions.VaultFunc))
	cmd.AddCommand(NewCmdRemove(o.StdioOptions, o.vaultOptions.VaultFunc))
	cmd.AddCommand(NewCmdUpdate(o.StdioOptions, o.vaultOptions.VaultFunc))
	cmd.AddCommand(NewCmdImport(o.StdioOptions, o.vaultOptions.VaultFunc))
	cmd.AddCommand(NewCmdExport(o.StdioOptions, o.vaultOptions.VaultFunc))

	return cmd
}
