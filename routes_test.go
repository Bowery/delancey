// Copyright 2014 Bowery, Inc.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Bowery/gopackages/requests"
	"github.com/Bowery/gopackages/schemas"
	"github.com/Bowery/gopackages/tar"
)

var Rcontainer = &schemas.Container{
	ID: "some-id",
}
var uploadPath = filepath.Join("test", "upload.tar.gz")

func init() {
	Env = "testing"
	err := os.MkdirAll("test", os.ModePerm|os.ModeDir)
	if err != nil {
		panic(err)
	}

	// If the tar file exists don't create it.
	_, err = os.Stat(uploadPath)
	if err == nil {
		return
	}

	// Create a gzipped tar file in test.
	file, err := os.Create(filepath.Join("test", "upload.tar.gz"))
	if err != nil {
		panic(err)
	}
	defer file.Close()

	contents, err := tar.Tar(".", nil)
	if err != nil {
		panic(err)
	}

	_, err = io.Copy(file, contents)
	if err != nil {
		panic(err)
	}
}

func TestUploadNoContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(UploadContainerHandler))
	defer server.Close()

	file, err := os.Open(uploadPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	res, err := http.Post(server.URL, "application/x-gzip", file)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	resData := new(requests.Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resData)
	if err != nil {
		t.Fatal(err)
	}

	if resData.Status != requests.StatusFailed {
		t.Error("Upload passed when it should've failed")
	}
}

func TestCreateContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(CreateContainerHandler))
	defer server.Close()

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	err := encoder.Encode(Rcontainer)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.Post(server.URL, "application/json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	containerRes := new(requests.ContainerRes)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(containerRes)
	if err != nil {
		t.Fatal(err)
	}

	if containerRes.Status != requests.StatusCreated {
		t.Error("Should've been created but failed")
	}

	Rcontainer.RemotePath = containerRes.Container.RemotePath
}

func TestCreateContainerCreated(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(CreateContainerHandler))
	defer server.Close()

	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	err := encoder.Encode(Rcontainer)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.Post(server.URL, "application/json", &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	containerRes := new(requests.ContainerRes)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(containerRes)
	if err != nil {
		t.Fatal(err)
	}

	if containerRes.Status == requests.StatusCreated {
		t.Error("Should've been failed but didn't")
	}

	if currentContainer == nil {
		t.Error("currentContainer should be set after create")
	}
}

func TestUpload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(UploadContainerHandler))
	defer server.Close()

	file, err := os.Open(uploadPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	res, err := http.Post(server.URL, "application/x-gzip", file)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	resData := new(requests.Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resData)
	if err != nil {
		t.Fatal(err)
	}

	if resData.Status != requests.StatusSuccess {
		t.Error("Upload failed when it should've passed")
	}
}

func TestUpdateDir(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(UpdateContainerHandler))
	defer server.Close()

	req, err := newUploadRequest(server.URL, nil, map[string]string{
		"pathtype": "dir",
		"path":     "newdir",
		"type":     "create",
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	resData := new(requests.Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resData)
	if err != nil {
		t.Fatal(err)
	}

	if resData.Status != requests.StatusUpdated {
		t.Error("Update failed but should've passed")
	}

	_, err = os.Stat(filepath.Join(Rcontainer.RemotePath, "newdir"))
	if err != nil {
		t.Error("newdir should exist but stat failed")
	}
}

func TestUpdateFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(UpdateContainerHandler))
	defer server.Close()

	req, err := newUploadRequest(server.URL, map[string]string{
		"file": uploadPath,
	}, map[string]string{
		"pathtype": "file",
		"path":     "somecoolfile",
		"type":     "create",
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	resData := new(requests.Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resData)
	if err != nil {
		t.Fatal(err)
	}

	if resData.Status != requests.StatusUpdated {
		t.Error("Update failed but should've passed")
	}

	_, err = os.Stat(filepath.Join(Rcontainer.RemotePath, "somecoolfile"))
	if err != nil {
		t.Error("somecoolfile should exist but stat failed")
	}
}

func TestUpdateDeleteFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(UpdateContainerHandler))
	defer server.Close()

	req, err := newUploadRequest(server.URL, nil, map[string]string{
		"pathtype": "file",
		"path":     "somecoolfile",
		"type":     "delete",
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	resData := new(requests.Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resData)
	if err != nil {
		t.Fatal(err)
	}

	if resData.Status != requests.StatusUpdated {
		t.Error("Update failed but should've passed")
	}

	_, err = os.Stat(filepath.Join(Rcontainer.RemotePath, "somecoolfile"))
	if err == nil {
		t.Error("somecoolfile shouldn't exist but stat passed")
	}
}

func TestRemoveContainer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(RemoveContainerHandler))
	defer server.Close()

	res, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	resData := new(requests.Res)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(resData)
	if err != nil {
		t.Fatal(err)
	}

	if resData.Status != requests.StatusRemoved {
		t.Error("Remove should've succeeded but didn't")
	}

	if currentContainer != nil {
		t.Error("currentContainer should be unset after remove")
	}
}

// newUploadRequest creates a new request with file uploads.
func newUploadRequest(url string, uploads map[string]string, params map[string]string) (*http.Request, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Write all given uploads.
	if uploads != nil {
		for k, p := range uploads {
			file, err := os.Open(p)
			if err != nil {
				return nil, err
			}
			defer file.Close()

			// Create a part for the form and copy contents.
			part, err := writer.CreateFormFile(k, filepath.Base(p))
			if err == nil {
				_, err = io.Copy(part, file)
			}
			if err != nil {
				return nil, err
			}
		}
	}

	// Write all the given params.
	if params != nil {
		for k, v := range params {
			err := writer.WriteField(k, v)
			if err != nil {
				return nil, err
			}
		}
	}
	writer.Close()

	// Just send POST, it doesn't matter since we're calling handers directly.
	req, err := http.NewRequest("POST", url, &body)
	if req != nil {
		req.Header.Set("Content-Type", writer.FormDataContentType())
	}

	return req, err
}
