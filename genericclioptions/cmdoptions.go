package genericclioptions

import "context"

// BaseOptions defines the interface for shared setup and validation logic.
type BaseOptions interface {
	Complete() error // Complete prepares the options for the command by setting required values.
	Validate() error // Validate checks that the options are valid before running the command.
}

// CmdOptions includes BaseOptions and adds the ability to run the command logic.
type CmdOptions interface {
	BaseOptions

	Run(ctx context.Context, args ...string) error
}

// ExecuteCommand executes the provided command options by first completing,
// then validating, and finally running the command.
func ExecuteCommand(ctx context.Context, cmd CmdOptions, args ...string) error {
	if err := cmd.Complete(); err != nil {
		return err
	}

	if err := cmd.Validate(); err != nil {
		return err
	}

	return cmd.Run(ctx, args...)
}
