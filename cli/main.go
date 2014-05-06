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
	flag.Parse()
	args := flag.Args()
	command := "help"

	if len(args) >= 1 {
		command = args[0]
		args = args[1:]
	}

	// Run command, and handle invalid commands.
	cmd, ok := Cmds[command]
	if !ok {
		log.Println("Invalid command.")
		os.Exit(1)
	}
	os.Exit(cmd.Run(args...))
}
