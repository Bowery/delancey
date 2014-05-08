// Copyright 2013-2014 Bowery, Inc.
package cmds

import (
	"Bowery/crosswalk/cli/log"
	"Bowery/crosswalk/cli/model"
	"Bowery/crosswalk/cli/opt"
	"Bowery/crosswalk/cli/prompt"
	"Bowery/crosswalk/cli/sync"
	"fmt"
	"net"
	"os"
	"os/signal"
)

func init() {
	Cmds["run"] = &Cmd{runRun, "run", "Run the process."}
}

func runRun(args ...string) int {
	// Register signals.
	signals := make(chan os.Signal, 1)
	done := make(chan int, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)
	defer signal.Stop(signals)

	// Service info.
	address := *opt.AgentAddr
	start := *opt.StartCmd
	path := *opt.Path
	build := *opt.BuildCmd
	test := *opt.TestCmd

	if address == "" || start == "" || build == "" {
		log.Println("yellow", "A few questions about your app")
		if address == "" {
			address, _ = prompt.Basic("Address", true)
		}

		if start == "" {
			start, _ = prompt.Basic("Start Command", true)
		}

		if build == "" {
			build, _ = prompt.Basic("Build Command", true)
		}
	}

	// Create Service
	service := model.Service{
		Address: address + ":3000",
		Commands: map[string]string{
			"build": build,
			"test":  test,
			"start": start,
		},
		Path: path,
	}

	// Attempt to reach server.
	if err := service.Ping(); err != nil {
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
	if err := syncer.Watch(service.Path); err != nil {
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
			case ev := <-syncer.Event:
				if err := service.Update(ev.Path, ev.Status); err != nil {
					done <- 1
				}
			}
		}
	}()

	// Connect to logs
	go func() {
		conn, err := net.Dial("tcp", address+":3002")
		if err != nil {
			done <- 1
		}
		defer conn.Close()
		for {
			log.Println("red", "logs")
		}
	}()

	return <-done
}
