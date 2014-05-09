// Copyright 2014 Bowery, Inc.
package main

import (
	"Bowery/crosswalk/agent/pubsub"
	"Bowery/crosswalk/agent/routes"
	"runtime"

	server "github.com/go-martini/martini"
	"github.com/martini-contrib/render"
)

func main() {
	runtime.GOMAXPROCS(1)

	// Create new server.
	s := server.Classic()

	// Middleware.
	s.Use(render.Renderer())

	// Set routes.
	s.Post("/", routes.HandleNewService)
	s.Put("/", routes.HandleUpdateService)
	s.Get("/", routes.HandleGetService)
	s.Get("/ping", routes.HandlePingService)

	// Start pubsub server.
	go pubsub.Run()

	// Run Server.
	s.Run()
}
