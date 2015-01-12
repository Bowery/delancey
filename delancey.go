// Copyright 2013-2014 Bowery, Inc.

package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/docker"
	"github.com/Bowery/gopackages/docker/quay"
	"github.com/Bowery/gopackages/util"
	"github.com/Bowery/gopackages/web"
	loggly "github.com/segmentio/go-loggly"
)

// Runtime info and clients.
var (
	agentHost, _ = util.GetHost()
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

	err = DockerClient.PullImage(config.DockerBaseImage + ":latest")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Pull down each tag in parallel.
	go func() {
		tags, err := quay.TagList(DockerClient.Auth, config.DockerBaseImage)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		delete(tags, "latest") // Already pulled at this point.
		var wg sync.WaitGroup

		for tag := range tags {
			wg.Add(1)
			go func(t string) {
				defer wg.Done()
				err := io.EOF

				// Keep trying to pull it until successful.
				for err != nil {
					err = DockerClient.PullImage(config.DockerBaseImage + ":" + t)
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
					}
				}
			}(tag)
		}

		wg.Wait()
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
		"ip":      agentHost,
	})

	server := web.NewServer(":"+port, []web.Handler{
		new(web.SlashHandler),
	}, Routes)
	server.AuthHandler = &web.AuthHandler{Auth: web.DefaultAuthHandler}

	err := server.ListenAndServe()
	if err != nil {
		go logClient.Error(err.Error(), map[string]interface{}{
			"ip": agentHost,
		})
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
