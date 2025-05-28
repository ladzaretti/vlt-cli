package genericclioptions

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func MarkFlagsHidden(sub *cobra.Command, hidden ...string) {
	f := sub.HelpFunc()
	sub.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		for _, n := range hidden {
			flag := cmd.Flags().Lookup(n)
			if flag != nil {
				flag.Hidden = true
			}
		}

		f(cmd, args)
	})
}

func RejectDisallowedFlags(cmd *cobra.Command, disallowed ...string) error {
	for _, name := range disallowed {
		if cmd.Flags().Changed(name) {
			return fmt.Errorf("flag --%s is not allowed with '%s' command", name, cmd.Name())
		}
	}

	return nil
}

func RunCommandWithInput(ctx context.Context, io *StdioOptions, r io.Reader, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)

	cmd.Stdin = r
	cmd.Stdout = io.Out
	cmd.Stderr = io.ErrOut

	return cmd.Run()
}

func RunCommand(ctx context.Context, io *StdioOptions, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)

	cmd.Stdin = io.In
	cmd.Stdout = io.Out
	cmd.Stderr = io.ErrOut

	return cmd.Run()
}

func RunHook(ctx context.Context, io *StdioOptions, alias string, hook []string) (retErr error) {
	if len(hook) == 0 {
		return nil
	}

	cmd, args := hook[0], hook[1:]

	io.Infof("\nrunning %s hook: %q...\n\n", alias, hook)

	defer func() {
		if retErr != nil {
			io.Warnf("\n%s hook failed.\n\n", alias)
			return
		}

		io.Infof("\n%s hook completed successfully.\n\n", alias)
	}()

	return RunCommand(ctx, io, cmd, args...)
}

func StringContains(str string, substrings ...string) bool {
	for _, substr := range substrings {
		if strings.Contains(str, substr) {
			return true
		}
	}

	return false
}

// SetHelpOutput overrides the help output for a Cobra command.
// It captures the default help text and passes it to the given filter f.
func SetHelpOutput(cmd *cobra.Command, f func(*cobra.Command, io.Reader)) {
	helpFunc := cmd.HelpFunc()

	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		var buf bytes.Buffer

		w := cmd.OutOrStdout()

		cmd.SetOut(&buf)
		helpFunc(cmd, args)

		cmd.SetOut(w)

		f(cmd, &buf)
	})
}

// HelpFilterFunc returns a Cobra-compatible help output filter function.
//
// This is intended to be used with [SetHelpOutput] to customize or reduce
// help output without affecting flag behavior.
//
// Example:
//
//	SetHelpOutput(cmd, HelpFilterFunc(os.Stdout, []string{"--verbose", "--config"}))
func HelpFilterFunc(out io.Writer, substrings []string) func(*cobra.Command, io.Reader) {
	return func(_ *cobra.Command, r io.Reader) {
		var sb strings.Builder

		s := bufio.NewScanner(r)
		for s.Scan() {
			line := s.Text()
			if StringContains(line, substrings...) {
				continue
			}

			sb.WriteString(line + "\n")
		}

		_, _ = fmt.Fprint(out, sb.String())
	}
}
