// Copyright 2013-2014 Bowery, Inc.
// Package cmds contains the commands for the cli binary.
package cmds

// Cmds are a map of commands by name.
var Cmds = make(map[string]*Cmd)

// Cmd represents a single command, with a runner, description and usage.
type Cmd struct {
	// The function executed when the user executes the command.
	// Returns an integer representing the exit status.
	Run func(...string) int

	// Command usage.
	// "[]" denotes optional parameter(s)
	// "<>" denotes required parameter(s)
	Usage string

	// Detailed description. Used in help page.
	Description string
}
