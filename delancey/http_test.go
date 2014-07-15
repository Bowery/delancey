// Copyright 2013-2014 Bowery, Inc.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/Bowery/gopackages/schemas"
)

func init() {
	ENV = "development"
	ServiceDir = filepath.Join("test", "application")
	tarPath := filepath.Join("test", "file.tar.gz")

	err := os.MkdirAll("test", os.ModePerm|os.ModeDir)
	if err != nil {
		panic(err)
	}

	// If the tar file exists don't create it.
	_, err = os.Stat(tarPath)
	if err == nil {
		return
	}

	// Create a gzipped tar file in test.
	file, err := os.Create(filepath.Join("test", "file.tar.gz"))
	if err != nil {
		panic(err)
	}
	defer file.Close()
	gzipper := gzip.NewWriter(file)
	defer gzipper.Close()
	tarWriter := tar.NewWriter(gzipper)
	defer tarWriter.Close()

	// Contents to copy to tar.
	contents, err := os.Open("http_test.go")
	if err != nil {
		panic(err)
	}
	defer contents.Close()

	// Create header for file.
	stats, err := contents.Stat()
	if err != nil {
		panic(err)
	}
	header, err := tar.FileInfoHeader(stats, "")
	if err != nil {
		panic(err)
	}

	// Copy contents.
	err = tarWriter.WriteHeader(header)
	if err == nil {
		_, err = io.Copy(tarWriter, contents)
	}
	if err != nil {
		panic(err)
	}
}

func TestUploadWithFile(t *testing.T) {
	apiServer := getApiServer()
	defer apiServer.Close()
	server := httptest.NewServer(http.HandlerFunc(UploadServiceHandler))
	defer server.Close()

	req, err := newUploadRequest(server.URL, map[string]string{
		"file": filepath.Join("test", "file.tar.gz"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	response := new(API)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(response)
	if err != nil {
		t.Fatal(err)
	}

	if response.Status == "failed" {
		t.Error(response.Err)
	}
}

func TestUploadNoFile(t *testing.T) {
	apiServer := getApiServer()
	defer apiServer.Close()
	server := httptest.NewServer(http.HandlerFunc(UploadServiceHandler))
	defer server.Close()

	req, err := newUploadRequest(server.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	response := new(API)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(response)
	if err != nil {
		t.Fatal(err)
	}

	if response.Status == "failed" {
		t.Error(response.Err)
	}
}

func TestUploadNonTar(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(UploadServiceHandler))
	defer server.Close()

	req, err := newUploadRequest(server.URL, map[string]string{
		"file": "http_test.go",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	response := new(API)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(response)
	if err != nil {
		t.Fatal(err)
	}

	if response.Status != "failed" {
		t.Error("Response should have failed, but didn't.")
	}
}

func TestUpdateSuccess(t *testing.T) {
	apiServer := getApiServer()
	defer apiServer.Close()
	server := httptest.NewServer(http.HandlerFunc(UpdateServiceHandler))
	defer server.Close()

	req, err := newUploadRequest(server.URL, map[string]string{
		"file": "http_test.go",
	}, map[string]string{
		"type": "update",
		"path": "http_test.go",
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	response := new(API)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(response)
	if err != nil {
		t.Fatal(err)
	}

	if response.Status == "failed" {
		t.Error(response.Err)
	}
}

func TestUpdateNoFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(UpdateServiceHandler))
	defer server.Close()

	req, err := newUploadRequest(server.URL, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	response := new(API)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(response)
	if err != nil {
		t.Fatal(err)
	}

	if response.Status != "failed" {
		t.Error("Response should have failed, but didn't.")
	}
}

func TestUpdateNoUpload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(UpdateServiceHandler))
	defer server.Close()

	req, err := newUploadRequest(server.URL, nil, map[string]string{
		"type": "update",
		"path": "http_test.go",
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	response := new(API)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(response)
	if err != nil {
		t.Fatal(err)
	}

	if response.Status != "failed" {
		t.Error("Response should have failed, but didn't.")
	}
}

func TestDeleteSuccess(t *testing.T) {
	apiServer := getApiServer()
	defer apiServer.Close()
	server := httptest.NewServer(http.HandlerFunc(UpdateServiceHandler))
	defer server.Close()

	req, err := newUploadRequest(server.URL, nil, map[string]string{
		"type": "delete",
		"path": "http_test.go",
	})
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	response := new(API)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(response)
	if err != nil {
		t.Fatal(err)
	}

	if response.Status == "failed" {
		t.Error(response.Err)
	}
}

func TestServicesNoFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(UpdateServicesHandler))
	defer server.Close()

	res, err := http.PostForm(server.URL, make(url.Values))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	response := new(API)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(response)
	if err != nil {
		t.Fatal(err)
	}

	if response.Status != "failed" {
		t.Error("Response should have failed, but didn't.")
	}
}

func TestServicesSuccess(t *testing.T) {
	apiServer := getApiServer()
	defer apiServer.Close()
	server := httptest.NewServer(http.HandlerFunc(UpdateServicesHandler))
	defer server.Close()

	var body bytes.Buffer
	encoder := json.NewEncoder(&body)
	err := encoder.Encode(API{Services: []*schemas.Service{{
		"",
		"test",
		"1.2.3.4",
		"",
		"",
		"",
		"",
		map[string]string{"27017": "1.2.3.4:27017"},
		"", "", "", "", map[string]string{"ENV": "development"}, "",
	}}})
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.Post(server.URL, "application/json", &body)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	response := new(API)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(response)
	if err != nil {
		t.Fatal(err)
	}

	if response.Status == "failed" {
		t.Error(response.Err)
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

// getApiServer creates a server setting BasePath to it's url.
func getApiServer() *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(getAppHandler))
	BasePath = server.URL
	return server
}

// getAppHandler acts as the api route to retrieve an application.
func getAppHandler(rw http.ResponseWriter, req *http.Request) {
	services := []*schemas.Service{
		{Name: ServiceName},
	}

	body, _ := json.Marshal(API{
		Status: "found",
		Application: &schemas.Application{
			ID:       ApplicationID,
			Services: services,
		},
		Services: services,
	})

	rw.Header().Set("Content-Type", "application/json")
	rw.Write(body)
}
