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
			"run":   "go run main.go",
		},
	}
}

func TestPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(pingHandler))
	defer server.Close()

	service.Address = server.URL

	if err := service.Ping(); err != nil {
		t.Fatal(err)
	}
}

func pingHandler(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte("ok"))
}
