package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ladzaretti/vlt-cli/vaultdaemon"
)

func main() {
	help := flag.Bool("help", false, "Show usage information")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), `vltd - background daemon for the 'vlt' cli.
		
Usage: vltd [options]

Manages user sessions for the 'vlt' cli.
Runs over a UNIX socket at /run/user/$UID/vlt.sock and takes no arguments.

Options:
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	vaultdaemon.Run()
}
