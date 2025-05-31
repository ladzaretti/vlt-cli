// Package clipboard provides utilities to interact with the system clipboard
// using external commands, with `xsel` as the default.
//
// It supports copying to and pasting from the clipboard,
// and allows customization of the commands used.
package clipboard

import (
	"os/exec"
)

var (
	defaultCopy  = []string{"xsel", "-ib"}
	defaultPaste = []string{"xsel", "-ob"}
)

// ConfigurationError indicates that a clipboard command is not available
// or misconfigured on the host system.
type ConfigurationError struct {
	Op  string
	Err error
}

func (ce *ConfigurationError) Error() string {
	return "clipboard: " + ce.Op + ": " + ce.Err.Error()
}

func (ce *ConfigurationError) Unwrap() error {
	return ce.Err
}

var clipboard = New()

// SetDefault replaces the global clipboard instance.
// Intended custom configurations or testing.
func SetDefault(c *Clipboard) {
	if c == nil {
		panic("clipboard: cannot set default to nil")
	}

	clipboard = c
}

// Copy writes the given string to the system clipboard
// using the default command.
func Copy(s string) error {
	return clipboard.Copy(s)
}

// Paste reads and returns the current contents of the system clipboard
// using the default command.
func Paste() (string, error) {
	return clipboard.Paste()
}

type cmd struct {
	cmd  string
	args []string
}

func newCmd(s []string) cmd {
	if len(s) == 0 {
		return cmd{}
	}

	return cmd{
		cmd:  s[0],
		args: s[1:],
	}
}

type Clipboard struct {
	copy  cmd
	paste cmd
}

type Opt func(*Clipboard)

// New returns a new [Clipboard] instance.
// By default, it uses xsel for both copy and paste.
func New(opts ...Opt) *Clipboard {
	c := &Clipboard{
		copy:  newCmd(defaultCopy),
		paste: newCmd(defaultPaste),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// WithCopyCmd sets a custom copy command.
func WithCopyCmd(copyCmd []string) Opt {
	return func(c *Clipboard) {
		c.copy = newCmd(copyCmd)
	}
}

// WithPasteCmd sets a custom paste command.
func WithPasteCmd(pasteCmd []string) Opt {
	return func(c *Clipboard) {
		c.paste = newCmd(pasteCmd)
	}
}

// Copy writes the provided string to the clipboard.
func (c *Clipboard) Copy(s string) error {
	if _, err := exec.LookPath(c.copy.cmd); err != nil {
		return &ConfigurationError{"copy-clipboard", err}
	}

	//nolint:gosec // G204: safe, user config on local CLI tool
	cmd := exec.Command(c.copy.cmd, c.copy.args...)

	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := in.Write([]byte(s)); err != nil {
		return err
	}

	if err := in.Close(); err != nil {
		return err
	}

	return cmd.Wait()
}

// Paste reads and returns the current contents of the system clipboard.
func (c *Clipboard) Paste() (string, error) {
	if _, err := exec.LookPath(c.paste.cmd); err != nil {
		return "", &ConfigurationError{"paste-clipboard", err}
	}

	//nolint:gosec // G204: safe, user config on local CLI tool
	cmd := exec.Command(c.paste.cmd, c.paste.args...)

	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return string(out), err
}
