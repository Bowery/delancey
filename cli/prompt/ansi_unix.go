// +build linux darwin

// Copyright 2013-2014 Bowery, Inc.
package prompt

import (
	"os"
)

// AnsiReader is an io.Reader that wraps a file.
type AnsiReader struct {
	file *os.File
}

// NewAnsiReader creates a AnsiReader from the given input.
func NewAnsiReader(in *os.File) *AnsiReader {
	return &AnsiReader{file: in}
}

// Read just wraps the files read.
func (ar *AnsiReader) Read(b []byte) (int, error) {
	return ar.file.Read(b)
}

// AnsiWriter is an io.Writer that wraps a file.
type AnsiWriter struct {
	file *os.File
}

// NewAnsiWriter creates a AnsiWriter from the given output.
func NewAnsiWriter(out *os.File) *AnsiWriter {
	return &AnsiWriter{file: out}
}

// Write just wraps the files write.
func (aw *AnsiWriter) Write(b []byte) (int, error) {
	return aw.file.Write(b)
}
