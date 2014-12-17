// Copyright 2013-2014 Bowery, Inc.
package main

import (
	stdtar "archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"code.google.com/p/go-uuid/uuid"
	"github.com/Bowery/delancey/delancey"
	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/docker"
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
	//HomeDir          = os.Getenv(sys.HomeVar)
	HomeDir          = "/home/ubuntu"
	BoweryDir        = filepath.Join(HomeDir, ".bowery")
	SSHDir           = filepath.Join(BoweryDir, "ssh")
	CurrentContainer *schemas.Container
)

var renderer = render.New(render.Options{
	IndentJSON:    true,
	IsDevelopment: true,
})

// List of named routes.
var Routes = []web.Route{
	{"GET", "/", IndexHandler, false},
	{"POST", "/", CreateContainerHandler, false},
	{"PUT", "/", UploadContainerHandler, false},
	{"PATCH", "/", UpdateContainerHandler, false},
	{"DELETE", "/", RemoveContainerHandler, false},
	{"PUT", "/ssh", UploadSSHHandler, false},
	{"GET", "/healthz", HealthzHandler, false},
	{"GET", "/_/state/container", ContainerStateHandler, false},
}

// GET /, Retrieve the containers code.
func IndexHandler(rw http.ResponseWriter, req *http.Request) {
	var (
		contents io.Reader
		err      error
	)
	empty, gzipWriter, tarWriter := tar.NewTarGZ()
	tarWriter.Close()
	gzipWriter.Close()

	// Require a container to exist.
	if CurrentContainer == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  delancey.ErrNotInUse.Error(),
		})
		return
	}

	// Tar the contents of the container.
	contents, err = tar.Tar(CurrentContainer.RemotePath, []string{})
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
func CreateContainerHandler(rw http.ResponseWriter, req *http.Request) {
	// Only allow one container at a time.
	if CurrentContainer != nil {
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
		"ip":        AgentHost,
	})

	// Create new Container.
	_, err = NewContainer(container)
	if err != nil {
		go logClient.Error(err.Error(), map[string]interface{}{
			"container": container,
			"ip":        AgentHost,
		})
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	image := config.DockerBaseImage + ":" + container.ImageID
	sshPath := filepath.Join(SSHDir, container.ID)
	err = os.MkdirAll(sshPath, os.ModePerm|os.ModeDir)
	if err != nil {
		go logClient.Error(err.Error(), map[string]interface{}{
			"container": container,
			"ip":        AgentHost,
		})
		renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
			"status": requests.StatusFailed,
			"error":  err.Error(),
		})
		return
	}

	if Env != "testing" {
		// Pull down the containers image.
		err = DockerClient.PullImage(image)
		if err != nil && !docker.IsTagNotFound(err) {
			go logClient.Error(err.Error(), map[string]interface{}{
				"container": container,
				"ip":        AgentHost,
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
			id, err := DockerClient.Create(new(docker.Config), config.DockerBaseImage, nil)
			if err != nil {
				go logClient.Error(err.Error(), map[string]interface{}{
					"container": container,
					"ip":        AgentHost,
				})
				renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}

			// Commit the empty container.
			err = DockerClient.CommitImage(id, image)
			if err != nil {
				go logClient.Error(err.Error(), map[string]interface{}{
					"container": container,
					"ip":        AgentHost,
				})
				renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}

			go DockerClient.PushImage(image)

			err = DockerClient.Remove(id)
			if err != nil {
				go logClient.Error(err.Error(), map[string]interface{}{
					"container": container,
					"ip":        AgentHost,
				})
				renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}
		}

		// Build the image to use for the container, which sets the password.
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
				"ip":        AgentHost,
			})
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}

		image, err = DockerClient.BuildImage(input, "", config.DockerBaseImage)
		if err != nil {
			go logClient.Error(err.Error(), map[string]interface{}{
				"container": container,
				"ip":        AgentHost,
			})
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}
		config := &docker.Config{
			Volumes: map[string]string{
				container.RemotePath: "/root",
				sshPath:              "/root/.ssh",
			},
			NetworkMode: "host",
		}

		id, err := DockerClient.Create(config, image, []string{"/usr/sbin/sshd", "-D"})
		if err != nil {
			go logClient.Error(err.Error(), map[string]interface{}{
				"container": container,
				"ip":        AgentHost,
			})
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}

		err = DockerClient.Start(config, id)
		if err != nil {
			go logClient.Error(err.Error(), map[string]interface{}{
				"container": container,
				"ip":        AgentHost,
			})
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}

		container.User = user
		container.Password = password
		container.DockerID = id
	}

	CurrentContainer = container
	SaveContainer()
	renderer.JSON(rw, http.StatusOK, map[string]interface{}{
		"status":    requests.StatusCreated,
		"container": container,
	})
}

// PATCH /, Upload code for container.
func UploadContainerHandler(rw http.ResponseWriter, req *http.Request) {
	// Require a container to exist.
	if CurrentContainer == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  delancey.ErrNotInUse.Error(),
		})
		return
	}

	// Untar the tar contents from the body to the containers path.
	err := tar.Untar(req.Body, CurrentContainer.RemotePath)
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

// PUT /, Update the FS with a file change.
func UpdateContainerHandler(rw http.ResponseWriter, req *http.Request) {
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
	path := req.FormValue("path")
	typ := req.FormValue("type")
	modeStr := req.FormValue("mode")
	if path == "" || typ == "" {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  "Missing form fields.",
		})
		return
	}
	path = filepath.Join(CurrentContainer.RemotePath, filepath.Join(strings.Split(path, "/")...))

	// Container needs to exist.
	if CurrentContainer == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  delancey.ErrNotInUse.Error(),
		})
		return
	}

	go logClient.Info("updating container", map[string]interface{}{
		"container": CurrentContainer,
		"ip":        AgentHost,
	})

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

			dest, err := os.Create(path)
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

	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusUpdated,
	})
}

// DELETE /, Remove service.
func RemoveContainerHandler(rw http.ResponseWriter, req *http.Request) {
	skipCommit := req.FormValue("skip") != ""

	// Container needs to exist.
	if CurrentContainer == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  delancey.ErrNotInUse.Error(),
		})
		return
	}

	go logClient.Info("removing container", map[string]interface{}{
		"container": CurrentContainer,
		"ip":        AgentHost,
	})

	if Env != "testing" {
		if skipCommit {
			// Get the changes for the image.
			changes, err := DockerClient.Changes(CurrentContainer.DockerID, nil)
			if err != nil {
				renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
					"status": requests.StatusFailed,
					"error":  err.Error(),
				})
				return
			}
			image := config.DockerBaseImage + ":" + CurrentContainer.ImageID

			// Push changes up.
			if len(changes) > 0 {
				err = DockerClient.CommitImage(CurrentContainer.DockerID, image)
				if err != nil {
					renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
						"status": requests.StatusFailed,
						"error":  err.Error(),
					})
					return
				}

				// Push in parallel and then send the image to Kenmare to signal an
				// update completed.
				go func(id string) {
					err := DockerClient.PushImage(image)
					if err == nil {
						kenmare.UpdateImage(id)
					}
				}(CurrentContainer.ImageID)
			}
		}

		// Get the container to remove the build image.
		container, err := DockerClient.Inspect(CurrentContainer.DockerID)
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}

		// Remove the container and its image.
		err = DockerClient.Remove(CurrentContainer.DockerID)
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}

		err = DockerClient.RemoveImage(container.Image)
		if err != nil {
			renderer.JSON(rw, http.StatusInternalServerError, map[string]string{
				"status": requests.StatusFailed,
				"error":  err.Error(),
			})
			return
		}
	}

	// Remove the containers path and clean up the current container.
	os.RemoveAll(CurrentContainer.RemotePath)
	CurrentContainer = nil
	SaveContainer()
	renderer.JSON(rw, http.StatusOK, map[string]string{
		"status": requests.StatusRemoved,
	})
}

// GET /_/state/container, Return the current container data.
func ContainerStateHandler(rw http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(CurrentContainer)
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

// GET /healthz, Return the status of the agent.
func HealthzHandler(rw http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(rw, "ok")
}

// PUT /ssh, Accepts ssh tarfile for user auth to their container
func UploadSSHHandler(rw http.ResponseWriter, req *http.Request) {
	// Require a container to exist.
	if CurrentContainer == nil {
		renderer.JSON(rw, http.StatusBadRequest, map[string]string{
			"status": requests.StatusFailed,
			"error":  delancey.ErrNotInUse.Error(),
		})
		return
	}

	// Untar the tar contents from the body to the containers path.
	err := tar.Untar(req.Body, filepath.Join(SSHDir, CurrentContainer.ID))
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
