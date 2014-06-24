// Copyright 2013-2014 Bowery, Inc.
// Contains routines to manage the service.
package main

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

var (
	currentInitCmd  = ""
	currentBuildCmd = ""
	currentTestCmd  = ""
	currentStartCmd = ""
)

// Restart restarts the current service process. A channel is returned
// and signaled when the process is started, or when the build fails.
// The init command is only restarted if hard is true.
func Restart(hard, resetCommands bool, init, build, test, start string) chan bool {
	fmt.Println("Restarting")
	started := make(chan bool, 1)

	app, err := GetApplication()
	if err != nil {
		started <- false
		return started
	}

	service := &Service{}
	for _, s := range app.Services {
		if s.Name == ServiceName {
			service = s
		}
	}

	// Kill previous commands.
	if currentBuildCmd != "" {
		exec.Command("pkill", "-f", currentBuildCmd).Run()
	}
	if currentTestCmd != "" {
		exec.Command("pkill", "-f", currentTestCmd).Run()
	}
	if currentStartCmd != "" {
		exec.Command("pkill", "-f", currentStartCmd).Run()
	}
	if hard && currentInitCmd != "" {
		exec.Command("pkill", "-f", currentInitCmd).Run()
		if resetCommands {
			currentInitCmd = init
		}
	}

	if resetCommands {
		currentBuildCmd = build
		currentTestCmd = test
		currentStartCmd = start
	}

	// Run processes in goroutine so channel reads can occur before the commands
	// are finished.
	go func() {
		var wg sync.WaitGroup
		redis := NewRedis()
		defer redis.Close()

		// Parse commands.
		buildCmd := parseCommand(currentBuildCmd, redis)
		testCmd := parseCommand(currentTestCmd, redis)
		startCmd := parseCommand(currentStartCmd, redis)
		initCmd := parseCommand(currentInitCmd, redis)

		// Run the build process, and only proceed if successful.
		if buildCmd != nil {
			redis.UpdateState(app.ID, service.Name, "building")

			err := buildCmd.Run()
			if err != nil {
				if isCmdErr(err) {
					redis.Write([]byte(err.Error() + "\n"))
				}

				started <- false
				return
			}
		}
		wg.Add(3)

		// Run the tests in a goroutine, writing output to the net writer.
		go func() {
			if testCmd != nil {
				redis.UpdateState(app.ID, service.Name, "testing")

				err := testCmd.Run()
				if isCmdErr(err) {
					redis.Write([]byte(err.Error() + "\n"))
				}
			}

			wg.Done()
		}()

		// Start the init command if it is a hard restart.
		go func() {
			if hard && initCmd != nil {
				// Signal and set state if no start command.
				if startCmd == nil {
					redis.UpdateState(app.ID, service.Name, "running")
					started <- true
				}

				err := initCmd.Run()
				if isCmdErr(err) {
					redis.Write([]byte(err.Error() + "\n"))
				}
			}

			wg.Done()
		}()

		// Start the new process, writing to the net writer.
		go func() {
			if startCmd != nil {
				redis.UpdateState(app.ID, service.Name, "running")
				started <- true

				err := startCmd.Run()
				if isCmdErr(err) {
					redis.Write([]byte(err.Error() + "\n"))
				}
			} else {
				started <- false
			}

			wg.Done()
		}()

		wg.Wait()
	}()

	return started
}

// parseCommand converts a string command to a command, connecting stdio
// a redis connection.
func parseCommand(command string, redis *Redis) *exec.Cmd {
	if command == "" {
		return nil
	}

	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], parts[1:len(parts)]...)
	cmd.Stdout = redis
	cmd.Stderr = redis

	return cmd
}

// isCmdErr checks if an error is a user cmd error such as a non zero exit
// or the command couldn't be found.
func isCmdErr(err error) bool {
	if err == nil {
		return false
	}

	if err == exec.ErrNotFound || strings.Contains(err.Error(), "exit status") {
		return true
	}

	return false
}
