// Copyright 2013-2014 Bowery, Inc.
package cmds

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"

	"launchpad.net/goyaml"
)

func init() {
	Cmds["run"] = &Cmd{runRun, "run", "Run the process."}
}

func runRun(args ...string) int {
	// Register signals
	signals := make(chan os.Signal, 1)
	done := make(chan int, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)
	defer signal.Stop(signals)

	// Read in .yaml file.
	data, err := ioutil.ReadFile("crosswalk.yaml")
	if err != nil {
		return 1
	}

	// Unmarshal the .yaml configuration.
	var config interface{}
	if err := goyaml.Unmarshal(data, &config); err != nil {
		log.Println("Invalid YAML.")
		return 1
	}

	log.Println(fmt.Sprintf("%s", config))

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

	go func() {
		<-signals
		done <- 0
	}()

	return <-done
}
