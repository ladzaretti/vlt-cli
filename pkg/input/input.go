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

// IsPiped checks if stdin is piped.
func IsPiped() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// ReadTrimStdin reads and trims input from stdin.
func ReadTrimStdin() (string, error) {
	bs, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read from stdin: %v", err)
	}

	return strings.TrimSpace(string(bs)), nil
}

// ReadSecure prompts for input and reads it securely (hides input).
func ReadSecure(prompt string) (string, error) {
	fmt.Println(prompt)

	pb, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("term read password: %v", err)
	}

	return string(pb), nil
}

// PromptPassword asks for the current password.
func PromptPassword() (string, error) {
	return ReadSecure("Enter password: ")
}

// PromptNewPassword asks for a new password with confirmation.
func PromptNewPassword() (string, error) {
	pass := ""

	for len(pass) < minPasswordLen {
		p, err := ReadSecure("Enter new password: ")
		if err != nil {
			return "", fmt.Errorf("read password: %v", err)
		}

		pass = p

		if len(pass) < minPasswordLen {
			fmt.Printf("Password must be at least %d characters. Please try again.\n", minPasswordLen)
		}
	}

	pass2, err := ReadSecure("Retype password: ")
	if err != nil {
		return "", fmt.Errorf("read password: %v", err)
	}

	if pass2 != pass {
		fmt.Println("Passwords do not match. Please try again.")
		return "", errors.New("user password mismatch")
	}

	return pass, nil
}
