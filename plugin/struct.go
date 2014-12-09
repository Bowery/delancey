// Copyright 2014 Bowery, Inc.
package plugin

import (
	"os/exec"

	"github.com/Bowery/gopackages/schemas"
)

// Plugin defines the properties and event handlers
// of a plugin.
type Plugin struct {
	// Name of plugin.
	Name string

	// Author of plugin.
	Author schemas.Author

	// Hooks and associated handlers.
	Hooks map[string]string

	// Required software and dependencies.
	Requirements schemas.Requirements

	// BackgroundProcess is the long standing background
	// process for a given plugin. The application's
	// Stdout and Stderr are piped to this process.
	BackgroundCommand *exec.Cmd
}

// PluginManager manages all of the plugins as well as
// channels for events and errors.
type PluginManager struct {
	// Array of active plugins.
	Plugins []*Plugin

	// PluginEvent channel.
	Event chan *PluginEvent

	// PluginError channel.
	Error chan *PluginError
}

// Event describes a plugin event along with
// associated data.
type PluginEvent struct {
	// The type of event (e.g. after-restart, before-update)
	Type string

	// The path of the file that has been changed.
	FilePath string

	// The directory of the application code.
	AppDir string

	// A unique identifier (typically the application id).
	Identifier string

	// The set of plugins applicable to this event.
	EnabledPlugins []string
}

// Error describes a plugin error along with
// associated data.
type PluginError struct {
	// The plugin the error came from.
	Plugin *Plugin

	Command string

	// The error that occured.
	Error error
}
