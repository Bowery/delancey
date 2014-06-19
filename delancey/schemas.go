// Copyright 2013-2014 Bowery, Inc.
// Contains a slim version of the api schemas used for marshalling.
package main

// Service is a single service with a public address.
type Service struct {
	Name        string            `json:"name"`
	PublicAddr  string            `json:"publicAddr"`
	CustomPorts map[string]string `json:"customPorts,omitempty"`
	Start       string            `json:"start,omitempty"`
	Build       string            `json:"build,omitempty"`
	Test        string            `json:"test,omitempty"`
	Init        string            `json:"init,omitempty"`
}

// Application is a single application from api.
type Application struct {
	ID       string     `json:"_id"`
	Services []*Service `json:"services"`
}
