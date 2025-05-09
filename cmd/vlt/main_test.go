package main_test

import (
	"fmt"
	"testing"

	"github.com/ladzaretti/vlt-cli/vlt"
)

// https://github.com/spf13/cobra/issues/1419
// https://github.com/cli/cli/blob/c0c28622bd62b273b32838dfdfa7d5ffc739eeeb/command/pr_test.go#L55-L67
func TestMain(t *testing.T) {
	b := true

	if !b {
		t.Error("Dummy test")
	}

	v, err := vlt.New(t.Context(), "password", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	_, err = v.InsertNewSecret(t.Context(), "name", "secret", []string{"label1", "label2"})
	if err != nil {
		t.Error(err)
	}

	m, err := v.ExportSecrets(t.Context())
	if err != nil {
		t.Error(err)
	}

	fmt.Printf("%v", m)

	err = v.Seal(t.Context())
	if err != nil {
		t.Error(err)
	}
}
