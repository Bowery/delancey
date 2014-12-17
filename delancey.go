// Copyright 2013-2014 Bowery, Inc.
// Contains the main entry point, service handling, and file watching.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/docker"
	"github.com/Bowery/gopackages/util"
	"github.com/Bowery/gopackages/web"
	loggly "github.com/segmentio/go-loggly"
)

var (
	AgentHost, _ = util.GetHost()
	logClient    = loggly.New(config.LogglyKey, "agent")
	DockerClient *docker.Client
	dockerAddr   string
	Env          string
	VERSION      string // This is set when release_agent.sh is ran.
	err          error
)

func main() {
	ver := false
	runtime.GOMAXPROCS(1)
	flag.StringVar(&dockerAddr, "docker", "unix:///var/run/docker.sock", "Set a custom endpoint for your local Docker service")
	flag.StringVar(&Env, "env", "production", "If you want to run the agent in development mode uses different ports")
	flag.BoolVar(&ver, "version", false, "Print the version")
	flag.Parse()
	if ver {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	fmt.Println("Starting up Delancey with Docker at", dockerAddr)
	DockerClient, err = docker.NewClient(dockerAddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Pull down base image with latest tag, and within a
	// routine, pull down all tags. This is a temporary, and
	// semi hacky, solution to speed up provision times.
	err = DockerClient.PullImage(config.DockerBaseImage + ":latest")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	go func() {
		for {
			<-time.After(10 * time.Second)
			DockerClient.PullImage(config.DockerBaseImage)
		}
	}()

	port := config.DelanceyProdPort
	if Env == "development" {
		port = config.DelanceyDevPort
	}
	LoadContainer()

	go logClient.Info("agent starting", map[string]interface{}{
		"version": VERSION,
		"arch":    runtime.GOARCH,
		"os":      runtime.GOOS,
		"ip":      AgentHost,
	})

	server := web.NewServer(":"+port, []web.Handler{
		new(web.SlashHandler),
	}, Routes)
	server.AuthHandler = &web.AuthHandler{Auth: web.DefaultAuthHandler}

	err := server.ListenAndServe()
	if err != nil {
		go logClient.Error(err.Error(), map[string]interface{}{
			"ip": AgentHost,
		})
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
