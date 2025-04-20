package genericclioptions

import (
	"errors"
	"fmt"
	"io"

	"github.com/ladzaretti/vlt-cli/input"
)

// ErrInvalidStdinUsage indicates stdin flag is used incorrectly.
var ErrInvalidStdinUsage = errors.New("stdin flag can only be used with piped input")

// StdioOptions provides stdin-related CLI helpers,
// intended to be embedded in option structs.
type StdioOptions struct {
	NonInteractive bool

	*IOStreams
}

var _ BaseOptions = &StdioOptions{}

// Complete sets default values, e.g., enabling Stdin if piped input is detected.
func (o *StdioOptions) Complete() error {
	if !o.NonInteractive {
		fi, err := o.In.Stat()
		if err != nil {
			return fmt.Errorf("stat input: %v", err)
		}

		if input.IsPipedOrRedirected(fi) {
			o.Debugf("Input is piped or redirected; enabling non-interactive mode for handling sensitive data.\n")
			o.NonInteractive = true
		}
	}

	if !o.Verbose {
		o.ErrOut = io.Discard
	}

	return nil
}

// Validate ensures the input mode (Stdin or interactive) is used appropriately.
func (o *StdioOptions) Validate() error {
	fi, err := o.In.Stat()
	if err != nil {
		return fmt.Errorf("stat input: %v", err)
	}

	if o.NonInteractive && !input.IsPipedOrRedirected(fi) {
		return ErrInvalidStdinUsage
	}

	return nil
}
