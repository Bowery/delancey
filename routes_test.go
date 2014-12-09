// Copyright 2014 Bowery, Inc.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Bowery/delancey/plugin"
)

func init() {
	Applications["some-app-id"] = &Application{
		ID: "some-app-id",
	}

	plugin.SetPluginManager()
	plugin.PluginDir = filepath.Join("test", "plugins")
	tarPath := filepath.Join("test", "plugin.tar.gz")

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
	file, err := os.Create(filepath.Join("test", "plugin.tar.gz"))
	if err != nil {
		panic(err)
	}
	defer file.Close()
	gzipper := gzip.NewWriter(file)
	defer gzipper.Close()
	tarWriter := tar.NewWriter(gzipper)
	defer tarWriter.Close()

	// Contents to copy to tar.
	contents, err := os.Open(filepath.Join("test", "plugin", "plugin.json"))
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

func TestUploadPluginHandlerWithNoName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(UploadPluginHandler))
	defer server.Close()

	req, err := newUploadRequest(server.URL, map[string]string{
		"file": filepath.Join("test", "plugin.tar.gz"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Error("Accepted a plugin with no name provided.")
	}
}

func TestUploadPluginHandlerWithValidRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(UploadPluginHandler))
	defer server.Close()

	req, err := newUploadRequest(server.URL, map[string]string{
		"file": filepath.Join("test", "plugin.tar.gz"),
	}, map[string]string{
		"appID": "some-app-id",
		"name":  "test-plugin",
	})

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Error("Failed to add plugin.")
	}
}

func TestUpdatePluginHandlerWithNoName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(UpdatePluginHandler))
	defer server.Close()

	req, err := http.NewRequest("PUT", server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Error("Accepted a plugin with no name provided.")
	}
}

func TestUpdatePluginHandlerWithValidRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(UpdatePluginHandler))
	defer server.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("appID", "some-app-id")
	writer.WriteField("name", "test-plugin")
	writer.WriteField("isEnabled", "true")
	writer.Close()

	req, err := http.NewRequest("PUT", server.URL, &body)
	if err != nil {
		t.Fatal(err)
	}
	if req != nil {
		req.Header.Set("Content-Type", writer.FormDataContentType())
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Error("Failed to update plugin.")
	}
}

func TestRemovePluginHandlerWithInvalidQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(RemovePluginHandler))
	defer server.Close()

	req, err := http.NewRequest("DELETE", server.URL+"?foo=bar", nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Error("Completed request with bad query.")
	}
}

func TestRemovePluginHandlerWithValidRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(RemovePluginHandler))
	defer server.Close()

	req, err := http.NewRequest("DELETE", server.URL+"?name=test-plugin", nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Error("Response not ok")
	}
}

func TestGetHealthz(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(HealthzHandler))
	defer server.Close()

	res, err := http.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		t.Error("Status Code of Healthz was not 200.")
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Fatal("Unable to read response body.")
	}

	if string(body) != "ok" {
		t.Error("Healthz body was not ok.")
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
