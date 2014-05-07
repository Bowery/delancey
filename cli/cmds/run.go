// Copyright 2013-2014 Bowery, Inc.
package cmds

import (
	"Bowery/crosswalk/cli/log"
	"Bowery/crosswalk/cli/model"
	"cli/prompt"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"

	"launchpad.net/goyaml"
)

type Config struct {
	Service model.Service
}

func init() {
	Cmds["run"] = &Cmd{runRun, "run", "Run the process."}
}

func runRun(args ...string) int {
	// Register signals.
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

	// Service info.
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
			Service: model.Service{
				Address: address,
				Commands: map[string]string{
					"build": build,
					"test":  test,
					"start": start,
				},
			},
		}

		data, err := goyaml.Marshal(config.Service)
		if err != nil {
			log.Println("red", fmt.Sprintf("Error: %s", err.Error()))
			return 1
		}

		if err = ioutil.WriteFile("crosswalk.yaml", data, 0644); err != nil {
			log.Println("red", fmt.Sprintf("Error: %s", err.Error()))
			return 1
		}
	} else {
		if err := goyaml.Unmarshal(data, &config.Service); err != nil {
			log.Println("yellow", "Invalid YAML.")
			return 1
		}
	}

	// Add sync port to service address.
	service := config.Service
	service.Address += ":3001"

	// Attempt to reach server.
	if err = service.Ping(); err != nil {
		log.Println("yellow", "Unable to connect to "+service.Address+". Make sure you've entered a valid address")
		log.Println("yellow", "and that port 3001 is publically accessible.")
		return 1
	}

	log.Println("cyan", fmt.Sprintf("Syncing file changes between your local machine and %s", service.Address))
	log.Println("")
	log.Println("cyan", fmt.Sprintf("  - Build: %s", service.Commands["build"]))
	log.Println("cyan", fmt.Sprintf("  - Test:  %s", service.Commands["test"]))
	log.Println("cyan", fmt.Sprintf("  - Start: %s", service.Commands["start"]))
	log.Println("")
	log.Println("cyan", "Logs will tail here...")

	go func() {
		<-signals
		done <- 0
	}()

	return <-done
}
