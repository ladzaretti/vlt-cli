package main_test

import (
	"testing"
)

// https://github.com/spf13/cobra/issues/1419
// https://github.com/cli/cli/blob/c0c28622bd62b273b32838dfdfa7d5ffc739eeeb/command/pr_test.go#L55-L67
func TestMain(t *testing.T) {
	b := true

	if !b {
		t.Error("Dummy test")
	}
}
