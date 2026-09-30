// Command termcade runs the arcade SSH server and manages its installation.
package main

import (
	"fmt"
	"os"
	"strings"

	"termcade/internal/version"
)

const usage = `termcade: a tiny multiplayer arcade you play over SSH

Usage:
  termcade [serve] [flags]   run the arcade server (the default)
  termcade update [flags]    install the newest release and restart the service
  termcade install [flags]   set up termcade as a systemd service (Linux, root)
  termcade version           print the version

Run "termcade <command> -h" for the flags of a command.
`

func main() {
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}

	var err error
	switch cmd {
	case "serve":
		err = serveCmd(args)
	case "update":
		err = updateCmd(args)
	case "install":
		err = installCmd(args)
	case "version":
		fmt.Println("termcade", version.String())
	case "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fail(err)
	}
}
