package create

import (
	"errors"
	"fmt"
	"log"

	"github.com/ladzaretti/vlt-cli/pkg/genericclioptions"
	"github.com/ladzaretti/vlt-cli/pkg/input"
	"github.com/ladzaretti/vlt-cli/vlt"

	"github.com/spf13/cobra"
)

var ErrUnexpectedPipedData = errors.New("unexpected piped data")

// createOptions have the data required to perform the create operation.
type createOptions struct {
	stdin bool

	genericclioptions.Opts
}

// newCreateOptions creates the options for create.
func newCreateOptions(opts genericclioptions.Opts) *createOptions {
	return &createOptions{Opts: opts}
}

// NewCmdCreate creates a new create command.
func NewCmdCreate(opts genericclioptions.Opts) *cobra.Command {
	o := newCreateOptions(opts)

	cmd := &cobra.Command{
		Use:   "create",
		Short: "initialize a new vault",
		Long:  "create a new vault by specifying the SQLite database file where credentials will be stored.",
		RunE: func(_ *cobra.Command, _ []string) error {
			return o.run()
		},
	}

	cmd.Flags().BoolVarP(&o.stdin, "input", "i", false,
		"read password from stdin instead of prompting the user")

	return cmd
}

func (o *createOptions) run() error {
	if err := o.ResolveFilePath(); err != nil {
		return err
	}

	log.Printf("Using database filepath: %q", o.File)

	mk, err := o.readNewMasterKey()
	if err != nil {
		log.Printf("Could not read user password: %v\n", err)
		return fmt.Errorf("read user password: %v", err)
	}

	vault, err := vlt.Open(o.File)
	if err != nil {
		return fmt.Errorf("create new vault: %v", err)
	}

	if err := vault.SetMasterKey(mk); err != nil {
		log.Printf("Failure setting vault master key: %v\n", err)
		return fmt.Errorf("set master key: %v", err)
	}

	return nil
}

func (o *createOptions) readNewMasterKey() (string, error) {
	if input.IsPiped() && !o.stdin {
		return "", ErrUnexpectedPipedData
	}

	if o.stdin {
		log.Println("Reading password from stdin")

		pass, err := input.ReadTrimStdin()
		if err != nil {
			return "", fmt.Errorf("read password: %v", err)
		}

		return pass, nil
	}

	pass, err := input.PromptNewPassword()
	if err != nil {
		return "", fmt.Errorf("prompt new password: %v", err)
	}

	return pass, nil
}
