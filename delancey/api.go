// Copyright 2013-2014 Bowery, Inc.
// Contains the paths and url structures for requests to api.
package main

import (
	"os"

	"github.com/Bowery/gopackages/schemas"
)

var (
	ENV           = os.Getenv("ENV")
	Host          = os.Getenv("HOST")
	ApplicationID = os.Getenv("APPLICATION")
	ServiceName   = os.Getenv("NAME")
)

// Base endpoints for api and Redis.
var (
	BasePath  = "http://api.bowery.io"
	RedisPath = "ec2-23-22-237-84.compute-1.amazonaws.com:6379"
)

// Common api endpoints.
var (
	APIApplicationPath = "applications/" + ApplicationID
)

func init() {
	if ENV == "development" {
		if Host == "" {
			Host = "localhost"
		}

		BasePath = "http://" + Host + ":3000"
		RedisPath = Host + ":6379"
	}
}

// API is a single request body or response from api.
type API struct {
	Status      string               `json:"status"`
	Application *schemas.Application `json:"application"`
	Services    []*schemas.Service   `json:"services"`
	Err         string               `json:"error"`
}

// Error returns the error message from the response.
func (api *API) Error() string {
	return api.Err
}
