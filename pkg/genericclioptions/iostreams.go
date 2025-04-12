package genericclioptions

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

type IOStreams struct {
	In     FdReader
	Out    io.Writer
	ErrOut io.Writer

	Verbose bool
}

// NewDefaultIOStreams returns the default IOStreams (using os.Stdin, os.Stdout, os.Stderr).
func NewDefaultIOStreams() *IOStreams {
	return &IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
}

// NewTestIOStreamsWithMockInput returns IOStreams with mock input,
// a [TestFdReader] and out and error buffers for unit tests.
//
//nolint:revive
func NewTestIOStreams(in *TestFdReader) (*IOStreams, *TestFdReader, *bytes.Buffer, *bytes.Buffer) {
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}

	return &IOStreams{
		In:     in,
		Out:    out,
		ErrOut: errOut,
	}, in, out, errOut
}

// NewTestIOStreamsDiscard returns IOStreams with mocked input,
// and discards both output and error output.
func NewTestIOStreamsDiscard(in *TestFdReader) *IOStreams {
	return &IOStreams{
		In:     in,
		Out:    io.Discard,
		ErrOut: io.Discard,
	}
}

// Debugf writes formatted debug output if Verbose is enabled.
func (s IOStreams) Debugf(format string, args ...any) {
	if s.Verbose {
		fmt.Fprintf(s.ErrOut, format, args...)
	}
}

// Infof writes a formatted message to standard output.
// Use this for user-facing informational messages.
func (s IOStreams) Infof(format string, args ...any) {
	fmt.Fprintf(s.Out, format, args...)
}
