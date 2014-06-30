// Copyright 2013-2014 Bowery, Inc.
// Contains routines to manage the service.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var (
	ImageScriptsDir = "/image_scripts"
	mutex           sync.Mutex
	cmdStrs         = [4]string{} // Order: init, build, test start.
	// Only assign here if the process has been started.
	prevInitCmd *exec.Cmd
)

// Proc describes a processes ids and its children.
type Proc struct {
	Pid      int
	Ppid     int
	Children []*Proc
}

// Kill kills the proc and its children.
func (proc *Proc) Kill() error {
	p, err := os.FindProcess(proc.Pid)
	if err != nil {
		return err
	}

	err = p.Kill()
	if err != nil {
		return err
	}

	if proc.Children != nil {
		for _, p := range proc.Children {
			err = p.Kill()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Restart restarts the services processes, the init cmd is only restarted
// if initReset is true. Commands to run are only updated if reset is true.
// A channel is returned and signaled if the commands start or the build fails.
func Restart(initReset, reset bool, init, build, test, start string) chan bool {
	mutex.Lock() // Lock here so no other restarts can interfere.
	finish := make(chan bool, 1)
	redis := NewRedis()
	fmt.Println("Restarting")

	err := killCmds(initReset)
	if err != nil {
		mutex.Unlock()
		redis.Close()
		finish <- false
		return finish
	}

	// Create cmds.
	if reset {
		if !initReset {
			init = cmdStrs[0]
		}

		cmdStrs = [4]string{init, build, test, start}
	}
	buildCmd := parseCmd(cmdStrs[1], redis)
	testCmd := parseCmd(cmdStrs[2], redis)
	startCmd := parseCmd(cmdStrs[3], redis)
	initCmd := parseCmd(cmdStrs[0], redis)

	// Run in goroutine so commands can run in the background with the redis
	// connection open.
	go func() {
		var wg sync.WaitGroup
		defer redis.Close()
		defer wg.Wait()
		defer mutex.Unlock()
		cmds := make([]*exec.Cmd, 0)

		// Get the image_scripts and start them.
		scriptPath := filepath.Join(ImageScriptsDir)
		dir, _ := os.Open(scriptPath)
		if dir != nil {
			infos, _ := dir.Readdir(0)
			if infos != nil {
				for _, info := range infos {
					if info.IsDir() {
						continue
					}

					cmd := parseCmd(filepath.Join(scriptPath, info.Name()), redis)
					if cmd != nil {
						err := startProc(cmd, redis)
						if err == nil {
							cmds = append(cmds, cmd)
						}
					}
				}
			}

			dir.Close()
		}

		// Run the build command, only proceed if successful.
		if buildCmd != nil {
			redis.UpdateState("building")

			err := buildCmd.Run()
			if err != nil {
				redis.Write([]byte(err.Error() + "\n"))

				killCmds(initReset)
				finish <- false
				return
			}
		}

		// Start the test command.
		if testCmd != nil {
			err := startProc(testCmd, redis)
			if err == nil {
				cmds = append(cmds, testCmd)
				redis.UpdateState("testing")
			}
		}

		// Start the init command if init is set.
		if initReset && initCmd != nil {
			err := startProc(initCmd, redis)
			if err == nil {
				prevInitCmd = initCmd
				redis.UpdateState("running")
			}
		}

		// Start the start command.
		if startCmd != nil {
			err := startProc(startCmd, redis)
			if err == nil {
				cmds = append(cmds, startCmd)
				redis.UpdateState("running")
			}
		}

		// Signal the start and prepare the wait group to keep redis open.
		finish <- true
		wg.Add(len(cmds))

		// Wait for the init process to end
		if initReset && prevInitCmd != nil {
			wg.Add(1)

			go func(c *exec.Cmd) {
				waitProc(c, redis)
				wg.Done()
			}(prevInitCmd)
		}

		// Loop the commands and wait for them in parallel.
		for _, cmd := range cmds {
			go func(c *exec.Cmd) {
				waitProc(c, redis)
				wg.Done()
			}(cmd)
		}

		fmt.Println("Restart complete")
	}()

	return finish
}

// waitProc waits for a process to end and writes errors to redis.
func waitProc(cmd *exec.Cmd, redis *Redis) {
	err := cmd.Wait()
	if err != nil {
		redis.Write([]byte(err.Error() + "\n"))
	}
}

// startProc starts a process and writes errors to redis.
func startProc(cmd *exec.Cmd, redis *Redis) error {
	err := cmd.Start()
	if err != nil {
		redis.Write([]byte(err.Error() + "\n"))
	}

	return err
}

// killCmds kills the running processes and resets them, the init cmd
// is only killed if init is true.
func killCmds(init bool) error {
	proc, err := GetPidTree(os.Getpid())
	if err != nil {
		return nil
	}

	for _, proc := range proc.Children {
		// Manage the previous init command properly.
		if prevInitCmd != nil && prevInitCmd.Process != nil &&
			prevInitCmd.Process.Pid == proc.Pid {
			if init {
				prevInitCmd = nil
			} else {
				continue
			}
		}

		err = proc.Kill()
		if err != nil {
			return err
		}
	}

	return nil
}

// parseCmd converts a string to a command, connecting stdio to a
// redis connection.
func parseCmd(command string, redis *Redis) *exec.Cmd {
	if command == "" {
		return nil
	}

	parts := strings.Fields(command)
	cmd := exec.Command(parts[0], parts[1:len(parts)]...)
	cmd.Stdout = redis
	cmd.Stderr = redis

	return cmd
}
