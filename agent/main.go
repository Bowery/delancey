// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"Bowery/crosswalk/agent/routes"
	"runtime"

	server "github.com/go-martini/martini"
)

func main() {
	runtime.GOMAXPROCS(1)

	s := server.Classic()
	s.Post("/", routes.HandleNewService)
	s.Put("/", routes.HandleUpdateService)
	s.Get("/", routes.HandleGetService)
	s.Get("/ping", routes.HandlePingService)

	s.Run()
}
