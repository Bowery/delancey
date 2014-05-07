// Copyright 2013-2014 Bowery, Inc.
package model

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var (
	service *Service
)

func init() {
	service = &Service{
		Address: "0.0.0.0",
		Commands: map[string]string{
			"build": "go get",
			"test":  "go test ./...",
			"start": "go run main.go",
		},
	}
}

func TestPingSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(pingSuccessHandler))
	defer server.Close()

	service.Address = server.URL

	if err := service.Ping(); err != nil {
		t.Fatal(err)
	}
}

func pingSuccessHandler(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte("ok"))
}

func TestPingFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(pingFailureHandler))
	defer server.Close()

	service.Address = server.URL

	err := service.Ping()
	if err == nil {
		t.Fatal(err)
	}

	if err.Error() != "Unexpected response." {
		t.Error("Bad response.")
	}
}

func pingFailureHandler(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte("bad"))
}
