// Copyright 2013-2014 Bowery, Inc.
// Package prompt provides routines to get, and verify user input
// from stdin.
package prompt

import (
	"Bowery/crosswalk/cli/errors"
	"Bowery/crosswalk/cli/log"
	"io"
	"os"
	"strings"
)

// Basic gets input and if required tests to ensure input was given.
func Basic(prefix string, required bool) (string, error) {
	return Custom(prefix, func(input string) (string, bool) {
		if required && input == "" {
			log.Fprintln(os.Stderr, "red", prefix, errors.ErrEmpty)
			return "", false
		}

		return input, true
	})
}

// BasicDefault gets input and if empty uses the given default.
func BasicDefault(prefix, def string) (string, error) {
	return Custom(prefix+"(Default: "+def+")", func(input string) (string, bool) {
		if input == "" {
			input = def
		}

		return input, true
	})
}

// Ask gets input and checks if it's truthy or not, and returns that
// in a boolean fashion.
func Ask(question string) (bool, error) {
	line, err := Custom(question+"?(y/n)", func(input string) (string, bool) {
		if input == "" {
			log.Fprintln(os.Stderr, "red", "Answer", errors.ErrEmpty)
			return "", false
		}
		input = strings.ToLower(input)

		if input == "y" || input == "yes" {
			return "yes", true
		}

		return "", true
	})

	ok := false
	if line != "" {
		ok = true
	}

	return ok, err
}

// Custom gets input and calls the given test function with the input to
// check if the input is valid, a true return will return the string.
func Custom(prefix string, test func(string) (string, bool)) (string, error) {
	var err error
	line := ""
	ok := false

	term, err := NewTerminal()
	if err != nil {
		return "", err
	}
	defer term.Close()

	for !ok {
		line, err = term.Prompt(prefix + ": ")
		if err != nil && err != io.EOF {
			se := errors.IsStackError(err)

			if se == nil || se.Err != io.EOF {
				return "", err
			}
		}

		line, ok = test(line)
	}

	return line, nil
}

// Password retrieves a password from stdin without echoing it.
func Password(prefix string) (string, error) {
	var err error
	line := ""

	term, err := NewTerminal()
	if err != nil {
		return "", err
	}
	defer term.Close()

	for line == "" {
		line, err = term.Password(prefix + ": ")
		if err != nil && err != io.EOF {
			se := errors.IsStackError(err)

			if se == nil || se.Err != io.EOF {
				return "", err
			}
		}

		if line == "" {
			log.Fprintln(os.Stderr, "red", prefix, errors.ErrEmpty)
		}
	}

	return line, nil
}
