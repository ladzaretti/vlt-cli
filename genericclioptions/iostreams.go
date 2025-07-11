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
func NewTestIOStreams(r *TestFdReader) (iostream *IOStreams, in *TestFdReader, out *bytes.Buffer, errOut *bytes.Buffer) {
	in = r
	out, errOut = &bytes.Buffer{}, &bytes.Buffer{}

	iostream = &IOStreams{
		In:     in,
		Out:    out,
		ErrOut: errOut,
	}

	return
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

// Printf writes a general, unprefixed formatted message to the standard output stream.
func (s IOStreams) Printf(format string, args ...any) {
	fmt.Fprintf(s.Out, format, args...)
}

// Debugf writes formatted debug output to the error stream.
// if Verbose is enabled.
func (s IOStreams) Debugf(format string, args ...any) {
	if s.Verbose {
		fmt.Fprintf(s.ErrOut, "DEBUG "+format, args...)
	}
}

// Infof writes a formatted message to the standard output stream.
func (s IOStreams) Infof(format string, args ...any) {
	fmt.Fprintf(s.Out, "INFO "+format, args...)
}

// Errorf writes a formatted message to the error stream.
func (s IOStreams) Errorf(format string, args ...any) {
	fmt.Fprintf(s.ErrOut, "WARN "+format, args...)
}
