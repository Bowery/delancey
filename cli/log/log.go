// Copyright 2013-2014 Bowery, Inc.
// Package log provides routines to log and print debug messages.
package log

import (
	"fmt"
	"io"
	"os"
	"strings"
)

var debug = os.Getenv("DEBUG")

// Debug prints the given arguments if the ENV var is set to development.
func Debug(args ...interface{}) {
	if debug == "cli" {
		Fprint(os.Stderr, "cyan", "DEBUG: ")
		Fprintln(os.Stderr, "", args...)
	}
}

// Print prints arguments with the given attributes, to stdout.
func Print(attrs string, args ...interface{}) {
	Fprint(os.Stdout, attrs, args...)
}

// Fprint prints arguments with the given attributes, to a writer.
func Fprint(w io.Writer, attrs string, args ...interface{}) {
	attrList := strings.Split(attrs, " ")
	for _ = range attrList {
		args = append(args, noAttr)
	}

	fmt.Fprint(w, getColor(attrList[0]))
	if len(attrList) > 1 {
		fmt.Fprint(w, getAttr(attrList[1]))
	}

	fmt.Fprint(w, args...)
}

// Println prints arguments with the given attributes, to stdout.
func Println(attrs string, args ...interface{}) {
	Fprintln(os.Stdout, attrs, args...)
}

// Fprintln prints arguments with the given attributes, to a writer.
func Fprintln(w io.Writer, attrs string, args ...interface{}) {
	attrList := strings.Split(attrs, " ")
	for _ = range attrList {
		args = append(args, noAttr)
	}

	fmt.Fprint(w, getColor(attrList[0]))
	if len(attrList) > 1 {
		fmt.Fprint(w, getAttr(attrList[1]))
	}

	fmt.Fprintln(w, args...)
}
