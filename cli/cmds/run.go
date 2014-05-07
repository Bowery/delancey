// Copyright 2013-2014 Bowery, Inc.
package cmds

import (
	"Bowery/crosswalk/cli/log"
	"cli/prompt"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"

	"launchpad.net/goyaml"
)

type Config struct {
	Address  string            `address`
	Commands map[string]string `commands`
}

func init() {
	Cmds["run"] = &Cmd{runRun, "run", "Run the process."}
}

func runRun(args ...string) int {
	// Register signals
	signals := make(chan os.Signal, 1)
	done := make(chan int, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)
	defer signal.Stop(signals)

	// Attempt to read in .yaml file.
	providedFile := true
	data, err := ioutil.ReadFile("crosswalk.yaml")
	if err != nil {
		providedFile = false
	}

	// Required info.
	var address, build, test, start string

	// Config.
	var config Config

	// If no file provided prompt user for info.
	if !providedFile {
		log.Println("yellow", "A few questions about your app")
		address, err = prompt.Basic("Address", true)
		if err != nil {
			return 1
		}

		build, err = prompt.Basic("Build command", false)
		if err != nil {
			return 1
		}

		test, err = prompt.Basic("Test command", false)
		if err != nil {
			return 1
		}

		start, err = prompt.Basic("Start command", false)
		if err != nil {
			return 1
		}

		config = Config{
			Address: address,
			Commands: map[string]string{
				"build": build,
				"test":  test,
				"start": start,
			},
		}

		data, err := goyaml.Marshal(config)
		if err != nil {
			log.Println("red", fmt.Sprintf("Error: %s", err.Error()))
			return 1
		}

		if err = ioutil.WriteFile("crosswalk.yaml", data, 0644); err != nil {
			log.Println("red", fmt.Sprintf("Error: %s", err.Error()))
			return 1
		}
	} else {
		if err := goyaml.Unmarshal(data, &config); err != nil {
			log.Println("yellow", "Invalid YAML.")
			return 1
		}
	}

	// Step 1. If it's run locally, just run the command as you would otherwise.
	//
	// Step 2. If it's being run online, ping the machines to verify that
	//         the agent is running. If it can't let the user know they need
	//         to download the agent and that port x needs to be exposed on
	//         the machine.
	//
	// Step 3. Initiate file watching based on the configuration and send
	//         appropriate http requests as needed.
	//
	// Step 4. Alert the user of the addresses of the services.

	log.Println("cyan", fmt.Sprintf("Syncing file changes between your local machine and %s", config.Address))
	log.Println("")
	log.Println("cyan", fmt.Sprintf("  - Build: %s", config.Commands["build"]))
	log.Println("cyan", fmt.Sprintf("  - Test:  %s", config.Commands["test"]))
	log.Println("cyan", fmt.Sprintf("  - Start: %s", config.Commands["start"]))
	log.Println("")
	log.Println("cyan", "Logs will tail here...")

	go func() {
		<-signals
		done <- 0
	}()

	return <-done
}
