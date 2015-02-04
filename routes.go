// Copyright 2013-2014 Bowery, Inc.

package main

import (
	stdtar "archive/tar"
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

	"code.google.com/p/go-uuid/uuid"
	"github.com/Bowery/delancey/delancey"
	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/docker"
	"github.com/Bowery/gopackages/docker/quay"
	"github.com/Bowery/gopackages/path"
	"github.com/Bowery/gopackages/requests"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/tar"
	"github.com/Bowery/gopackages/web"
	"github.com/Bowery/kenmare/kenmare"
	"github.com/unrolled/render"
)

const (
	// 32 MB, same as http.
	httpMaxMem = 32 << 10
	// Dockerfile contents to use when creating an image.
	passwordDockerfile = "FROM {{baseimage}}\nRUN echo '{{user}}:{{password}}' | chpasswd"
)

var (
	homeDir          = "/home/ubuntu"
	boweryDir        = filepath.Join(homeDir, ".bowery")
	currentContainer *schemas.Container
)

var renderer = render.New(render.Options{
	IndentJSON:    true,
	IsDevelopment: true,
})

// List of named routes.
var Routes = []web.Route{
	{"GET", "/", indexHandler, false},
	{"POST", "/", createContainerHandler, false},
	{"PUT", "/", uploadContainerHandler, false},
	{"PATCH", "/", updateContainerHandler, false},
	{"PATCH", "/batch", batchUpdateContainerHandler, false},
	{"PUT", "/containers", saveContainerHandler, false},
	{"DELETE", "/", removeContainerHandler, false},
	{"PUT", "/ssh", uploadSSHHandler, false},
	{"GET", "/healthz", healthzHandler, false},
	{"GET", "/_/state/container", containerStateHandler, false},
	{"POST", "/_/pull", pullImageHandler, false},
}

// GET /, Retrieve the containers code.
func indexHandler(rw http.ResponseWriter, req *http.Request) {
	var (
		contents io.Reader
		err      error
	)
	empty, gzipWriter, tarWriter := tar.NewTarGZ()
	tarWriter.Close()
	gzipWriter.Close()

	// Require a container to exist.
	if currentContainer == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  delancey.ErrNotInUse.Error(),
		})
		return
	}

	// Tar the contents of the container.
	contents, err = tar.Tar(currentContainer.RemotePath, []string{})
	if err != nil && !os.IsNotExist(err) {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	// If the path didn't exist, just provide an empty targz stream.
	if err != nil {
		contents = empty
	}

	rw.WriteHeader(http.StatusOK)
	io.Copy(rw, contents)
}

// POST /, Create container.
func createContainerHandler(rw http.ResponseWriter, req *http.Request) {
	// Only allow one container at a time.
	if currentContainer != nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  delancey.ErrInUse.Error(),
		})
		return
	}

	// Get container from body.
	container := new(schemas.Container)
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(container)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	go logClient.Info("creating container", map[string]interface{}{
		"container": container,
		"ip":        agentHost,
	})

	// Create new Container.
	_, err = NewContainer(container)
	if err != nil {
		go logClient.Error(err.Error(), map[string]interface{}{
			"container": container,
			"ip":        agentHost,
		})
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}
	image := config.DockerBaseImage + ":" + container.ImageID

	if Env != "testing" {
		// Pull the image down to check if it exists.
		log.Println("Pulling down image", container.ImageID)
		progChan := make(chan float64)

		go func() {
			for prog := range progChan {
				val := "environment:" + strconv.FormatFloat(prog, 'e', -1, 64)
				pusherC.Publish(val, "progress", fmt.Sprintf("container-%s", container.ID))
			}
		}()

		err = quay.PullImage(DockerClient, image, progChan)
		if err != nil && !quay.IsNotFound(err) {
			go logClient.Error(err.Error(), map[string]interface{}{
				"container": container,
				"ip":        agentHost,
			})
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}

		// If the tag doesn't exist yet, create it from the base.
		if err != nil {
			// Create a container using the base.
			log.Println("Image doesn't exist", container.ImageID)
			log.Println("Creating build container using base image for", container.ImageID)
			id, err := DockerClient.Create(new(docker.Config), config.DockerBaseImage, nil)
			if err != nil {
				go logClient.Error(err.Error(), map[string]interface{}{
					"container": container,
					"ip":        agentHost,
				})
				renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}

			// Commit the empty container to the image name.
			log.Println("Commit build container to image", container.ImageID)
			err = DockerClient.CommitImage(id, image)
			if err != nil {
				go logClient.Error(err.Error(), map[string]interface{}{
					"container": container,
					"ip":        agentHost,
				})
				renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}

			// Clean up the container.
			log.Println("Removing build container", container.ImageID)
			go DockerClient.Remove(id)
		}

		// Build the image to use for the container, which sets the password.
		log.Println("Creating Dockerfile input", container.ImageID)
		user := "root"
		password := uuid.New()
		input, err := createImageInput(passwordDockerfile, map[string]string{
			"baseimage": image,
			"user":      user,
			"password":  password,
		})
		if err != nil {
			go logClient.Error(err.Error(), map[string]interface{}{
				"container": container,
				"ip":        agentHost,
			})
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}

		log.Println("Creating runner image for container", container.ImageID)
		image, err = DockerClient.BuildImage(input, "", config.DockerBaseImage)
		if err != nil {
			go logClient.Error(err.Error(), map[string]interface{}{
				"container": container,
				"ip":        agentHost,
			})
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}

		container.ContainerPath = "/root/" + filepath.Base(path.RelSystem(container.LocalPath))
		config := &docker.Config{
			Volumes: map[string]string{
				container.RemotePath: container.ContainerPath,
				container.SSHPath:    "/root/.ssh",
			},
			NetworkMode: "host",
		}

		log.Println("Creating container", container.ImageID)
		id, err := DockerClient.Create(config, image, []string{"/usr/sbin/sshd", "-D"})
		if err != nil {
			go logClient.Error(err.Error(), map[string]interface{}{
				"container": container,
				"ip":        agentHost,
			})
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}

		log.Println("Starting container", container.ImageID)
		err = DockerClient.Start(config, id)
		if err != nil {
			go logClient.Error(err.Error(), map[string]interface{}{
				"container": container,
				"ip":        agentHost,
			})
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}
		log.Println("Container started", id, container.ImageID)

		container.User = user
		container.Password = password
		container.DockerID = id
	}

	currentContainer = container
	SaveContainer()
	renderer.JSON(rw, http.StatusOK, map[string]interface{}{
		"status":    requests.StatusCreated,
		"container": container,
	})
}

// PUT /, Upload code for container.
func uploadContainerHandler(rw http.ResponseWriter, req *http.Request) {
	// Require a container to exist.
	if currentContainer == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  delancey.ErrNotInUse.Error(),
		})
		return
	}

	// Untar the tar contents from the body to the containers path.
	err := tar.Untar(req.Body, currentContainer.RemotePath)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusSuccess,
	})
}

// PATCH /, Update the FS with a file change.
func updateContainerHandler(rw http.ResponseWriter, req *http.Request) {
	// Get the fields required to do the path update.
	err := req.ParseMultipartForm(httpMaxMem)
	if err != nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}
	pathType := req.FormValue("pathtype")
	relPath := req.FormValue("path")
	typ := req.FormValue("type")
	modeStr := req.FormValue("mode")
	if relPath == "" || typ == "" {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "Missing form fields.",
		})
		return
	}
	fullPath := filepath.Join(currentContainer.RemotePath, path.RelSystem(relPath))

	// Container needs to exist.
	if currentContainer == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  delancey.ErrNotInUse.Error(),
		})
		return
	}

	go logClient.Info("updating container", map[string]interface{}{
		"container": currentContainer,
		"ip":        agentHost,
	})

	if typ == delancey.DeleteStatus {
		// Delete path from the service.
		err = os.RemoveAll(fullPath)
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}
	} else {
		// Create/Update path in the service.
		if pathType == "dir" {
			err = os.MkdirAll(fullPath, os.ModePerm|os.ModeDir)
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
			err = os.MkdirAll(filepath.Dir(fullPath), os.ModePerm|os.ModeDir)
			if err != nil {
				renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}

			dest, err := os.Create(fullPath)
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

			err = os.Chmod(fullPath, os.FileMode(mode))
			if err != nil {
				renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}
		}
	}

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusUpdated,
	})
}

// PATCH /batch, Update the FS with a list of file changes. Only creates/updates
// can be done here.
func batchUpdateContainerHandler(rw http.ResponseWriter, req *http.Request) {
	// Require a container to exist.
	if currentContainer == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  delancey.ErrNotInUse.Error(),
		})
		return
	}

	go logClient.Info("batch updating container", map[string]interface{}{
		"container": currentContainer,
		"ip":        agentHost,
	})

	// Untar the tar contents from the body to the containers path.
	err := tar.Untar(req.Body, currentContainer.RemotePath)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusUpdated,
	})
}

// PUT /containers, Save service.
func saveContainerHandler(rw http.ResponseWriter, req *http.Request) {
	if currentContainer == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  delancey.ErrNotInUse.Error(),
		})
		return
	}

	if Env == "testing" {
		renderer.JSON(rw, http.StatusOK, map[string]string{
			"status": requests.StatusUpdated,
		})
		return
	}

	// Get the changes for the image.
	log.Println("Getting changes for container", currentContainer.ImageID)
	changes, err := DockerClient.Changes(currentContainer.DockerID, nil)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}
	image := config.DockerBaseImage + ":" + currentContainer.ImageID

	// No changes made so just return successfully.
	if len(changes) <= 0 {
		renderer.JSON(rw, http.StatusOK, map[string]string{
			"status": requests.StatusUpdated,
		})
		return
	}
	progChan := make(chan float64)
	prevProg := float64(0)

	// Get events for merge committing.
	go func() {
		for prog := range progChan {
			prog = prog / 2
			prevProg = prog

			val := "environment:" + strconv.FormatFloat(prog, 'e', -1, 64)
			pusherC.Publish(val, "progress", fmt.Sprintf("container-%s", currentContainer.ID))
		}
	}()

	log.Println("Committing image changes", currentContainer.ImageID)
	err = quay.CommitImage(DockerClient, currentContainer.DockerID, image, progChan)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}
	progChan = make(chan float64)
	if prevProg < 0.5 {
		prevProg = 0.5
	}

	// Get events for pushing.
	go func() {
		for prog := range progChan {
			prog = (prog / 2) + prevProg

			val := "environment:" + strconv.FormatFloat(prog, 'e', -1, 64)
			pusherC.Publish(val, "progress", fmt.Sprintf("container-%s", currentContainer.ID))
		}
	}()

	log.Println("Pushing image to hub", currentContainer.ImageID)
	err = DockerClient.PushImage(image, progChan)
	if err == nil {
		kenmare.UpdateImage(currentContainer.ImageID)
	}
	log.Println("Image push complete", currentContainer.ImageID)

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusUpdated,
	})
}

// DELETE /, Remove service.
func removeContainerHandler(rw http.ResponseWriter, req *http.Request) {
	// Container needs to exist.
	if currentContainer == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  delancey.ErrNotInUse.Error(),
		})
		return
	}

	go logClient.Info("removing container", map[string]interface{}{
		"container": currentContainer,
		"ip":        agentHost,
	})

	if Env != "testing" {
		// Get the container to remove the build image.
		log.Println("Inspecting container", currentContainer.ImageID)
		container, err := DockerClient.Inspect(currentContainer.DockerID)
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}

		// Remove the container and its image.
		log.Println("Removing container", currentContainer.ImageID)
		err = DockerClient.Remove(currentContainer.DockerID)
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}

		log.Println("Removing runner image", currentContainer.ImageID)
		err = DockerClient.RemoveImage(container.Image)
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}
	}

	// Remove the containers path/ssh and clean up the current container.
	os.RemoveAll(currentContainer.RemotePath)
	os.RemoveAll(currentContainer.SSHPath)
	currentContainer = nil
	SaveContainer()
	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusRemoved,
	})
}

// PUT /ssh, Accepts ssh tarfile for user auth to their container
func uploadSSHHandler(rw http.ResponseWriter, req *http.Request) {
	// Require a container to exist.
	if currentContainer == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  delancey.ErrNotInUse.Error(),
		})
		return
	}

	// Untar the tar contents from the body to the containers path.
	err := tar.Untar(req.Body, currentContainer.SSHPath)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	// Ensure files/directories are private.
	err = filepath.Walk(currentContainer.SSHPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || currentContainer.SSHPath == path {
			return err
		}

		if info.IsDir() {
			return os.Chmod(path, 0700)
		}

		return os.Chmod(path, 0600)
	})
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusSuccess,
	})
}

// GET /healthz, Return the status of the agent.
func healthzHandler(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "ok")
}

// GET /_/state/container, Return the current container data.
func containerStateHandler(rw http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(currentContainer)
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

// PUT /_/pull, Pulls an image down from Docker.
func pullImageHandler(rw http.ResponseWriter, req *http.Request) {
	image := req.FormValue("image")
	if image == "" {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "Image query param required",
		})
		return
	}

	// If not in format repo:tag, assume it's a tag for the default repo.
	if !strings.Contains(image, ":") {
		image = config.DockerBaseImage + ":" + image
	}

	err := DockerClient.PullImage(image, nil)
	if err != nil {
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusSuccess,
	})
}

// createImageInput creates a tar reader using a template as the Dockerfile.
func createImageInput(tmpl string, vars map[string]string) (io.Reader, error) {
	// Do replaces in the tmpl.
	if vars != nil {
		for key, value := range vars {
			tmpl = strings.Replace(tmpl, "{{"+key+"}}", value, -1)
		}
	}

	// Create the tar writer and the header for the Dockerfile.
	var buf bytes.Buffer
	tarW := stdtar.NewWriter(&buf)
	header := &stdtar.Header{
		Name: "Dockerfile",
		Size: int64(len(tmpl)),
		Mode: 0644,
	}

	// Write the entry to the tar writer closing afterwards.
	err = tarW.WriteHeader(header)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(tarW, strings.NewReader(tmpl))
	if err != nil {
		return nil, err
	}

	return &buf, tarW.Close()
}
