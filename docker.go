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

// buildImage builds an image for the repo from a list of paths that should
// include a Dockerfile. If strip is true, the Dockerfile found is stripped of
// unsafe instructions. Progress is sent across the given channel.
func buildImage(strip bool, paths map[string]string, vars map[string]string, repo string, progress chan float64) (string, error) {
	dockerfile, ok := paths["Dockerfile"]
	if strip && ok {
		fileContents, err := stripInstructions(strings.NewReader(dockerfile))
		if err != nil {
			return "", err
		}

		paths["Dockerfile"] = fileContents.String()
		dockerfile = paths["Dockerfile"]
	}

	input, err := createImageInput(paths, vars)
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

// createImageInput creates a tar reader using the given templates as files.
// The given vars are replaced in all the templates encountered.
func createImageInput(tmpls, vars map[string]string) (io.Reader, error) {
	var buf bytes.Buffer
	tarW := stdtar.NewWriter(&buf)
	header := &stdtar.Header{
		Mode: 0644,
	}

	for path, tmpl := range tmpls {
		if vars != nil {
			for key, val := range vars {
				tmpl = strings.Replace(tmpl, "{{"+key+"}}", val, -1)
			}
		}

		header.Name = path
		header.Size = int64(len(tmpl))
		err := tarW.WriteHeader(header)
		if err != nil {
			return nil, err
		}

		_, err = io.Copy(tarW, strings.NewReader(tmpl))
		if err != nil {
			return nil, err
		}
	}

	return &buf, tarW.Close()
}

// stripInstructions reads Dockerfile input and strips unsafe or disallowed
// instructions. It also only allows URL based sources for the ADD instruction.
func stripInstructions(contents io.Reader) (*bytes.Buffer, error) {
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
