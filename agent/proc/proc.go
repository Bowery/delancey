// Copyright 2014 Bowery, Inc.
package proc

import (
	"Bowery/crosswalk/agent/opt"
	"Bowery/crosswalk/agent/pubsub"
	"os/exec"
	"strings"
	"sync"
)

func Restart(build, test, start string) chan bool {
	started := make(chan bool, 1)
	writer := pubsub.NewNetWriter()

	// Kill previous commands
	if build != "" {
		exec.Command("pkill", "-f", build).Run()
	}
	if test != "" {
		exec.Command("pkill", "-f", test).Run()
	}
	if start != "" {
		exec.Command("pkill", "-f", start).Run()
	}

	// Run processes in goroutine so the request doesn't wait.
	go func() {
		var wg sync.WaitGroup
		var buildCmd, testCmd, startCmd *exec.Cmd

		// Parse commands.
		if build != "" {
			buildCmd = parseCommand(build)
			buildCmd.Dir = *opt.TargetDir
		}
		if test != "" {
			testCmd = parseCommand(test)
			testCmd.Dir = *opt.TargetDir
		}
		if start != "" {
			startCmd = parseCommand(start)
			startCmd.Dir = *opt.TargetDir
		}

		// Run the build process, and only proceed if successful.
		if buildCmd != nil {
			buildCmd.Stdout = writer
			buildCmd.Stderr = writer
			if err := buildCmd.Run(); err != nil {
				started <- false
				return
			}
		}
		wg.Add(2)

		go func() {
			if testCmd != nil {
				testCmd.Stdout = writer
				testCmd.Stderr = writer
				testCmd.Run()
			}
			wg.Done()
		}()

		go func() {
			if startCmd != nil {
				startCmd.Stdout = writer
				startCmd.Stderr = writer
				started <- true
				startCmd.Run()
			} else {
				started <- false
			}
			wg.Done()
		}()

		wg.Wait()
	}()

	return started
}

// Convert string to command
func parseCommand(command string) *exec.Cmd {
	parts := strings.Fields(command)
	head := parts[0]
	parts = parts[1:len(parts)]

	return exec.Command(head, parts...)
}
