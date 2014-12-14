// Copyright 2013-2014 Bowery, Inc.
// Contains the main entry point, service handling, and file watching.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/docker"
	"github.com/Bowery/gopackages/util"
	"github.com/Bowery/gopackages/web"
	loggly "github.com/segmentio/go-loggly"
)

var (
	AgentHost, _ = util.GetHost()
	DockerClient *docker.Client
	Env          string
	VERSION      string // This is set when release_agent.sh is ran.
	logClient    = loggly.New(config.LogglyKey, "agent")
	err          error
)

func main() {
	ver := false
	runtime.GOMAXPROCS(1)
	flag.StringVar(&Env, "env", "production", "If you want to run the agent in development mode uses different ports")
	flag.BoolVar(&ver, "version", false, "Print the version")
	flag.Parse()
	if ver {
		fmt.Println(VERSION)
		os.Exit(0)
	}

	DockerClient, err = docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	err = DockerClient.PullImage(config.DockerBaseImage + ":latest")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

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