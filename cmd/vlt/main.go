package main

import (
	"github.com/ladzaretti/vlt-cli/internal/cmd"
)

func main() {
	_ = cmd.MustInitialize()
	cmd.Execute()
}
