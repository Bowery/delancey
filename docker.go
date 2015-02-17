package main

import (
	stdtar "archive/tar"
	"bytes"
	"io"
	"log"
	"strings"

	"github.com/Bowery/gopackages/docker"
)

// createImage creates the given image from a base image.
func createImage(imageID, image, baseImage string) error {
	log.Println("Creating build container using", baseImage, "for", imageID)
	id, err := DockerClient.Create(new(docker.Config), baseImage, nil)
	if err != nil {
		return err
	}

	// Commit the empty container to the image name.
	log.Println("Commit build container to image", image)
	err = DockerClient.CommitImage(id, image)
	if err != nil {
		return err
	}

	// Clean up the container.
	log.Println("Removing build container", imageID)
	go DockerClient.Remove(id)
	return nil
}

// buildImage builds an image for the repo from a given Dockerfile, sending step
// progress across the channel.
func buildImage(dockerfile string, vars map[string]string, repo string, progress chan float64) (string, error) {
	input, err := createImageInput(dockerfile, vars)
	if err != nil {
		return "", err
	}

	if progress == nil {
		return DockerClient.BuildImage(input, "", repo, nil)
	}

	steps, err := docker.ParseDockerfile(strings.NewReader(dockerfile))
	if err != nil {
		return "", err
	}
	progChan := make(chan int)
	stepsNum := float64(len(steps))

	go func() {
		for prog := range progChan {
			progress <- (float64(prog) + 1) / stepsNum
		}
	}()

	return DockerClient.BuildImage(input, "", repo, progChan)
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
