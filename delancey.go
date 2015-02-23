// Copyright 2013-2014 Bowery, Inc.

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
	"github.com/timonv/pusher"
)

// Runtime info and clients.
var (
	agentHost, _ = util.GetHost()
	logClient    = loggly.New(config.LogglyKey, "agent")
	pusherC      *pusher.Client
	DockerClient *docker.Client
	dockerAddr   string
	Env          string
	VERSION      string // This is set when release_agent.sh is ran.
)

func main() {
	var err error
	ver := false
	runtime.GOMAXPROCS(1)
	pusherC = pusher.NewClient(config.PusherAppID, config.PusherKey, config.PusherSecret)
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

	err = DockerClient.PullImage(config.DockerBaseImage+":latest", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	currentContainer, _ = LoadContainer()

	go logClient.Info("agent starting", map[string]interface{}{
		"version": VERSION,
		"arch":    runtime.GOARCH,
		"os":      runtime.GOOS,
		"ip":      agentHost,
	})

	port := config.DelanceyProdPort
	if Env == "development" {
		port = config.DelanceyDevPort
	}

	server := web.NewServer(":"+port, []web.Handler{
		new(web.SlashHandler),
	}, Routes)
	server.AuthHandler = &web.AuthHandler{Auth: web.DefaultAuthHandler}

	err = server.ListenAndServe()
	if err != nil {
		go logClient.Error(err.Error(), map[string]interface{}{
			"ip": agentHost,
		})
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
