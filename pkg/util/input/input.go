package input

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

const (
	minPasswordLen = 8
)

// IsPiped checks if the given file is a pipe (not a character device).
func IsPiped(fi os.FileInfo) bool {
	return (fi.Mode() & os.ModeCharDevice) == 0
}

// ReadTrim reads and trims input from r.
func ReadTrim(r io.Reader) (string, error) {
	bs, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read from stdin: %v", err)
	}

	return strings.TrimSpace(string(bs)), nil
}

// ReadSecure prompts for input and reads it securely (hides input).
func ReadSecure(fd int, prompt string, a ...any) (string, error) {
	fmt.Printf(prompt, a...)
	defer fmt.Println()

	pb, err := term.ReadPassword(fd)
	if err != nil {
		return "", fmt.Errorf("term read password: %v", err)
	}

	return string(pb), nil
}

// PromptPassword asks for the current password.
func PromptPassword(fd int) (string, error) {
	return ReadSecure(fd, "Enter password: ")
}

// PromptNewPassword asks for a new password with confirmation.
func PromptNewPassword(fd int) (string, error) {
	pass := ""

	for len(pass) < minPasswordLen {
		p, err := ReadSecure(fd, "Enter new password: ")
		if err != nil {
			return "", fmt.Errorf("read password: %v", err)
		}

		pass = p

		if len(pass) < minPasswordLen {
			fmt.Printf("Password must be at least %d characters. Please try again.\n", minPasswordLen)
		}
	}

	pass2, err := ReadSecure(fd, "Retype password: ")
	if err != nil {
		return "", fmt.Errorf("read password: %v", err)
	}

	if pass2 != pass {
		fmt.Println("Passwords do not match. Please try again.")
		return "", errors.New("user password mismatch")
	}

	return pass, nil
}
