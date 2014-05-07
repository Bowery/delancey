// +build linux darwin

// Copyright 2013-2014 Bowery, Inc.
package prompt

var (
	mvLeftEdge = []byte("\u001b[0G")
	clsScreen  = []byte("\u001b[H\u001b[2J")
	delRight   = []byte("\u001b[0K")
	mvToCol    = "\u001b[0G\u001b[%dC"
)
