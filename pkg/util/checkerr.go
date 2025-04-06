package util

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ladzaretti/vlt-cli/pkg/genericclioptions"
	"github.com/ladzaretti/vlt-cli/pkg/vaulterrors"
)

const (
	DefaultErrorExitCode = 1
)

var (
	// fatalErrHandler is the function used to handle fatal errors.
	// By default, it calls os.Exit(1).
	fatalErrHandler = fatal

	// fprintf is the function used to format and print errors.
	fprintf = fmt.Fprintf
)

// BehaviorOnFatal allows you to override the default behavior when a fatal
// error occurs. By default, it calls os.Exit(1). You can pass 'panic' as a function
// here if you prefer a panic() instead of os.Exit(1).
func BehaviorOnFatal(f func(string, int)) {
	fatalErrHandler = f
}

// DefaultBehaviorOnFatal restores the default behavior for fatal errors,
// which is to call os.Exit(1). Useful for tests.
func DefaultBehaviorOnFatal() {
	fatalErrHandler = fatal
}

// SetDefaultFprintf sets the default function used to print errors.
func SetDefaultFprintf(f func(w io.Writer, format string, a ...any) (n int, err error)) {
	fprintf = f
}

// fatal prints the message (if provided) and then exits with the given code.
func fatal(msg string, code int) {
	if len(msg) > 0 {
		// add newline if needed
		if !strings.HasSuffix(msg, "\n") {
			msg += "\n"
		}

		_, _ = fprintf(os.Stderr, msg)
	}

	//nolint:revive // Intentional exit after fatal error.
	os.Exit(code)
}

// ErrExit may be passed to CheckError to instruct it to output nothing but exit with
// status code 1.
var ErrExit = errors.New("exit")

// CheckErr prints a user friendly error and exits with a non-zero
// exit code. Unrecognized errors will be printed with an "error: " prefix.
//
// This method is generic to the command in use.
func CheckErr(err error) {
	checkErr(err, fatalErrHandler)
}

//nolint:revive
func checkErr(err error, handleErr func(string, int)) {
	if err == nil {
		return
	}

	switch {
	case errors.Is(err, ErrExit):
		handleErr("", DefaultErrorExitCode)
	case errors.Is(err, vaulterrors.ErrVaultFileExists):
		handleErr("Vault file already exists.\nConsider deleting the file first before running `create` to create a new vault.", DefaultErrorExitCode)
	case errors.Is(err, vaulterrors.ErrVaultFileNotFound):
		handleErr("Vault file not found.\nUse the `create` command to create a new vault file.", DefaultErrorExitCode)
	case errors.Is(err, genericclioptions.ErrInvalidStdinUsage):
		handleErr("Invalid use of the --stdin flag.\nMake sure you're piping input into the command when using this flag.", DefaultErrorExitCode)
	default:
		msg, ok := StandardErrorMessage(err)
		if !ok {
			msg = err.Error()
			if !strings.HasPrefix(msg, "error: ") {
				msg = "error: " + msg
			}
		}

		handleErr(msg, DefaultErrorExitCode)
	}
}

func StandardErrorMessage(_ error) (string, bool) {
	return "", false
}
