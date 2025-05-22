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
	// fatalErrHandler is the function used to handle fatal errors.
	// By default, it calls os.Exit(1).
	fatalErrHandler = fatal

	// fprintf is the function used to format and print errors.
	fprintf = fmt.Fprintf

	// debugMode enables always printing raw error values.
	debugMode bool
)

// BehaviorOnFatal allows overriding the default behavior when a fatal
// error occurs. By default, it calls os.Exit(1).
func BehaviorOnFatal(f func(string, int)) {
	fatalErrHandler = f
}

// DefaultBehaviorOnFatal restores the default behavior for fatal errors,
// which is to call os.Exit(1).
//
// Useful for tests.
func DefaultBehaviorOnFatal() {
	fatalErrHandler = fatal
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

// fatal prints the message provided and then exits with the given code.
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

func debugPrint(err error) {
	if !debugMode {
		return
	}

	_, _ = fprintf(os.Stderr, "debug: %+v\n", err)
}

// ErrExit may be passed to CheckError to instruct it to output nothing but exit with
// status code 1.
var ErrExit = errors.New("exit")

// Check prints a user friendly error and exits with a non-zero
// exit code. Unrecognized errors will be printed with an "error: " prefix.
func Check(err error) {
	check(err, fatalErrHandler)
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
