// Copyright 2014 Bowery, Inc.
// Tests for the Plugin API. Includes loading, adding, updating, and
// removing plugins.
//
// Note(steve): Not completed yet.
package plugin

import (
	"testing"

	"github.com/Bowery/gopackages/schemas"
)

var (
	testPlugin = &Plugin{
		Name: "test-plugin",
		Hooks: map[string]string{
			"AFTER_APP_RESTART": "echo Restart",
		},
		Author: schemas.Author{
			Name:    "Steve Kaliski",
			Email:   "steve@bowery.io",
			Twitter: "@stevekaliski",
			GitHub:  "github.com/sjkaliski",
		},
	}
	testPluginManager *PluginManager
)

func init() {
	PluginDir = "plugins"
}

func TestNewPluginManager(t *testing.T) {
	testPluginManager = NewPluginManager()

	if len(testPluginManager.Plugins) > 0 {
		t.Fatal("NewPluginManager created with non-zero quantity of plugins.")
	}
}

func TestAddPlugin(t *testing.T) {
	testPluginManager.AddPlugin(testPlugin)

	for _, plugin := range testPluginManager.Plugins {
		if plugin.Name == "test-plugin" {
			return
		}
	}

	t.Error("Failed to add plugin.")
}

func TestRemovePlugin(t *testing.T) {
	err := testPluginManager.RemovePlugin("test-plugin")
	if err != nil {
		t.Fatal(err)
	}

	for _, plugin := range testPluginManager.Plugins {
		if plugin.Name == "test-plugin" {
			t.Fatal("Failed to remove the plugin")
		}
	}
}
