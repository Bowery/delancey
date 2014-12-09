// Copyright 2014 Bowery, Inc.
package main

import (
	"os"
	"path/filepath"
	"testing"
)

var (
	testOutputWriter *OutputWriter
	testOutputPath   = filepath.Join("tmp", "output.txt")
	err              error
)

func TestNewOutputWriter(t *testing.T) {
	testOutputWriter, err = NewOutputWriter(testOutputPath)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWrite(t *testing.T) {
	_, err := testOutputWriter.Write([]byte(`Hello World`))
	if err != nil {
		t.Fatal(err)
	}

	os.RemoveAll("tmp")
}
