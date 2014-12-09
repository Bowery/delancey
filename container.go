// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/Bowery/gopackages/schemas"
)

var (
	StoredContainerPath = filepath.Join(BoweryDir, "agent_container.json")
	ContainersDir       = filepath.Join(BoweryDir, "containers")
)

// NewContainer creates the remote path for the given container.
func NewContainer(container *schemas.Container) (*schemas.Container, error) {
	root := filepath.Join(ContainersDir, container.ID)
	if err := os.MkdirAll(root, os.ModePerm|os.ModeDir); err != nil {
		return nil, err
	}

	container.RemotePath = root
	return container, nil
}

// LoadContainer reads the stored container info and creates it in memory.
func LoadContainer() error {
	file, err := os.Open(StoredContainerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}
	defer file.Close()

	containers := make([]*schemas.Container, 0)
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&containers)
	if err != nil {
		return err
	}
	container := containers[0] // Will always be available.

	if container != nil {
		_, err = NewContainer(container)
		if err != nil {
			return err
		}
	}

	CurrentContainer = container
	return nil
}

// SaveContainer saves the current container.
func SaveContainer() error {
	// Use slice so loading can capture null, rather than the zero value.
	container := []*schemas.Container{CurrentContainer}
	dat, err := json.MarshalIndent(container, "", "  ")
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(dat)

	err = os.MkdirAll(BoweryDir, os.ModePerm|os.ModeDir)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(StoredContainerPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, buf)
	return err
}
