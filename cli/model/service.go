// Copyright 2013-2014 Bowery, Inc.
package model

import (
	"Bowery/Mir/cli/tar"
	"bytes"
	"cli/log"
	"errors"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

type Service struct {
	// The location of the service.
	Address string `address`

	// A map of commands.
	Commands map[string]string `commands`

	// Location of files.
	Path string `path`
}

// Ping the service. If the machine is inaccessible
// or does not respond as expected, return error.
func (s *Service) Ping() error {
	res, err := http.Get("http://" + s.Address + "/ping")
	if err != nil {
		return err
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	content := string(data[:])
	if content != "ok" {
		return errors.New("Unexpected response.")
	}

	return nil
}

func (s *Service) Upload() error {
	// Package the upload.
	path := filepath.Join(".crosswalk", "upload.tgz")
	upload, err := tar.Tar(s.Path)
	if err != nil {
		return err
	}

	// Create .crosswalk dir.
	if err = os.MkdirAll(".crosswalk", 0755); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		log.Debug(err)
		return err
	}

	defer os.RemoveAll(path)
	defer file.Close()

	_, err = io.Copy(file, upload)
	if err != nil {
		return err
	}

	_, err = file.Seek(0, os.SEEK_SET)
	if err != nil {
		return err
	}

	// Create request.
	var body bytes.Buffer

	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "upload")
	if err != nil {
		return err
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}

	res, err := http.Post("http://"+s.Address, writer.FormDataContentType(), &body)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// TODO(steve): handle response

	return nil
}

func (s *Service) Update(name, status string) error {
	// Ignore anything in .crosswalk
	path := filepath.Join(s.Path, name)
	if path == ".crosswalk/upload.tgz" {
		return nil
	}

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)
	err := writer.WriteField("type", status)
	if err == nil {
		err = writer.WriteField("path", name)
	}
	if err != nil {
		return err
	}

	if status == "update" || status == "create" {
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		part, err := writer.CreateFormFile("file", "upload")
		if err != nil {
			return err
		}

		_, err = io.Copy(part, file)
		if err != nil {
			return err
		}
	}

	if err = writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", "http://"+s.Address, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// TODO(steve): handle response

	return nil
}
