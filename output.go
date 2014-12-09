// Copyright 2014 Bowery, Inc.
package main

import (
	"os"
	"path/filepath"
	"sync"
)

// OutputWriter is a writer and buffer.
type OutputWriter struct {
	file  *os.File
	mutex sync.Mutex
}

// NewOutputWriter creates a new OutputWriter at the specified path.
func NewOutputWriter(outputPath string) (*OutputWriter, error) {
	// Ensure the parent directory has been generated.
	err := os.MkdirAll(filepath.Dir(outputPath), os.ModePerm|os.ModeDir)
	if err != nil {
		return nil, err
	}

	out, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	// Return OutputWriter
	return &OutputWriter{file: out}, nil
}

// Write implements io.Writer.
func (ow *OutputWriter) Write(b []byte) (int, error) {
	ow.mutex.Lock()
	defer ow.mutex.Unlock()

	n, err := ow.file.Write(b)
	if err != nil {
		return n, err
	}

	return n, ow.file.Sync()
}

func (ow *OutputWriter) Close() error {
	ow.mutex.Lock()
	defer ow.mutex.Unlock()
	return ow.file.Close()
}
