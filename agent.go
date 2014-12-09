// Copyright 2013-2014 Bowery, Inc.
// Contains the main entry point, service handling, and file watching.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/Bowery/delancey/plugin"
	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/util"
	"github.com/Bowery/gopackages/web"
	loggly "github.com/segmentio/go-loggly"
)

var (
	AgentHost     string
	Env           string
	VERSION       string // This is set when release_agent.sh is ran.
	InDevelopment = false
	Applications  = map[string]*Application{}
	logClient     = loggly.New(config.LogglyKey, "agent")
)

func main() {
	ver := false
	runtime.GOMAXPROCS(1)
	flag.StringVar(&Env, "env", "production", "If you want to run the agent in development mode uses different ports")
	flag.BoolVar(&ver, "version", false, "Print the version")
	flag.Parse()
	if Env == "development" {
		InDevelopment = true
	}
	if ver {
		fmt.Println(VERSION)
		return
	}

	// Get host
	AgentHost, _ = util.GetHost()

	port := config.BoweryAgentProdSyncPort
	if InDevelopment {
		port = config.BoweryAgentDevSyncPort
	}

	// Start plugin management.
	go plugin.StartPluginListener()

	// Add saved applications.
	LoadApps()
	for _, app := range Applications {
		<-Restart(app, true, true)
	}

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
