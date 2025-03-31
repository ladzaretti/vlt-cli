package main

import (
	"log"

	"github.com/ladzaretti/vlt-cli/internal/cmd"
)

func main() {
	if err := cmd.MustInitialize(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	if err := cmd.Execute(); err != nil {
		log.Fatalf("Command execution failed: %v", err)
	}
}
