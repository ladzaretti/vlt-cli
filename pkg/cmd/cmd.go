package cmd

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ladzaretti/vlt-cli/pkg/cmd/create"
	"github.com/ladzaretti/vlt-cli/pkg/cmd/login"
	"github.com/ladzaretti/vlt-cli/pkg/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vlt"

	"github.com/spf13/cobra"
)

const (
	defaultFilename = ".vlt"
)

var ErrFileExists = errors.New("vault file path already exists")

type VltOptions struct {
	File string

	Vault *vlt.Vault

	genericclioptions.IOStreams
}

var _ genericclioptions.CmdOptions = &VltOptions{}

func NewVltOptions(iostreams genericclioptions.IOStreams) *VltOptions {
	return &VltOptions{IOStreams: iostreams}
}

func (o *VltOptions) Complete() error {
	if len(o.File) == 0 {
		p, err := defaultVaultPath()
		if err != nil {
			return err
		}

		o.File = p
	}

	if !o.Verbose {
		o.ErrOut = io.Discard
	}

	return nil
}

func (o *VltOptions) Validate() error {
	if _, err := os.Stat(o.File); !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(o.ErrOut, "A file already exists at path: %q. Cannot create a new vault.\n", o.File)
		return ErrFileExists
	}

	return nil
}

func (o *VltOptions) Run() error {
	v, err := vlt.New(o.File)
	if err != nil {
		return err
	}

	o.Vault = v

	return nil
}

// NewDefaultVltCommand creates the `vlt` command with its sub-commands.
func NewDefaultVltCommand(iostreams genericclioptions.IOStreams, args []string) *cobra.Command {
	o := NewVltOptions(iostreams)

	cmd := &cobra.Command{
		Use:   "vlt",
		Short: "vault CLI for managing secrets",
		Long:  "vlt is a command-line password manager for securely storing and retrieving credentials.",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return genericclioptions.ExecuteCommand(o)
		},
	}

	cmd.SetArgs(args)

	cmd.PersistentFlags().BoolVarP(&o.Verbose, "verbose", "v", false, "enable verbose output")
	cmd.PersistentFlags().StringVarP(&o.File, "file", "f", "", "path to the vault file")

	cmd.AddCommand(login.NewCmdLogin())
	cmd.AddCommand(create.NewCmdCreate(o.IOStreams, o.Vault))

	return cmd
}

func defaultVaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, defaultFilename), nil
}
