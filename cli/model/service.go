// Copyright 2013-2014 Bowery, Inc.
package model

import (
	"errors"
	"io/ioutil"
	"net/http"
)

type Service struct {
	// The location of the service.
	Address string `address`

	// A map of commands.
	Commands map[string]string `commands`
}

// Ping the service. If the machine is inaccessible
// or does not respond as expected, return error.
func (s *Service) Ping() error {
	res, err := http.Get(s.Address + "/ping")
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
