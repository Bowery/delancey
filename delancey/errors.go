// Copyright 2013-2014 Bowery, Inc.
// Contains common errors.
package main

import (
	"errors"
)

// Standard errors that may occur.
var (
	ErrAppDevEnv     = errors.New("APPLICATION or DEVELOPER environment variable missing.")
	ErrHostEnv       = errors.New("HOST environment variable is required in development.")
	ErrMissingFields = errors.New("Missing form fields.")
)
