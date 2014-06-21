// Copyright 2014 Bowery, Inc.
package main

import (
	"errors"
	"log"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

var (
	sampleService *service       // sample Bowery service
	dockerClient  *docker.Client // client for docker remote agent
)

func init() {
	// Create new Docker client.
	var err error
	dockerClient, err = docker.NewClient("http://localhost:4243")
	if err != nil {
		log.Println("Unable to connect to Docker.")
	}
}

// Attempts to build, inspect, and destroy a docker container.
// In the event any step fails, an error is returned.
func CheckDocker() error {
	// Build container.
	id, err := runBuildContainer()
	if err != nil {
		return err
	}

	// Wait 5 seconds (let it boot up).
	<-time.After(5 * time.Second)

	// Check to make sure it's operating as expected.
	if err := runInspectContainer(id); err != nil {
		return err
	}

	// Remove container.
	if err := runDestroyContainer(id); err != nil {
		return err
	}

	return nil
}

// Build a new container, using a Bowery service
// as an example. Returns the id of the created container.
func runBuildContainer() (string, error) {
	sampleService = &service{
		Type:  "base",
		Ports: []string{"3000"},
	}

	// Start container.
	id, err := sampleService.Run()
	if err != nil {
		return "", err
	}

	return id, nil
}

// Inspect the running container and ensure it
// is currently running. If not, return an error.
func runInspectContainer(id string) error {
	// Inspect container.
	container, err := dockerClient.InspectContainer(id)
	if err != nil {
		return err
	}

	// If it is not running, fail.
	if !container.State.Running {
		return errors.New("Container is not running.")
	}

	return nil
}

// Destroy the running container. If it fails to destroy
// return an error.
func runDestroyContainer(id string) error {
	// Destroy container.
	return dockerClient.RemoveContainer(docker.RemoveContainerOptions{
		ID:            id,
		RemoveVolumes: true,
		Force:         true,
	})
}

// Bowery service.
type service struct {
	DockerID string   // docker id for the container
	Ports    []string //
	Type     string
}

func (s *service) Run() (string, error) {
	options := docker.CreateContainerOptions{
		Config: config(s.Type, s.Ports),
	}

	container, err := dockerClient.CreateContainer(options)
	if err != nil {
		return "", err
	}

	portBindings := map[docker.Port][]docker.PortBinding{
		"22/tcp":   []docker.PortBinding{{HostPort: ""}},
		"80/tcp":   []docker.PortBinding{{HostPort: ""}},
		"3001/tcp": []docker.PortBinding{{HostPort: ""}},
	}

	for i := range s.Ports {
		portBindings[docker.Port(s.Ports[i]+"/tcp")] = []docker.PortBinding{{HostPort: ""}}
	}

	config := &docker.HostConfig{
		Binds:        []string{"/satellite:/satellite"},
		PortBindings: portBindings,
	}

	if err = dockerClient.StartContainer(container.ID, config); err != nil {
		return "", err
	}

	return container.ID, nil
}

func config(typ string, userPorts []string) *docker.Config {
	volumes := make(map[string]struct{})
	volumes["/satellite"] = struct{}{}

	ports := map[docker.Port]struct{}{
		"22/tcp":   {},
		"80/tcp":   {},
		"3001/tcp": {},
	}

	for i := range userPorts {
		port := userPorts[i]
		if port != "22" && port != "80" && port != "3001" {
			ports[docker.Port(port+"/tcp")] = struct{}{}
		}
	}

	if typ == "" {
		typ = "base"
	}

	return &docker.Config{
		Image: "bowery/" + typ,
		Env: []string{
			"ENV=production",
			"HOST=api.bowery.io",
			"APPLICATION=someid",
			"DEVELOPER=somedev",
			"DEBUG=satellite",
		},
		ExposedPorts: ports,
		Cmd:          []string{"/usr/bin/supervisord"},
		Volumes:      volumes,
	}
}
