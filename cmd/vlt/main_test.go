package main_test

import (
	"testing"

	"github.com/ladzaretti/vlt-cli/randstring"
)

// https://github.com/spf13/cobra/issues/1419
// https://github.com/cli/cli/blob/c0c28622bd62b273b32838dfdfa7d5ffc739eeeb/command/pr_test.go#L55-L67
func TestMain(t *testing.T) {
	b := true

	policy := randstring.PasswordPolicy{
		MinLowercase: 1,
		MinUppercase: 1,
		MinDigits:    1,
		MinSymbols:   1,
		MinLength:    4,
	}

	s, err := randstring.NewWithPolicy(policy)

	_, _ = s, err

	if !b {
		t.Error("Dummy test")
	}
}
