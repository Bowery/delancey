// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Bowery/gopackages/sys"
)

var AppsPath = filepath.Join(BoweryDir, "agent_apps.json")

// storedApp defines the internal structure when saving/loading apps.
type storedApp struct {
	ID    string `json:"id"`
	Init  string `json:"init,omitempty"`
	Build string `json:"build,omitempty"`
	Test  string `json:"test,omitempty"`
	Start string `json:"start,omitempty"`
	Path  string `json:"path,omitempty"`
}

// Application defines an application.
type Application struct {
	// Unique identifier.
	ID string `json:"id"`

	// Init command. Ran on start and in background.
	Init string `json:"init,omitempty"`

	// Existing init command.
	InitCmd *exec.Cmd `json:"initCmd,omitempty"`

	// Build command. Ran prior to test.
	Build string `json:"build,omitempty"`

	// Existing build command.
	BuildCmd *exec.Cmd `json:"buildCmd,omitempty"`

	// Test command. Ran prior to start.
	Test string `json:"test,omitempty"`

	// Existing test command.
	TestCmd *exec.Cmd `json:"testCmd,omitempty"`

	// Start command. Ran in the background.
	Start string `json:"start,omitempty"`

	// Existing start command.
	StartCmd *exec.Cmd `json:"startCmd,omitempty"`

	// The location of the application's code.
	Path string `json:"path,omitempty"`

	// Commands.
	CmdStrs [4]string `json:"cmdStrs,omitempty"`

	// Enabled plugins: name@version.
	EnabledPlugins []string `json:"enabledPlugins,omitempty"`

	// Plugin processes. Maps a plugin to background and init
	// process pids.
	PluginProcesses map[string]map[string]int `json:"pluginProcesses,omitempty"`

	// State of process (e.g. building, testing, running, etc.)
	State string `json:"processState,omitempty"`

	// OutputWriter for stdout.
	StdoutWriter *OutputWriter `json:"stdoutWriter,-"`

	// OutputWriter for stderr.
	StderrWriter *OutputWriter `json:"stderrWriter,-"`
}

// NewApplication creates a new Application. Validates contents
// and determines target path. Returns a pointer to an Application.
func NewApplication(id, init, build, test, start, path string) (*Application, error) {
	root := ""
	pathList := strings.Split(path, "::")
	if len(pathList) == 2 {
		root = pathList[1]
		if len(root) > 0 && root[0] == '~' {
			root = filepath.Join(os.Getenv(sys.HomeVar), string(root[1:]))
		}
		if (len(root) > 0 && filepath.Separator == '/' && root[0] != '/') ||
			(filepath.Separator != '/' && filepath.VolumeName(root) == "") {
			root = filepath.Join(HomeDir, root)
		}
	} else {
		root = pathList[0]
	}
	if err := os.MkdirAll(root, os.ModePerm|os.ModeDir); err != nil {
		return nil, err
	}

	// Create stdout and stderr writers
	outputPath := filepath.Join(os.Getenv(sys.HomeVar), ".bowery", "log")
	stdoutWriter, err := NewOutputWriter(filepath.Join(outputPath, fmt.Sprintf("%s-stdout.log", id)))
	if err != nil {
		return nil, err
	}
	stderrWriter, err := NewOutputWriter(filepath.Join(outputPath, fmt.Sprintf("%s-stderr.log", id)))
	if err != nil {
		return nil, err
	}

	app := &Application{
		ID:           id,
		Init:         init,
		Build:        build,
		Test:         test,
		Start:        start,
		CmdStrs:      [4]string{},
		Path:         root,
		StdoutWriter: stdoutWriter,
		StderrWriter: stderrWriter,
	}

	return app, nil
}

// LoadApps reads the stored app info and creates them in memory.
func LoadApps() error {
	file, err := os.Open(AppsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}
	defer file.Close()

	apps := make(map[string]*storedApp)
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&apps)
	if err != nil {
		return err
	}

	// Create the full applications and add to the map.
	for _, appInfo := range apps {
		app, err := NewApplication(appInfo.ID, appInfo.Init, appInfo.Build, appInfo.Test, appInfo.Start, appInfo.Path)
		if err != nil {
			return err
		}

		Applications[app.ID] = app
	}

	return nil
}

func SaveApps() error {
	// Create the simple apps data.
	apps := make(map[string]*storedApp)
	if Applications != nil {
		for id, app := range Applications {
			apps[id] = &storedApp{
				ID:    app.ID,
				Init:  app.Init,
				Build: app.Build,
				Test:  app.Test,
				Start: app.Start,
				Path:  app.Path,
			}
		}
	}

	dat, err := json.MarshalIndent(apps, "", "  ")
	if err != nil {
		return err
	}
	buf := bytes.NewBuffer(dat)

	err = os.MkdirAll(BoweryDir, os.ModePerm|os.ModeDir)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(AppsPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, buf)
	return err
}
