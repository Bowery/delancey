// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	VERSION = "1.0.0" // current monitor version
)

var (
	system          *SystemInfo         // host system info.
	apiUrl          string              // api address.
	host            = os.Getenv("HOST") // host address.
	api             = os.Getenv("API")  // api address.
	env             = os.Getenv("ENV")  // mode to run in.
	engTeamContacts = []string{         // phone numbers of team.
		"+17814924545",
	}
)

func main() {
	apiUrl = "http://api.bowery.io"
	if api != "" {
		apiUrl = "http://" + api
	}
	if env == "development" {
		apiUrl += ":3000"
	}

	// Create new System Object.
	system = NewSystemInfo(host)

	// Update API with stats.
	go updateAPIWithStats()

	// Check to see if Docker is installed.
	// Exit if it is not.
	if !isDockerInstalled() {
		log.Fatal("Docker is not installed.")
	}

	go checkDocker()

	// Run server.
	log.Println("Running Monitor on port :3002")

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/healthz", handleHealthCheck)
	http.HandleFunc("/version", handleVersion)
	http.ListenAndServe(":3002", nil)
}

func handleIndex(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "Bowery Monitoring Service")
}

func handleHealthCheck(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "ok")
}

func handleVersion(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, VERSION)
}

func updateAPIWithStats() {
	for {
		// Delay 5 seconds.
		<-time.After(5 * time.Second)
		log.Println("Sending API System Stats...")

		// Encode stats.
		var body bytes.Buffer
		bodyReq := system

		encoder := json.NewEncoder(&body)
		err := encoder.Encode(bodyReq)
		if err != nil {
			log.Println(err)
			continue
		}

		// Send stats if the server is available.
		if system.IsAvailable {
			res, err := http.Post(apiUrl+"/servers", "application/json", &body)
			if err != nil {
				log.Println(err)
				continue
			}

			// Decode response.
			pingRes := new(response)
			decoder := json.NewDecoder(res.Body)
			err = decoder.Decode(pingRes)
			res.Body.Close()
			if err != nil {
				log.Println(err)
			}
		}
	}
}

// Run, inspect, and destroy containers every minute
// to verify that Docker is operational.
func checkDocker() {
	for {
		<-time.After(1 * time.Minute)
		log.Println("Checking Docker...")

		if err := CheckDocker(); err != nil {
			// If there is an error set system as unavailable
			// and notify the eng team.
			system.IsAvailable = false
			SendText("Host ("+host+") down", engTeamContacts)
			continue
		}

		log.Println("Docker is operating as expected")
		system.IsAvailable = true
	}
}

type response struct {
	Status string `json:"status"`
	Err    string `json:"error"`
}
