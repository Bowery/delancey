// Copyright 2013-2014 Bowery, Inc.
// Contains the main entry point, service handling, and file watching.
package main

import (
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/gorilla/mux"
)

var (
	DeveloperToken = os.Getenv("DEVELOPER")
	err            error
)

func main() {
	runtime.GOMAXPROCS(1)

	err := os.MkdirAll(ServiceDir, os.ModePerm|os.ModeDir)
	if err == nil {
		err = os.Chdir(ServiceDir)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if ApplicationID == "" || DeveloperToken == "" {
		fmt.Fprintln(os.Stderr, ErrAppDevEnv)
		os.Exit(1)
	}
	if ENV == "development" && Host == "" {
		fmt.Fprintln(os.Stderr, ErrHostEnv)
		os.Exit(1)
	}

	err = Retry(ServiceList, 1000)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Register routes.
	router := mux.NewRouter()
	router.NotFoundHandler = NotFoundHandler
	for _, r := range Routes {
		route := router.NewRoute()
		route.Path(r.Path).Methods(r.Methods...)
		route.HandlerFunc(r.Handler)
	}

	// Start the server.
	server := &http.Server{
		Addr:    ":3001",
		Handler: &SlashHandler{&LogHandler{os.Stdout, router}},
	}

	// Start tcp.
	go StartTCP()

	fmt.Println("Satellite starting!")
	err = server.ListenAndServe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
