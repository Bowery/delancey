// Copyright 2013-2014 Bowery, Inc.
package main

import (
	. "Bowery/crosswalk/cli/cmds"
	"flag"
	"log"
	"os"
)

func main() {
	// Parse flags and get arguments.
	command := "run"
	commandArgs := flag.Args()

	if len(commandArgs) >= 1 {
		command = commandArgs[0]
	}

	// Run command, and handle invalid commands.
	cmd, ok := Cmds[command]
	if !ok {
		log.Println("Invalid command.")
		os.Exit(1)
	}
	os.Exit(cmd.Run())
}
