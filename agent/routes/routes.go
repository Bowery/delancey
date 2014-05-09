// Copyright 2014 Bowery, Inc.
package routes

import (
	"Bowery/crosswalk/agent/opts"
	"Bowery/crosswalk/agent/proc"
	"Bowery/crosswalk/agent/tar"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/martini-contrib/render"
)

const httpMaxMem = 32 << 10

func HandleNewService(r render.Render, res http.ResponseWriter, req *http.Request) {
	attach, _, err := req.FormFile("file")
	if err != nil {
		if err == http.ErrMissingFile {
			err = errors.New("Missing form fields.")
		}
		r.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}
	defer attach.Close()

	if err = os.MkdirAll(*opts.TargetDir, 0755); err != nil {
		r.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
	}

	if err = tar.Untar(attach, *opts.TargetDir); err != nil {
		r.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
		return
	}

	build := req.FormValue("build")
	test := req.FormValue("test")
	start := req.FormValue("start")
	<-proc.Restart(build, test, start)

	r.JSON(http.StatusOK, map[string]interface{}{"status": "created"})
}

func HandleUpdateService(r render.Render, res http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(httpMaxMem); err != nil {
		r.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
		return
	}

	path := req.FormValue("path")
	typ := req.FormValue("type")

	if path == "" || typ == "" {
		r.JSON(http.StatusBadRequest, map[string]interface{}{"error": "Missing form fields."})
		return
	}

	path = filepath.Join(*opts.TargetDir, path)

	if typ == "delete" {
		err := os.RemoveAll(path)
		if err != nil {
			r.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			return
		}
	} else {
		attach, _, err := req.FormFile("file")
		if err != nil {
			if err == http.ErrMissingFile {
				err = errors.New("Missing form fields.")
			}
			r.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
			return
		}
		defer attach.Close()

		if err = os.MkdirAll(filepath.Dir(path), os.ModePerm|os.ModeDir); err != nil {
			r.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			return
		}

		dest, err := os.Create(path)
		if err != nil {
			r.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			return
		}
		defer dest.Close()

		_, err = io.Copy(dest, attach)
		if err != nil {
			r.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
			return
		}

		build := req.FormValue("build")
		test := req.FormValue("test")
		start := req.FormValue("start")
		<-proc.Restart(build, test, start)

		r.JSON(http.StatusOK, map[string]interface{}{"status": "created"})
	}
}

func HandleGetService() string {
	return "Crosswalk Agent"
}

func HandlePingService() string {
	return "ok"
}
