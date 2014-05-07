// Copyright 2013, 2014 Bowery, Inc.
package log

const noAttr = ""

// getColor is a no-op on Windows.
func getColor(color string) string {
	return noAttr
}

// getAttr is a no-op on Windows.
func getAttr(attr string) string {
	return noAttr
}
