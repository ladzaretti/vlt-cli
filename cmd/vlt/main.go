package main

import (
	"os"

	"github.com/ladzaretti/vlt-cli/cli"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
)

func main() {
	iostream := genericclioptions.NewDefaultIOStreams()
	vlt := cli.NewDefaultVltCommand(iostream, os.Args[1:])
	_ = vlt.Execute()
}
