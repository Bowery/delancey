// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Bowery/delancey/plugin"
	"github.com/Bowery/gopackages/requests"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/sys"
	"github.com/Bowery/gopackages/tar"
	"github.com/Bowery/gopackages/web"
	"github.com/unrolled/render"
)

// 32 MB, same as http.
const httpMaxMem = 32 << 10

var (
	HomeDir   = os.Getenv(sys.HomeVar)
	BoweryDir = filepath.Join(HomeDir, ".bowery")
)

var renderer = render.New(render.Options{
	IndentJSON:    true,
	IsDevelopment: true,
})

// List of named routes.
var Routes = []web.Route{
	{"GET", "/", IndexHandler, false},
	{"POST", "/", UploadServiceHandler, false},
	{"PUT", "/", UpdateServiceHandler, false},
	{"DELETE", "/", RemoveServiceHandler, false},
	{"POST", "/command", RunCommandHandler, false},
	{"POST", "/commands", RunCommandsHandler, false},
	{"POST", "/plugins", UploadPluginHandler, false},
	{"PUT", "/plugins", UpdatePluginHandler, false},
	{"DELETE", "/plugins", RemovePluginHandler, false},
	{"GET", "/network", NetworkHandler, false},
	{"GET", "/healthz", HealthzHandler, false},
	{"POST", "/password", PasswordHandler, false},
	{"GET", "/_/state/apps", AppStateHandler, false},
	{"GET", "/_/state/plugins", PluginStateHandler, false},
}

// runCmdsReq is the request body to execute a command.
type runCmdReq struct {
	AppID string `json:"appID"`
	Cmd   string `json:"cmd"`
}

// runCmdsReq is the request body to execute a number of commands.
type runCmdsReq struct {
	AppID string   `json:"appID"`
	Cmds  []string `json:"cmds"`
}

// GET /, Home page.
func IndexHandler(rw http.ResponseWriter, req *http.Request) {
	id := req.FormValue("id")
	if id == "" {
		fmt.Fprintf(rw, "Bowery Agent v"+VERSION)
		return
	}

	app := Applications[id]
	if app == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "invalid app id",
		})
		return
	}

	contents, err := tar.Tar(app.Path, []string{})
	if err != nil && !os.IsNotExist(err) {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	// If the path didn't exist, just provide an empty targz stream.
	if err != nil {
		empty, gzipWriter, tarWriter := tar.NewTarGZ()
		tarWriter.Close()
		gzipWriter.Close()
		contents = empty
	}

	rw.WriteHeader(http.StatusOK)
	io.Copy(rw, contents)
}

// POST /, Upload service code running init steps.
func UploadServiceHandler(rw http.ResponseWriter, req *http.Request) {
	attach, _, err := req.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}
	id := req.FormValue("id")
	init := req.FormValue("init")
	build := req.FormValue("build")
	test := req.FormValue("test")
	start := req.FormValue("start")
	path := req.FormValue("path")

	go logClient.Info("creating application", map[string]interface{}{
		"appID": id,
		"ip":    AgentHost,
	})

	// Create new application.
	app, err := NewApplication(id, init, build, test, start, path)
	if err != nil {
		go logClient.Error(err.Error(), map[string]interface{}{
			"app": app,
			"ip":  AgentHost,
		})
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	// Set new application, killing any existing cmds created from an app with the same id.
	if oldApp, ok := Applications[id]; ok {
		Kill(oldApp, true)
	}
	Applications[id] = app
	SaveApps()

	plugin.EmitPluginEvent(schemas.BeforeFullUpload, "", app.Path, app.ID, app.EnabledPlugins)

	if attach != nil {
		defer attach.Close()

		err = tar.Untar(attach, app.Path)
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}
	}

	plugin.EmitPluginEvent(schemas.AfterFullUpload, "", app.Path, app.ID, app.EnabledPlugins)
	<-Restart(app, true, true)
	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusCreated,
	})
}

// PUT /, Update service.
func UpdateServiceHandler(rw http.ResponseWriter, req *http.Request) {
	err := req.ParseMultipartForm(httpMaxMem)
	if err != nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}
	id := req.FormValue("id")
	pathType := req.FormValue("pathtype")
	path := req.FormValue("path")
	typ := req.FormValue("type")
	modeStr := req.FormValue("mode")
	init := req.FormValue("init")
	build := req.FormValue("build")
	test := req.FormValue("test")
	start := req.FormValue("start")

	go logClient.Info("updating application", map[string]interface{}{
		"appID": id,
		"ip":    AgentHost,
	})

	app := Applications[id]
	if app == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "invalid app id",
		})
		return
	}

	// Update application.
	app.Init = init
	app.Build = build
	app.Test = test
	app.Start = start
	SaveApps()

	if path == "" || typ == "" {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "Missing form fields.",
		})
		return
	}
	switch typ {
	case "delete":
		plugin.EmitPluginEvent(schemas.BeforeFileDelete, path, app.Path, app.ID, app.EnabledPlugins)
	case "update":
		plugin.EmitPluginEvent(schemas.BeforeFileUpdate, path, app.Path, app.ID, app.EnabledPlugins)
	case "create":
		plugin.EmitPluginEvent(schemas.BeforeFileCreate, path, app.Path, app.ID, app.EnabledPlugins)
	}
	path = filepath.Join(app.Path, filepath.Join(strings.Split(path, "/")...))

	if typ == "delete" {
		// Delete path from the service.
		err = os.RemoveAll(path)
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}
	} else {
		// Create/Update path in the service.
		var dest *os.File

		if pathType == "dir" {
			err = os.MkdirAll(path, os.ModePerm|os.ModeDir)
			if err != nil {
				renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}
		} else {
			attach, _, err := req.FormFile("file")
			if err != nil {
				if err == http.ErrMissingFile {
					err = errors.New("Missing form fields.")
				}

				renderer.JSON(rw, http.StatusBadRequest, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}
			defer attach.Close()

			// Ensure parents exist.
			err = os.MkdirAll(filepath.Dir(path), os.ModePerm|os.ModeDir)
			if err != nil {
				renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}

			dest, err = os.Create(path)
			if err != nil {
				renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}
			defer dest.Close()

			// Copy updated contents to destination.
			_, err = io.Copy(dest, attach)
			if err != nil {
				renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}
		}

		// Set the file permissions if given.
		if modeStr != "" {
			mode, err := strconv.ParseUint(modeStr, 10, 32)
			if err != nil {
				renderer.JSON(rw, http.StatusBadRequest, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}

			err = os.Chmod(path, os.FileMode(mode))
			if err != nil {
				renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}
		}
	}

	switch typ {
	case "delete":
		plugin.EmitPluginEvent(schemas.AfterFileDelete, path, app.Path, app.ID, app.EnabledPlugins)
	case "update":
		plugin.EmitPluginEvent(schemas.AfterFileUpdate, path, app.Path, app.ID, app.EnabledPlugins)
	case "create":
		plugin.EmitPluginEvent(schemas.AfterFileCreate, path, app.Path, app.ID, app.EnabledPlugins)
	}

	<-Restart(app, false, true)
	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusUpdated,
	})
}

// DELETE /, Remove service.
func RemoveServiceHandler(rw http.ResponseWriter, req *http.Request) {
	id := req.FormValue("id")
	app := Applications[id]
	if app != nil {
		plugin.EmitPluginEvent(schemas.BeforeAppDelete, "", app.Path, app.ID, app.EnabledPlugins)
		Kill(app, true)
		delete(Applications, id)
		plugin.EmitPluginEvent(schemas.AfterAppDelete, "", app.Path, app.ID, app.EnabledPlugins)
	}

	go logClient.Info("removing application", map[string]interface{}{
		"appID": id,
		"ip":    AgentHost,
	})

	SaveApps()
	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusRemoved,
	})
}

// POST /command, Run a command.
func RunCommandHandler(rw http.ResponseWriter, req *http.Request) {
	body := new(runCmdReq)
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(body)
	if err != nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	// Validate body.
	if body.Cmd == "" {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "cmd field is required.",
		})
		return
	}

	go logClient.Info("running command", map[string]interface{}{
		"command": body.Cmd,
		"ip":      AgentHost,
	})

	// Get the data from the optional application.
	path := HomeDir
	var stdout *OutputWriter
	var stderr *OutputWriter
	app := Applications[body.AppID]
	if app != nil {
		path = app.Path
		stdout = app.StdoutWriter
		stderr = app.StderrWriter
	}

	cmd := parseCmd(body.Cmd, path, stdout, stderr)
	go func() {
		err := cmd.Run()
		if err != nil {
			if stderr != nil {
				stderr.Write([]byte(err.Error()))
			}

			log.Println(err)
		}
	}()

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusSuccess,
	})
}

// POST /commands, Run multiple commands. Do not respond successfully
// until all commands have finished running.
func RunCommandsHandler(rw http.ResponseWriter, req *http.Request) {
	body := new(runCmdsReq)
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(body)
	if err != nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	if len(body.Cmds) <= 0 {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "cmds field is required.",
		})
		return
	}

	go logClient.Info("running commands", map[string]interface{}{
		"commands": body.Cmds,
		"ip":       AgentHost,
	})

	// Get the data from the optional application.
	path := HomeDir
	var stdout *OutputWriter
	var stderr *OutputWriter
	app := Applications[body.AppID]
	if app != nil {
		path = app.Path
		stdout = app.StdoutWriter
		stderr = app.StderrWriter
	}

	for _, c := range body.Cmds {
		cmd := parseCmd(c, path, stdout, stderr)
		err := cmd.Run()
		if err != nil {
			if stderr != nil {
				stderr.Write([]byte(err.Error()))
			}

			log.Println(err)
		}
	}

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusSuccess,
	})
}

// POST /plugins, Upload a plugin
func UploadPluginHandler(rw http.ResponseWriter, req *http.Request) {
	attach, _, err := req.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	appID := req.FormValue("appID")
	if appID == "" {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "appID required",
		})
		return
	}

	app := Applications[appID]
	if app == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  fmt.Sprintf("no app exists with id %s", appID),
		})
		return
	}

	name := req.FormValue("name")
	if name == "" {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "plugin name required",
		})
		return
	}

	// Create a new plugin.
	hooks := req.FormValue("hooks")
	requirements := req.FormValue("requirements")
	p, err := plugin.NewPlugin(name, hooks, requirements)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	// Untar the plugin upload.
	pluginPath := filepath.Join(plugin.PluginDir, name)
	if attach != nil {
		defer attach.Close()
		if err = tar.Untar(attach, pluginPath); err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}
	}

	// Add it to the plugin manager.
	if err := plugin.AddPlugin(p); err == nil {
		app.EnabledPlugins = append(app.EnabledPlugins, name)
	}

	// Fire off init and background plugin events.
	go plugin.EmitPluginEvent(schemas.OnPluginInit, "", "", app.ID, app.EnabledPlugins)
	go plugin.EmitPluginEvent(schemas.Background, "", "", app.ID, app.EnabledPlugins)

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusSuccess,
	})
}

// PUT /plugins, Updates a plugin
func UpdatePluginHandler(rw http.ResponseWriter, req *http.Request) {
	// TODO (sjkaliski or rm): edit hooks
	appID := req.FormValue("appID")
	name := req.FormValue("name")
	isEnabledStr := req.FormValue("isEnabled")
	if appID == "" || name == "" || isEnabledStr == "" {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "missing fields",
		})
		return
	}

	app := Applications[appID]
	if app == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  fmt.Sprintf("no app exists with id %s", appID),
		})
		return
	}

	isEnabled, err := strconv.ParseBool(isEnabledStr)
	if err != nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	// Verify the plugin exists.
	p := plugin.GetPlugin(name)
	if p == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "invalid plugin name",
		})
		return
	}

	// Add/remove from enabled plugins.
	if isEnabled {
		app.EnabledPlugins = append(app.EnabledPlugins, p.Name)

		// Fire off init and background events.
		go plugin.EmitPluginEvent(schemas.OnPluginInit, "", "", app.ID, app.EnabledPlugins)
		go plugin.EmitPluginEvent(schemas.Background, "", "", app.ID, app.EnabledPlugins)
	} else {
		for i, ep := range app.EnabledPlugins {
			if ep == p.Name {
				j := i + 1
				app.EnabledPlugins = append(app.EnabledPlugins[:i], app.EnabledPlugins[j:]...)
				break
			}
		}
	}

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusSuccess,
	})
}

// DELETE /plugins?name=PLUGIN_NAME, Removes a plugin
func RemovePluginHandler(rw http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()

	if len(query["name"]) < 1 {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "valid plugin name required",
		})
		return
	}

	pluginName := query["name"][0]

	if err := plugin.RemovePlugin(pluginName); err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  "unable to remove plugin",
		})
		return
	}

	if err := os.RemoveAll(filepath.Join(plugin.PluginDir, pluginName)); err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  "unable to remove plugin code",
		})
		return
	}

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusSuccess,
	})
}

// GET /network, returns network information for an app.
func NetworkHandler(rw http.ResponseWriter, req *http.Request) {
	id := req.FormValue("id")

	app := Applications[id]
	if app == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "invalid app id",
		})
		return
	}

	appNetwork, generic, err := GetNetwork(app)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	renderer.JSON(rw, http.StatusOK, map[string]interface{}{
		"status":  requests.StatusSuccess,
		"app":     appNetwork,
		"generic": generic,
	})
}

// GET /state, Return the current application data.
func AppStateHandler(rw http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(Applications)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Write(data)
}

func PluginStateHandler(rw http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(plugin.GetPlugins())
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.Write(data)
}

// POST /password, Sets the password for a user and sets up ssh for password.
func PasswordHandler(rw http.ResponseWriter, req *http.Request) {
	sshPath := "/etc/ssh/sshd_config"
	user := req.FormValue("user")
	pass := req.FormValue("password")
	if user == "" || pass == "" {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "user and password is required",
		})
		return
	}
	buf := bytes.NewBufferString(user + ":" + pass)
	var buferr bytes.Buffer

	// Set the users password.
	cmd := sys.NewCommand("chpasswd", nil)
	cmd.Stdin = buf
	cmd.Stderr = &buferr
	err := cmd.Run()
	if err != nil {
		if cmd.ProcessState != nil && !cmd.ProcessState.Success() {
			err = errors.New("ProcessError: " + strings.TrimSpace(buferr.String()))
		}

		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	// Open the sshd config to edit it.
	source, err := os.Open(sshPath)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}
	defer source.Close()

	// Create tmp file to store changes.
	tmp := filepath.Join(os.TempDir(), "bowery_ssh_"+strconv.FormatInt(time.Now().Unix(), 10))
	dest, err := os.Create(tmp)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}
	defer dest.Close()

	// Replace the PasswordAuthentication to yes.
	scanner := bufio.NewScanner(source)
	for scanner.Scan() {
		text := scanner.Text()
		isComment := len(text) > 0 && text[0] == '#'
		text = strings.TrimLeft(text, "# ")
		passStr := "PasswordAuthentication"

		// If we've found the password auth line, reset it to yes.
		if len(text) >= len(passStr) && text[:len(passStr)] == passStr &&
			len(strings.Fields(text)) == 2 {
			text = "PasswordAuthentication yes"
		}
		if isComment {
			text = "# " + text
		}

		_, err := dest.Write([]byte(text + "\n"))
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}
	}

	// Move the tmp file back to the ssh path.
	err = scanner.Err()
	if err == nil {
		err = os.Rename(tmp, sshPath)
	}
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	// Restart the sshd daemon.
	cmd = sys.NewCommand("service ssh restart", nil)
	cmd.Run()
	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusSuccess,
	})
}

// GET /healthz, Return the status of a container
func HealthzHandler(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "ok")
}
