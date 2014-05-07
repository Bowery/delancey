// Copyright 2013-2014 Bowery, Inc.
package cmds

import (
	"Bowery/crosswalk/cli/log"
	"Bowery/crosswalk/cli/model"
	"Bowery/crosswalk/cli/sync"
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
	var address, path, build, test, start string

	// Config.
	var config Config

	// If no file provided prompt user for info.
	if !providedFile {
		log.Println("yellow", "A few questions about your app")
		address, err = prompt.Basic("Address", true)
		if err != nil {
			return 1
		}

		path, err = prompt.BasicDefault("Path", ".")
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
				Path: path,
			},
		}

		// Marshal config.
		data, err := goyaml.Marshal(config.Service)
		if err != nil {
			log.Println("red", fmt.Sprintf("Error: %s", err.Error()))
			return 1
		}

		// Write config to .yaml file.
		if err = ioutil.WriteFile("crosswalk.yaml", data, 0644); err != nil {
			log.Println("red", fmt.Sprintf("Error: %s", err.Error()))
			return 1
		}
	} else {
		// Unmarshal .yaml config.
		if err := goyaml.Unmarshal(data, &config.Service); err != nil {
			log.Println("yellow", "Invalid YAML.")
			return 1
		}
	}

	// Add sync port to service address.
	service := config.Service
	service.Address += ":3000"

	// Attempt to reach server.
	if err = service.Ping(); err != nil {
		log.Println("red", err.Error())
		log.Println("yellow", "Unable to connect to "+service.Address+". Make sure you've entered a valid address")
		log.Println("yellow", "and that port 3000 is publically accessible.")
		return 1
	}

	// Display runtime info to user.
	log.Println("cyan", fmt.Sprintf("Syncing file changes between your local machine and %s", service.Address))
	log.Println("")
	if service.Commands["build"] != "" {
		log.Println("cyan", fmt.Sprintf("  - Build: %s", service.Commands["build"]))
	}
	if service.Commands["test"] != "" {
		log.Println("cyan", fmt.Sprintf("  - Test:  %s", service.Commands["test"]))
	}
	if service.Commands["start"] != "" {
		log.Println("cyan", fmt.Sprintf("  - Start: %s", service.Commands["start"]))
	}
	log.Println("")

	// Initiate sync.
	syncer := sync.NewSyncer()
	if err = syncer.Watch(service.Path); err != nil {
		log.Println("red", err.Error())
		return 1
	}
	defer syncer.Close()

	// Exit on signal.
	go func() {
		<-signals
		done <- 0
	}()

	// Run initial upload and watch for events.
	go func() {
		if err := service.Upload(); err != nil {
			done <- 1
		}
		for {
			select {
			case service := <-syncer.Upload:
				log.Println("red", service)
			case ev := <-syncer.Event:
				log.Println("", ev)
				if err := service.Update(); err != nil {
					done <- 1
				}
			}
		}
	}()

	return <-done
}
