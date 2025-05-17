package main

import (
	"os"

	"github.com/ladzaretti/vlt-cli/cli"
	"github.com/ladzaretti/vlt-cli/genericclioptions"
	"github.com/ladzaretti/vlt-cli/vaultdaemon"
)

func main() {
	// FIXME: remove dummy driver
	_ = vaultdaemon.Client()

	iostream := genericclioptions.NewDefaultIOStreams()
	vlt := cli.NewDefaultVltCommand(iostream, os.Args[1:])
	_ = vlt.Execute()
}
