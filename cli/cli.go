package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ladzaretti/vlt-cli/clierror"
	"github.com/ladzaretti/vlt-cli/clipboard"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vaulterrors"
	"github.com/ladzaretti/vlt-cli/vlt"

	"github.com/spf13/cobra"
)

const (
	// defaultFilename is the default name for the vault file,
	// created under the user's home directory.
	defaultFilename = ".vlt"
)

type VaultOptions struct {
	Path  string
	Vault *vlt.Vault

	newVault bool
}

var _ genericclioptions.CmdOptions = &VaultOptions{}

type VaultOptionsOpts func(*VaultOptions)

// NewVaultOptions creates a new VaultOptions with provided configurations.
// It will open an existing vault or create a new one if [WithNewVault] is enabled.
func NewVaultOptions(opts ...VaultOptionsOpts) *VaultOptions {
	o := &VaultOptions{}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

// WithNewVault enables the creation of a new vault at the specified path.
func WithNewVault(enabled bool) VaultOptionsOpts {
	return func(o *VaultOptions) {
		o.newVault = enabled
	}
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
	if o.newVault {
		return o.validateNewVault()
	}

	return o.validateExistingVault()
}

// Run initializes the Vault object from the specified existing file.
func (o *VaultOptions) Run(_ context.Context) error {
	// creating a new vault is handled internally by the create subcommand.
	if o.newVault {
		return nil
	}

	v, err := vlt.New(o.Path)
	if err != nil {
		return err
	}

	o.Vault = v

	return nil
}

func defaultVaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, defaultFilename), nil
}

func (o *VaultOptions) validateNewVault() error {
	if _, err := os.Stat(o.Path); !errors.Is(err, fs.ErrNotExist) {
		return vaulterrors.ErrVaultFileExists
	}

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

type DefaultVltOptions struct {
	config Config

	*VaultOptions

	*genericclioptions.StdioOptions
}

var _ genericclioptions.CmdOptions = &DefaultVltOptions{}

func NewDefaultVltOptions(iostreams *genericclioptions.IOStreams, vaultOptions *VaultOptions) (*DefaultVltOptions, error) {
	config, err := LoadConfig()
	if err != nil && !errors.Is(err, ErrNoConfigAvailable) {
		return nil, err
	}

	return &DefaultVltOptions{
		config:       config,
		StdioOptions: &genericclioptions.StdioOptions{IOStreams: iostreams},
		VaultOptions: vaultOptions,
	}, nil
}

func (o *DefaultVltOptions) Complete() error {
	vaultPath := o.config.Vault.Path
	if len(vaultPath) > 0 {
		o.Path = vaultPath
	}

	copyCmd, pasteCmd := o.config.Clipboard.CopyCmd, o.config.Clipboard.PasteCmd
	if len(copyCmd) > 0 || len(pasteCmd) > 0 {
		opts := []clipboard.Opt{
			clipboard.WithCopyCmd(copyCmd),
			clipboard.WithPasteCmd(pasteCmd),
		}
		clipboard.SetDefault(clipboard.New(opts...))
	}

	if err := o.StdioOptions.Complete(); err != nil {
		return err
	}

	return o.VaultOptions.Complete()
}

func (o *DefaultVltOptions) Validate() error {
	if err := o.StdioOptions.Validate(); err != nil {
		return err
	}

	return o.VaultOptions.Validate()
}

func (o *DefaultVltOptions) Run(ctx context.Context) error {
	return o.VaultOptions.Run(ctx)
}

// NewDefaultVltCommand creates the `vlt` command with its sub-commands.
func NewDefaultVltCommand(iostreams *genericclioptions.IOStreams, args []string) *cobra.Command {
	o, err := NewDefaultVltOptions(iostreams, NewVaultOptions())
	clierror.Check(err)

	cmd := &cobra.Command{
		Use:          "vlt",
		Short:        "vault CLI for managing secrets",
		Long:         "vlt is a command-line password manager for securely storing and retrieving credentials.",
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			if cmd.Name() == "create" {
				WithNewVault(true)(o.VaultOptions)
			}

			clierror.Check(genericclioptions.ExecuteCommand(cmd.Context(), o))
		},
	}

	cmd.SetArgs(args)

	cmd.PersistentFlags().BoolVarP(&o.Verbose, "verbose", "v", false, "enable verbose output")
	cmd.PersistentFlags().StringVarP(&o.Path, "file", "f", "", "path to the vault file")

	cmd.AddCommand(NewCmdCreate(o.StdioOptions, func() string { return o.Path }))
	cmd.AddCommand(NewCmdLogin(o.StdioOptions, func() *vlt.Vault { return o.Vault }))
	cmd.AddCommand(NewCmdSave(o.StdioOptions, func() *vlt.Vault { return o.Vault }))
	cmd.AddCommand(NewCmdFind(o.StdioOptions, func() *vlt.Vault { return o.Vault }))
	cmd.AddCommand(NewCmdRemove(o.StdioOptions, func() *vlt.Vault { return o.Vault }))
	cmd.AddCommand(NewCmdShow(o.StdioOptions, func() *vlt.Vault { return o.Vault }))

	return cmd
}
