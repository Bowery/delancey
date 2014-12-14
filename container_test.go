// Copyright 2014 Bowery, Inc.
package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Bowery/gopackages/schemas"
)

var Ccontainer = &schemas.Container{
	ID: "some-id",
}

func TestNewContainer(t *testing.T) {
	_, err := NewContainer(Ccontainer)
	if err != nil {
		t.Fatal(err)
	}

	if Ccontainer.RemotePath != filepath.Join(ContainersDir, Ccontainer.ID) {
		t.Error("Container path isn't as expected.")
	}
}

func TestSaveContainerNoContainer(t *testing.T) {
	err := SaveContainer()
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(StoredContainerPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	if err != nil {
		t.Error("StoredContainerPath doesn't exist when it should.")
	}
}

func TestLoadContainerNoContainer(t *testing.T) {
	err := LoadContainer()
	if err != nil {
		t.Fatal(err)
	}

	if CurrentContainer != nil {
		t.Error("CurrentContainer should be nil but isn't.")
	}
}

func TestSaveContainer(t *testing.T) {
	CurrentContainer = Ccontainer
	err := SaveContainer()
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(StoredContainerPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	if err != nil {
		t.Error("StoredContainerPath doesn't exist when it should.")
	}
}

func TestLoadContainer(t *testing.T) {
	err := LoadContainer()
	if err != nil {
		t.Fatal(err)
	}

	if CurrentContainer == nil {
		t.Error("CurrentContainer shouldn't be nil but is.")
	}

	if CurrentContainer.ID != Ccontainer.ID {
		t.Error("CurrentContainers ID doesn't match what was set.")
	}

	CurrentContainer = nil
	os.RemoveAll(StoredContainerPath)
	os.RemoveAll(ContainersDir)
}
