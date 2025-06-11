package genericclioptions

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// MarkFlagsHidden hides the given flags from the target's help output.
func MarkFlagsHidden(target *cobra.Command, hidden ...string) {
	f := target.HelpFunc()

	target.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd == target {
			for _, n := range hidden {
				flag := cmd.Flags().Lookup(n)
				if flag != nil {
					flag.Hidden = true
				}
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

	io.Infof("running %s hook: %q...\n\n", alias, hook)

	defer func() {
		if retErr != nil {
			io.Errorf("%s hook failed.\n\n", alias)
			return
		}

		io.Infof("%s hook completed successfully.\n\n", alias)
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
