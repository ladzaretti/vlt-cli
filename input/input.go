package input

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

func IsPipedOrRedirected(fi os.FileInfo) bool {
	return (fi.Mode() & os.ModeCharDevice) == 0
}

// ReadTrim reads and trims input from r.
func ReadTrim(r io.Reader) (string, error) {
	bs, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read trim: %w", err)
	}

	return strings.TrimSpace(string(bs)), nil
}

// PromptRead prompts via w for input and reads it from r until a newline is entered.
func PromptRead(w io.Writer, r io.Reader, prompt string, a ...any) (string, error) {
	fmt.Fprintf(w, prompt, a...)

	reader := bufio.NewReader(r)

	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("prompt read: %w", err)
	}

	return strings.TrimSpace(line), nil
}

// PromptReadSecure prompts the user via w for input and securely reads it
// from the given file descriptor.
func PromptReadSecure(w io.Writer, fd int, prompt string, a ...any) (string, error) {
	fmt.Fprintf(w, prompt, a...)
	defer fmt.Println()

	bs, err := term.ReadPassword(fd)
	if err != nil {
		return "", fmt.Errorf("term read password: %w", err)
	}

	return strings.TrimSpace(string(bs)), nil
}

// PromptPassword prompts the user to enter the current password securely.
// The prompt is displayed via the writer w, and input is read from the
// given file descriptor fd.
func PromptPassword(w io.Writer, fd int) (string, error) {
	return PromptReadSecure(w, fd, "Enter password: ")
}

// PromptNewPassword prompts the user to enter a new password of the specified length.
// The prompt is displayed via the writer w, and input is read from the given file descriptor fd.
func PromptNewPassword(w io.Writer, fd int, length int) (string, error) {
	pass := ""

	for len(pass) < length {
		p, err := PromptReadSecure(w, fd, "Enter new password: ")
		if err != nil {
			return "", fmt.Errorf("prompt new password: %w", err)
		}

		pass = p

		if len(pass) < length {
			fmt.Fprintf(w, "Password must be at least %d characters. Please try again.\n", length)
		}
	}

	pass2, err := PromptReadSecure(w, fd, "Retype password: ")
	if err != nil {
		return "", fmt.Errorf("prompt new password: %w", err)
	}

	if pass2 != pass {
		fmt.Fprintln(w, "Passwords do not match. Please try again.")
		return "", errors.New("prompt new password: passwords do not match")
	}

	return pass, nil
}
