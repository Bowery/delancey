package main

import (
	stdtar "archive/tar"
	"bytes"
	"io"
	"log"
	"net/url"
	"strings"

	"github.com/Bowery/gopackages/docker"
	"github.com/docker/docker/builder/command"
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

	fileContents, err := stripInstructions(strings.NewReader(dockerfile))
	if err != nil {
		return "", err
	}

	steps, err := docker.ParseDockerfile(fileContents)
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

// stripInstructions reads Dockerfile input and strips unsafe or disallowed
// instructions. It also only allows URL based sources for the ADD instruction.
func stripInstructions(contents io.Reader) (io.Reader, error) {
	var buf bytes.Buffer
	nodes, err := docker.ParseDockerfile(contents)
	if err != nil {
		return nil, err
	}

	for _, node := range nodes {
		// Commands disabled, either for security or because they use features
		// disallowed(like local file copying).
		if node.Value == command.Cmd || node.Value == command.Copy ||
			node.Value == command.Entrypoint || node.Value == command.Volume ||
			node.Value == command.User || node.Value == command.Onbuild {
			continue
		}

		// Only allow ADD instructions using URLs for src paths.
		if node.Value == command.Add {
			skip := false
			n := node.Next

			// Descend the src paths, Value field is the path.
			for n != nil {
				// Skip the dest(last) path.
				if n.Next == nil {
					break
				}

				parsedURL, err := url.Parse(n.Value)
				if err != nil {
					skip = true
					break
				}

				if parsedURL.Scheme == "" || parsedURL.Scheme == "file" {
					skip = true
					break
				}

				n = n.Next
			}

			if skip {
				continue
			}
		}

		buf.Write([]byte(node.Original + "\n"))
	}

	return &buf, nil
}
