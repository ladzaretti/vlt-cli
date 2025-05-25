package genericclioptions

import (
	"context"
	"fmt"
	"io"
	"os/exec"

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

func RunHook(ctx context.Context, io *StdioOptions, hook []string) (retErr error) {
	if len(hook) == 0 {
		return nil
	}

	cmd, args := hook[0], hook[1:]

	io.Infof("\nRunning hook: %q...\n\n", hook)

	defer func() {
		if retErr != nil {
			io.Warnf("\nHook failed. \n\n")
			return
		}

		io.Infof("\nHook completed successfully.\n\n")
	}()

	return RunCommand(ctx, io, cmd, args...)
}
