package main

import (
	"os"

	"github.com/ladzaretti/vlt-cli/pkg/cmd"
	"github.com/ladzaretti/vlt-cli/pkg/genericclioptions"
)

func main() {
	iostream := genericclioptions.NewDefaultIOStreams()
	vlt := cmd.NewDefaultVltCommand(iostream, os.Args[1:])
	_ = vlt.Execute()
}
