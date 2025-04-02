package main

import (
	"log"

	"github.com/ladzaretti/vlt-cli/pkg/cmd"
)

func main() {
	if err := cmd.NewDefaultVltCommand().Execute(); err != nil {
		log.Fatalf("command execution failed: %v", err)
	}
}
