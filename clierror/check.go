package clierror

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ladzaretti/vlt-cli/vaultdaemon"
	"github.com/ladzaretti/vlt-cli/vaulterrors"
)

const (
	DefaultErrorExitCode = 1
)

var (
	// errHandler is the function used to handle cli errors.
	errHandler = FatalErrHandler

	// errWriter is used to output cli error messages.
	errWriter io.Writer = os.Stderr

	// fprintf is the function used to format and print errors.
	fprintf = fmt.Fprintf

	// debugMode enables always printing raw error values.
	debugMode bool
)

// SetErrorHandler overrides the default [FatalErrHandler] error handler.
func SetErrorHandler(f func(string, int)) {
	errHandler = f
}

// ResetErrorHandler restores the default error handler.
func ResetErrorHandler() {
	errHandler = FatalErrHandler
}

// SetErrWriter overrides the default error output writer [os.Stderr].
func SetErrWriter(w io.Writer) {
	errWriter = w
}

// ResetErrWriter restores the default error output writer to [os.Stderr].
func ResetErrWriter() {
	errWriter = os.Stderr
}

// SetDefaultFprintf sets the default function used to print errors.
func SetDefaultFprintf(f func(w io.Writer, format string, a ...any) (n int, err error)) {
	fprintf = f
}

// DebugMode sets whether debug logging is enabled.
//
// When enabled, raw error values are printed to stderr.
func DebugMode(enabled bool) {
	debugMode = enabled
}

// FatalErrHandler prints the message provided and then exits with the given code.
func FatalErrHandler(msg string, code int) {
	printError(msg)

	//nolint:revive // Intentional exit after fatal error.
	os.Exit(code)
}

func PrintErrHandler(msg string, _ int) {
	printError(msg)
}

func printError(msg string) {
	if len(msg) == 0 {
		return
	}

	// add newline if needed
	if !strings.HasSuffix(msg, "\n") {
		msg += "\n"
	}

	_, _ = fprintf(errWriter, msg)
}

func debugPrint(err error) {
	if !debugMode {
		return
	}

	_, _ = fprintf(errWriter, "DEBUG %+v\n", err)
}

// ErrExit may be passed to CheckError to instruct it to output nothing but exit with
// status code 1.
var ErrExit = errors.New("exit")

// Check prints a user-friendly error message and invokes the configured error handler.
//
// When the [FatalErrHandler] is used, the program will exit before this function returns.
func Check(err error) error {
	check(err, errHandler)
	return err
}

//nolint:revive
func check(err error, handleErr func(string, int)) {
	if err == nil {
		return
	}

	debugPrint(err)

	switch {
	case errors.Is(err, ErrExit):
		handleErr("", DefaultErrorExitCode)
	case errors.Is(err, vaulterrors.ErrVaultFileExists):
		handleErr("vlt: vault file already exists\nConsider deleting the file first before running 'create' to create a new vault at the specified path.", DefaultErrorExitCode)
	case errors.Is(err, vaulterrors.ErrVaultFileNotFound):
		handleErr("vlt: "+err.Error()+"\nUse the `create` command to create a new vault file.", DefaultErrorExitCode)
	case errors.Is(err, vaulterrors.ErrWrongPassword):
		handleErr("vlt: incorrect password\nPlease check your password and try again.", DefaultErrorExitCode)
	case errors.Is(err, vaulterrors.ErrNonInteractiveUnsupported):
		handleErr("vlt: this command supports interactive input only.", DefaultErrorExitCode)
	case errors.Is(err, vaulterrors.ErrInteractiveLoginDisabled):
		handleErr("vlt: no login session available and interactive login is disabled\nuse 'vlt login' or remove --no-login-prompt to continue", DefaultErrorExitCode)
	case errors.Is(err, vaultdaemon.ErrSocketUnavailable):
		handleErr("vlt: vault daemon is not running\nStart `vltd` to enable session support", DefaultErrorExitCode)
	default:
		msg, ok := StandardErrorMessage(err)
		if !ok {
			msg = err.Error()
			if !strings.HasPrefix(msg, "vlt: ") {
				msg = "vlt: " + msg
			}
		}

		handleErr(msg, DefaultErrorExitCode)
	}
}

func StandardErrorMessage(_ error) (string, bool) {
	return "", false
}
