package genericclioptions

// CmdOptions defines the interface for command options that require
// completion, validation, and execution.
type CmdOptions interface {
	Complete() error // Complete prepares the options for the command by setting required values.
	Validate() error // Validate checks that the options are valid before running the command.
	Run() error      // Run executes the main logic of the command.
}

// ExecuteCommand executes the provided command options by first completing,
// then validating, and finally running the command.
func ExecuteCommand(cmd CmdOptions) error {
	if err := cmd.Complete(); err != nil {
		return err
	}

	if err := cmd.Validate(); err != nil {
		return err
	}

	return cmd.Run()
}
