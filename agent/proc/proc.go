// Copyright 2014 Bowery, Inc.
package proc

func Restart() chan bool {
	started := make(chan bool, 1)
	started <- true
	return started
}
