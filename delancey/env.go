// Copyright 2013-2014 Bowery, Inc.
// Contains routines to manage services and the enviromment.
package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ServiceList retrieves the list of services for the application, and
// sets the environment.
func ServiceList() error {
	app, err := GetApplication()
	if err != nil {
		return err
	}

	return SetEnv(app.Services)
}

// GetApplication retrieves the application.
func GetApplication() (*Application, error) {
	res, err := http.Get(BasePath + "/" + APIApplicationPath)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Decode JSON to struct.
	data := new(API)
	decoder := json.NewDecoder(res.Body)
	err = decoder.Decode(data)
	if err != nil {
		return nil, err
	}

	// If not found, return the response error.
	if data.Status != "found" {
		return nil, data
	}

	return data.Application, nil
}

// SetEnv setsthe service environment variables and writes them to a
// .htaccess file.
func SetEnv(services []*Service) error {
	envs := make(map[string]string)

	// Get the env names and values.
	for _, service := range services {
		// Normalize by replacing whitespace and dashes with underscores.
		name := strings.Trim(strings.ToUpper(service.Name), " ")
		for _, c := range "-\f\n\r\t\v\u00A0\u2028\u2029" {
			name = strings.Replace(name, string(c), "_", -1)
		}

		envs[name+"_ADDR"] = service.PublicAddr

		// Set up custom ports (e.g. API_PORT_2552_ADDR)
		if len(service.CustomPorts) > 0 {
			for port, addr := range service.CustomPorts {
				envs[name+"_PORT_"+port+"_ADDR"] = addr
			}
		}
	}

	for name, val := range envs {
		err := os.Setenv(name, val)
		if err != nil {
			return err
		}
	}

	// Read from htaccess.
	lines := make([]string, 0)
	found := make([]string, 0)
	htaccess, err := os.Open(filepath.Join(ServiceDir, ".htaccess"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if htaccess != nil {
		// Read lines, and replace vars with new address.
		scanner := bufio.NewScanner(htaccess)
		for scanner.Scan() {
			line := scanner.Text()

			// Check if the line is an env var we set.
			for name, val := range envs {
				if strings.Contains(line, "SetEnv "+name+" ") {
					line = "SetEnv " + name + " " + val
					found = append(found, name)
					break
				}
			}

			lines = append(lines, line)
		}
		err = scanner.Err()
		if err != nil {
			return err
		}

		err = htaccess.Close()
		if err != nil {
			return err
		}
	}

	// Append new vars to the lines.
	for _, name := range found {
		delete(envs, name)
	}
	for name, val := range envs {
		lines = append(lines, "SetEnv "+name+" "+val)
	}
	// Write the lines back to htaccess.
	data := []byte(strings.Join(lines, "\n") + "\n")
	htaccess, err = os.Create(filepath.Join(ServiceDir, ".htaccess"))
	if err != nil {
		return err
	}
	defer htaccess.Close()

	_, err = htaccess.Write(data)
	return err
}

// Retry a function until it passes or reaches a given limit.
func Retry(run func() error, limit int) error {
	var err error

	for i := 0; i < limit; i++ {
		if err != nil {
			<-time.After(time.Millisecond * time.Duration(i*i*100))
		}

		err = run()
		if err == nil {
			return err
		}
	}

	return err
}
