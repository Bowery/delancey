// Copyright 2014 Bowery, Inc.

package delancey

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/Bowery/gopackages/config"
	"github.com/Bowery/gopackages/path"
	"github.com/Bowery/gopackages/requests"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/tar"
)

// Status event types that are used for updating.
const (
	CreateStatus       = "create"
	UpdateStatus       = "update"
	DeleteStatus       = "delete"
	UploadStartStatus  = "upload-start"
	UploadFinishStatus = "upload-finish"
	BatchStartStatus   = "batch-start"
	BatchFinishStatus  = "batch-finish"
)

// Errors that may occur.
var (
	ErrInUse    = errors.New("This Delancey instance is in use")
	ErrNotInUse = errors.New("This Delancey instance is not in use")
)

// BatchError is used when the batch update encounters an error but
// just skips it instead of treating it as fatal.
type BatchError struct {
	Path string
	Err  error
}

func (be *BatchError) Error() string {
	return be.Err.Error()
}

// Download retrieves the containers contents on the instance.
func Download(container *schemas.Container) (io.Reader, error) {
	addr := net.JoinHostPort(container.Address, config.DelanceyProdPort)
	res, err := http.Get("http://" + addr)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Decode failure response.
	if res.StatusCode != http.StatusOK {
		resData := new(requests.Res)
		decoder := json.NewDecoder(res.Body)
		err = decoder.Decode(resData)
		if err != nil {
			return nil, err
		}

		// If the error matches return var.
		if resData.Error() == ErrNotInUse.Error() {
			err = ErrNotInUse
		}

		return nil, err
	}

	body := new(bytes.Buffer)
	_, err = io.Copy(body, res.Body)
	return body, err
}

// Create creates the given container on the instance using a dockerfile
// as the base if given.
func Create(container *schemas.Container, dockerfile string) error {
	var body bytes.Buffer
	reqContainer := &requests.DockerfileContainerReq{
		Container:  container,
		Dockerfile: dockerfile,
	}

	encoder := json.NewEncoder(&body)
	err := encoder.Encode(reqContainer)
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(container.Address, config.DelanceyProdPort)
	res, err := http.Post("http://"+addr, "application/json", &body)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	containerRes := new(requests.ContainerRes)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(containerRes)
	if err != nil {
		return err
	}

	if containerRes.Status != requests.StatusCreated {
		// If the error matches return var.
		if containerRes.Error() == ErrInUse.Error() {
			return ErrInUse
		}

		return containerRes
	}

	container.DockerID = containerRes.Container.DockerID
	container.RemotePath = containerRes.Container.RemotePath
	container.SSHPath = containerRes.Container.SSHPath
	container.ContainerPath = containerRes.Container.ContainerPath
	container.User = containerRes.Container.User
	container.Password = containerRes.Container.Password
	return nil
}

// Upload uploads the given reader to the instance.
func Upload(container *schemas.Container, contents io.Reader) error {
	addr := net.JoinHostPort(container.Address, config.DelanceyProdPort)
	req, err := http.NewRequest("PUT", "http://"+addr, contents)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resData := new(requests.Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resData)
	if err != nil {
		return err
	}

	if resData.Status != requests.StatusSuccess {
		// If the error matches return var.
		if resData.Error() == ErrNotInUse.Error() {
			return ErrNotInUse
		}

		return resData
	}

	return nil
}

// Update updates the given path to the instance.
func Update(container *schemas.Container, full, name, status string) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	err := writer.WriteField("type", status)
	if err != nil {
		return err
	}

	err = writer.WriteField("path", path.RelUnix(name))
	if err != nil {
		return err
	}

	// Attach file if update/create status.
	if status == UpdateStatus || status == CreateStatus {
		file, err := os.Open(full)
		if err != nil {
			return err
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			return err
		}

		// Add the files permissions from stats mode.
		err = writer.WriteField("mode", strconv.FormatUint(uint64(stat.Mode().Perm()), 10))
		if err != nil {
			return err
		}

		// Get the file type from stat.
		pathType := "file"
		if stat.IsDir() {
			pathType = "dir"
		}
		err = writer.WriteField("pathtype", pathType)
		if err != nil {
			return err
		}

		// Add the contents if it's a directory.
		if pathType == "file" {
			part, err := writer.CreateFormFile("file", "upload")
			if err != nil {
				return err
			}

			_, err = io.Copy(part, file)
			if err != nil {
				return err
			}
		}
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(container.Address, config.DelanceyProdPort)
	req, err := http.NewRequest("PATCH", "http://"+addr, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resData := new(requests.Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resData)
	if err != nil {
		return err
	}

	if resData.Status != requests.StatusUpdated {
		// If the error matches return var.
		if resData.Error() == ErrNotInUse.Error() {
			return ErrNotInUse
		}

		return resData
	}

	return nil
}

// BatchUpdate updates a list of paths to the instance. Only update/create
// events should be included.
func BatchUpdate(container *schemas.Container, paths map[string]string, errorChan chan error) error {
	body, gzipWriter, tarWriter := tar.NewTarGZ()

	for full, rel := range paths {
		info, err := os.Lstat(full)
		if err != nil {
			if os.IsNotExist(err) {
				errorChan <- &BatchError{Path: full, Err: err}
				continue
			}

			return err
		}

		err = tar.WritePath(tarWriter, info, full, rel)
		if err != nil {
			if os.IsNotExist(err) {
				errorChan <- &BatchError{Path: full, Err: err}
				continue
			}

			return err
		}
	}
	tarWriter.Close()
	gzipWriter.Close()

	addr := net.JoinHostPort(container.Address, config.DelanceyProdPort)
	req, err := http.NewRequest("PATCH", "http://"+addr+"/batch", body)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resData := new(requests.Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resData)
	if err != nil {
		return err
	}

	if resData.Status != requests.StatusUpdated {
		// If the error matches return var.
		if resData.Error() == ErrNotInUse.Error() {
			return ErrNotInUse
		}

		return resData
	}

	return nil
}

// Save commits and pushes the current container on the instance.
func Save(container *schemas.Container) error {
	addr := net.JoinHostPort(container.Address, config.DelanceyProdPort)
	req, err := http.NewRequest("PUT", "http://"+addr+"/containers", nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resData := new(requests.Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resData)
	if err != nil {
		return err
	}

	if resData.Status != requests.StatusUpdated {
		// If the error matches return var.
		if resData.Error() == ErrNotInUse.Error() {
			return ErrNotInUse
		}

		return resData
	}

	return nil
}

// Delete removes the container from the instance.
func Delete(container *schemas.Container) error {
	addr := net.JoinHostPort(container.Address, config.DelanceyProdPort)
	req, err := http.NewRequest("DELETE", "http://"+addr, nil)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resData := new(requests.Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resData)
	if err != nil {
		return err
	}

	if resData.Status != requests.StatusRemoved {
		// If the error matches return var.
		if resData.Error() == ErrNotInUse.Error() {
			return ErrNotInUse
		}

		return resData
	}

	return nil
}

// UploadSSH sends the .ssh directory to the container
func UploadSSH(container *schemas.Container, path string) error {
	contents, err := tar.Tar(path, []string{})
	if err != nil {
		return err
	}

	addr := net.JoinHostPort(container.Address, config.DelanceyProdPort)
	req, err := http.NewRequest("PUT", "http://"+addr+"/ssh", contents)
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resData := new(requests.Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resData)
	if err != nil {
		return err
	}

	if resData.Status != requests.StatusSuccess {
		// If the error matches return var.
		if resData.Error() == ErrNotInUse.Error() {
			return ErrNotInUse
		}

		return resData
	}

	return nil
}

// Health checks if a delancey instance is running.
func Health(addr string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}

	addr = net.JoinHostPort(addr, config.DelanceyProdPort)
	res, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return http.ErrNotSupported
	}

	return nil
}

// PullImage tells a delancey instance to pull an image.
func PullImage(addr, image string) error {
	addr = net.JoinHostPort(addr, config.DelanceyProdPort)
	res, err := http.PostForm("http://"+addr+"/_/pull", url.Values{"image": {image}})
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resData := new(requests.Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resData)
	if err != nil {
		return err
	}

	if resData.Status != requests.StatusSuccess {
		return resData
	}

	return nil
}
