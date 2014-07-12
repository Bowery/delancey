// Copyright 2013-2014 Bowery, Inc.
// Contains the routes for satellite.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// 32 MB, same as http.
const httpMaxMem = 32 << 10

// Directory the service lives in.
var HomeDir = "/root/" // default for ubuntu docker container
var ServiceDir = "/application"
var LastServiceDir = "/application" // so we can cleanup after ourselves

// List of named routes.
var Routes = []*Route{
	&Route{"/", []string{"POST"}, UploadServiceHandler},
	&Route{"/", []string{"PUT"}, UpdateServiceHandler},
	&Route{"/", []string{"GET"}, GetServiceHandler},
	&Route{"/", []string{"DELETE"}, RemoveServiceHandler},
	&Route{"/env", []string{"POST"}, UpdateServicesHandler},
	&Route{"/healthz", []string{"GET"}, HealthzHandler},
}

// Route is a single named route with a http.HandlerFunc.
type Route struct {
	Path    string
	Methods []string
	Handler http.HandlerFunc
}

// POST /, Upload service code running init steps.
func UploadServiceHandler(rw http.ResponseWriter, req *http.Request) {
	res := NewResponder(rw, req)
	attach, _, err := req.FormFile("file")
	if err != nil && err != http.ErrMissingFile {
		res.Body["error"] = err.Error()
		res.Send(http.StatusBadRequest)
		return
	}
	init := req.FormValue("init")
	build := req.FormValue("build")
	test := req.FormValue("test")
	start := req.FormValue("start")
	path := req.FormValue("path")
	pathList := strings.Split(path, ":")

	// If target path is specified and path has changed.
	if len(pathList) == 2 && ServiceDir != pathList[1] {
		root := pathList[1]
		fmt.Println("root", root)
		if string(root[0]) == "~" {
			root = HomeDir + string(root[1:])
		}
		if string(root[0]) != "/" {
			root = HomeDir + root
		}
		ServiceDir = root
		fmt.Println("before/after", ServiceDir, LastServiceDir)
		if err := os.RemoveAll(LastServiceDir); err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
		LastServiceDir = ServiceDir
	}

	if attach != nil {
		defer attach.Close()

		err = Untar(attach, ServiceDir)
		if err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
	}

	err = ServiceList()
	if err != nil {
		res.Body["error"] = err.Error()
		res.Send(http.StatusInternalServerError)
		return
	}

	<-Restart(true, true, init, build, test, start)
	res.Body["status"] = "created"
	res.Send(http.StatusOK)
}

// PUT /, Update service.
func UpdateServiceHandler(rw http.ResponseWriter, req *http.Request) {
	res := NewResponder(rw, req)
	err := req.ParseMultipartForm(httpMaxMem)
	if err != nil {
		res.Body["error"] = err.Error()
		res.Send(http.StatusBadRequest)
		return
	}
	path := req.FormValue("path")
	typ := req.FormValue("type")
	modeStr := req.FormValue("mode")
	init := req.FormValue("init")
	build := req.FormValue("build")
	test := req.FormValue("test")
	start := req.FormValue("start")

	if path == "" || typ == "" {
		res.Body["error"] = ErrMissingFields.Error()
		res.Send(http.StatusBadRequest)
		return
	}
	path = filepath.Join(ServiceDir, filepath.Join(strings.Split(path, "/")...))

	if typ == "delete" {
		// Delete path from the service.
		err = os.RemoveAll(path)
		if err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
	} else {
		// Create/Update path in the service.
		attach, _, err := req.FormFile("file")
		if err != nil {
			if err == http.ErrMissingFile {
				err = ErrMissingFields
			}

			res.Body["error"] = err.Error()
			res.Send(http.StatusBadRequest)
			return
		}
		defer attach.Close()

		// Ensure parents exist.
		err = os.MkdirAll(filepath.Dir(path), os.ModePerm|os.ModeDir)
		if err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}

		dest, err := os.Create(path)
		if err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}
		defer dest.Close()

		// Copy updated contents to destination.
		_, err = io.Copy(dest, attach)
		if err != nil {
			res.Body["error"] = err.Error()
			res.Send(http.StatusInternalServerError)
			return
		}

		// Set the file permissions if given.
		if modeStr != "" {
			mode, err := strconv.ParseUint(modeStr, 10, 32)
			if err != nil {
				res.Body["error"] = err.Error()
				res.Send(http.StatusBadRequest)
				return
			}

			err = dest.Chmod(os.FileMode(mode))
			if err != nil {
				res.Body["error"] = err.Error()
				res.Send(http.StatusInternalServerError)
				return
			}
		}
	}

	<-Restart(false, true, init, build, test, start)
	res.Body["status"] = "updated"
	res.Send(http.StatusOK)
}

// GET /, Retrieve the service and send it in a gzipped tar.
func GetServiceHandler(rw http.ResponseWriter, req *http.Request) {
	contents, err := Tar(ServiceDir)
	if err != nil {
		res := NewResponder(rw, req)
		res.Body["error"] = err.Error()
		res.Send(http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusOK)
	io.Copy(rw, contents)
}

// DEL /, Remove service files.
func RemoveServiceHandler(rw http.ResponseWriter, req *http.Request) {
	res := NewResponder(rw, req)

	err := os.RemoveAll("/application")
	if err != nil {
		res.Body["error"] = err.Error()
		res.Send(http.StatusInternalServerError)
		return
	}

	res.Body["status"] = "success"
	res.Send(http.StatusOK)
}

// POST /env, Update services list and restart service.
func UpdateServicesHandler(rw http.ResponseWriter, req *http.Request) {
	res := NewResponder(rw, req)

	body := new(API)
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(body)
	if err != nil {
		if err == io.EOF {
			err = ErrMissingFields
		}

		res.Body["error"] = err.Error()
		res.Send(http.StatusInternalServerError)
		return
	}

	err = SetEnv(body.Services)
	if err != nil {
		res.Body["error"] = err.Error()
		res.Send(http.StatusInternalServerError)
		return
	}

	<-Restart(true, false, "", "", "", "")
	res.Body["status"] = "updated"
	res.Send(http.StatusOK)
}

// GET /healthz, Return the status of a container
func HealthzHandler(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "ok")
}
