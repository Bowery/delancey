// Copyright 2014 Bowery, Inc.
package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
)

var (
	pluginManager *PluginManager
	PluginDir     = filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "plugins")
	LogDir        = filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "log")
)

// Create plugin dir.
func init() {
	if PluginDir == "" {
		filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "plugins")
	}
	if err := os.MkdirAll(PluginDir, os.ModePerm|os.ModeDir); err != nil {
		panic(err)
	}
}

// NewPlugin creates a new plugin.
func NewPlugin(name, hooks, requirements string) (*Plugin, error) {
	plugin := &Plugin{Name: name, Hooks: make(map[string]string)}

	if hooks != "" {
		err := json.Unmarshal([]byte(hooks), &plugin.Hooks)
		if err != nil {
			return nil, err
		}
	}

	if requirements != "" {
		err := json.Unmarshal([]byte(requirements), &plugin.Requirements)
		if err != nil {
			return nil, err
		}
	}

	// Verify the os requirements satisfy the current system.
	if plugin.Requirements.OS != nil && len(plugin.Requirements.OS) > 0 {
		found := false

		for _, os := range plugin.Requirements.OS {
			if os == runtime.GOOS || (runtime.GOOS == "darwin" && os == "osx") {
				found = true
			}
		}

		if !found {
			return nil, errors.New("The plugin " + name + " doesn't support the OS " + runtime.GOOS)
		}
	}

	return plugin, nil
}

// NewPluginManager creates a PluginManager.
func NewPluginManager() *PluginManager {
	plugins := make([]*Plugin, 0)

	return &PluginManager{
		Plugins: plugins,
		Event:   make(chan *PluginEvent),
		Error:   make(chan *PluginError),
	}
}

func SetPluginManager() *PluginManager {
	pluginManager = NewPluginManager()
	return pluginManager
}

func AddPlugin(plugin *Plugin) error {
	return pluginManager.AddPlugin(plugin)
}

// AddPlugin adds a new Plugin.
func (pm *PluginManager) AddPlugin(plugin *Plugin) error {
	// makes sure that when dev-mode is turned on the dev plugins overwrite the old ones
	for i, p := range pm.Plugins {
		if p.Name == plugin.Name {
			pm.Plugins[i] = plugin
			return errors.New("plugin exists")
		}
	}

	pm.Plugins = append(pm.Plugins, plugin)
	return nil
}

// RemovePlugin removes a Plugin.
func RemovePlugin(name string) error {
	return pluginManager.RemovePlugin(name)
}

// RemovePlugin removes a Plugin by name.
func (pm *PluginManager) RemovePlugin(name string) error {
	index := -1
	for i, plugin := range pm.Plugins {
		if plugin.Name == name {
			index = i
			break
		}
	}

	if index == -1 {
		return errors.New("invalid plugin name")
	}

	pm.Plugins = append(pm.Plugins[:index], pm.Plugins[index+1:]...)
	return nil
}

// GetPlugins returns a slice of Plugins.
func GetPlugins() []*Plugin {
	return pluginManager.Plugins
}

func GetPlugin(name string) *Plugin {
	return pluginManager.GetPlugin(name)
}

func (pm *PluginManager) GetPlugin(name string) *Plugin {
	for _, plugin := range pm.Plugins {
		if plugin.Name == name {
			return plugin
		}
	}

	return nil
}

// StartPluginListener creates a new plugin manager and
// listens for events.
func StartPluginListener() {
	if pluginManager == nil {
		SetPluginManager()
	}

	// On Event and Error events, execute commands for
	// plugins that have appropriate handlers and are
	// enabled by the specified application.
	for {
		select {
		case ev := <-pluginManager.Event:
			log.Println(fmt.Sprintf("plugin event: %s", ev.Type))
			for _, plugin := range pluginManager.Plugins {
				for _, ep := range ev.EnabledPlugins {
					if ep == plugin.Name {
						background := false
						if ev.Type == schemas.Background {
							background = true
						}

						executeHook(plugin, ev, background)
					}
				}
			}
		}
	}
}

// executeHook runs the specified command and returns the
// resulting output.
func executeHook(plugin *Plugin, ev *PluginEvent, background bool) {
	command := plugin.Hooks[fmt.Sprintf("%s-%s", ev.Type, runtime.GOOS)]
	if command == "" {
		command = plugin.Hooks[ev.Type]
	}
	if command == "" {
		return
	}

	name := plugin.Name
	path := ev.FilePath
	dir := ev.AppDir
	log.Println("plugin execute:", fmt.Sprintf("%s: `%s`", name, command))

	// Set the env for the hook, includes info about the file being modified,
	// and if background the paths for stdio.
	env := map[string]string{
		"APP_DIR":       dir,
		"FILE_AFFECTED": path,
	}
	if background {
		env["STDOUT"] = filepath.Join(LogDir, "stdout.log")
		env["STDERR"] = filepath.Join(LogDir, "stderr.log")
	}

	cmd := sys.NewCommand(command, env)
	cmd.Dir = filepath.Join(PluginDir, name)

	// If it is not a background process, execute immediately
	// and wait for it to complete. If it is a background process
	// pipe the agent's Stdin into the command and run.
	if !background {
		data, err := cmd.CombinedOutput()
		if err != nil {
			handlePluginError(plugin, command, err)
			return
		}

		// debugging
		log.Println(string(data))
	} else {
		// Start the process. If there is an issue starting, alert
		// the client.
		plugin.BackgroundCommand = cmd

		go func() {
			if err := cmd.Start(); err != nil {
				handlePluginError(plugin, command, err)
				return
			}
			if err := cmd.Wait(); err != nil {
				handlePluginError(plugin, command, err)
			}
		}()
	}
}

// handlePluginError emits the values of the plugin error to pluginManager.Error
func handlePluginError(plugin *Plugin, command string, err error) {
	pluginManager.Error <- &PluginError{
		Plugin:  plugin,
		Command: command,
		Error:   err,
	}
}

// EmitPluginEvent creates a new PluginEvent and sends it
// to the pluginManager Event channel.
func EmitPluginEvent(typ, path, dir, id string, enabledPlugins []string) {
	// todo(steve): handle error
	pluginManager.Event <- &PluginEvent{
		Type:           typ,
		FilePath:       path,
		AppDir:         dir,
		Identifier:     id,
		EnabledPlugins: enabledPlugins,
	}
}
