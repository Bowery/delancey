// +build !windows

// Copyright 2013-2014 Bowery, Inc.
package log

const noAttr = "\u001b[0m"

// getColor retrieves the ASCII code for the color.
func getColor(color string) string {
	switch color {
	case "black":
		return "\u001b[30m"
	case "grey":
		return "\u001b[90m"
	case "red":
		return "\u001b[31m"
	case "green":
		return "\u001b[32m"
	case "yellow":
		return "\u001b[33m"
	case "blue":
		return "\u001b[34m"
	case "magenta":
		return "\u001b[35m"
	case "cyan":
		return "\u001b[36m"
	case "white":
		return "\u001b[37m"
	}

	return ""
}

// getAttr retrieves the ASCII code for text attributes(italics, bold, etc.).
func getAttr(attr string) string {
	switch attr {
	case "bold":
		return "\u001b[1m"
	case "underline":
		return "\u001b[4m"
	}

	return ""
}
