// Copyright 2014 Bowery, Inc.

package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Bowery/gopackages/schemas"
)

var (
	storedContainerPath = filepath.Join(boweryDir, "agent_container.json")
	containersDir       = filepath.Join(boweryDir, "containers")
	sshDir              = filepath.Join(boweryDir, "ssh")
	currentContainer    *Container
)

// Container wraps a schemas container to provide methods on it.
type Container struct {
	*schemas.Container
}

// NewContainer creates the paths for the given container.
func NewContainer(container *schemas.Container) (*Container, error) {
	root := filepath.Join(containersDir, container.ID)
	if err := os.MkdirAll(root, os.ModePerm|os.ModeDir); err != nil {
		return nil, err
	}

	sshPath := filepath.Join(sshDir, container.ID)
	if err := os.MkdirAll(root, os.ModePerm|os.ModeDir); err != nil {
		return nil, err
	}

	container.RemotePath = root
	container.SSHPath = sshPath
	return &Container{Container: container}, nil
}

// LoadContainer reads the stored container info and creates it in memory.
func LoadContainer() (*Container, error) {
	file, err := os.Open(storedContainerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}
	defer file.Close()

	var containers []*Container
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&containers)
	if err != nil {
		return nil, err
	}

	container := containers[0] // Will always be available.
	if container != nil {
		container, err = NewContainer(container.Container)
	}

	return container, err
}

// Save saves the container info to the FS.
func (container *Container) Save() error {
	// Use slice so loading can capture null, rather than the zero value.
	containers := []*Container{container}
	dat, err := json.MarshalIndent(containers, "", "  ")
	if err != nil {
		return err
	}

	err = os.MkdirAll(boweryDir, os.ModePerm|os.ModeDir)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(storedContainerPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, strings.NewReader(string(dat)))
	return err
}

// DeleteContainer removes the Docker container for the container and it's
// image.
func (container *Container) DeleteDocker() error {
	// Inspect to get the containers image.
	dcontainer, err := DockerClient.Inspect(container.DockerID)
	if err != nil {
		return err
	}

	err = DockerClient.Remove(container.DockerID)
	if err != nil {
		return err
	}

	return DockerClient.RemoveImage(dcontainer.Image)
}

// Delete deletes the containers paths.
func (container *Container) DeletePaths() error {
	err := os.RemoveAll(container.RemotePath)
	if err != nil {
		return err
	}

	return os.RemoveAll(container.SSHPath)
}
