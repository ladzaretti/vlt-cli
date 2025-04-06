package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ladzaretti/vlt-cli/cli/create"
	"github.com/ladzaretti/vlt-cli/cli/login"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vlt"

	"github.com/spf13/cobra"
)

const (
	// defaultFilename is the default name for the vault file,
	// created under the user's home directory.
	defaultFilename = ".vlt"
)

var (
	ErrVaultFileExists   = errors.New("vault file path already exists")
	ErrVaultFileNotFound = errors.New("vault file does not exist")
)

type VaultOptions struct {
	File  string
	Vault *vlt.Vault

	newVault bool
}

var _ genericclioptions.BaseOptions = &VaultOptions{}

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
	if len(o.File) == 0 {
		p, err := defaultVaultPath()
		if err != nil {
			return err
		}

		o.File = p
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

// Run initializes the Vault object from the specified file.
func (o *VaultOptions) Run() error {
	v, err := vlt.New(o.File)
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
	if _, err := os.Stat(o.File); !errors.Is(err, fs.ErrNotExist) {
		return ErrVaultFileExists
	}

	return nil
}

func (o *VaultOptions) validateExistingVault() error {
	if _, err := os.Stat(o.File); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrVaultFileNotFound
		}

		return fmt.Errorf("stat vault file: %w", err)
	}

	return nil
}

type DefaultVltOptions struct {
	*VaultOptions

	genericclioptions.IOStreams
}

var _ genericclioptions.CmdOptions = &DefaultVltOptions{}

func NewDefaultVltOptions(iostreams genericclioptions.IOStreams, vaultOptions *VaultOptions) *DefaultVltOptions {
	return &DefaultVltOptions{
		IOStreams:    iostreams,
		VaultOptions: vaultOptions,
	}
}

// NewDefaultVltCommand creates the `vlt` command with its sub-commands.
func NewDefaultVltCommand(iostreams genericclioptions.IOStreams, args []string) *cobra.Command {
	o := NewDefaultVltOptions(iostreams, NewVaultOptions())

	cmd := &cobra.Command{
		Use:           "vlt",
		Short:         "vault CLI for managing secrets",
		Long:          "vlt is a command-line password manager for securely storing and retrieving credentials.",
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Name() == "create" {
				WithNewVault(true)(o.VaultOptions)
			}
			return genericclioptions.ExecuteCommand(o)
		},
	}

	cmd.SetArgs(args)

	cmd.PersistentFlags().BoolVarP(&o.Verbose, "verbose", "v", false, "enable verbose output")
	cmd.PersistentFlags().StringVarP(&o.File, "file", "f", "", "path to the vault file")

	cmd.AddCommand(create.NewCmdCreate(o.IOStreams, o.Vault))
	cmd.AddCommand(login.NewCmdLogin(o.IOStreams, o.Vault))

	return cmd
}
