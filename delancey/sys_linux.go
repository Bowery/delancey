// Copyright 2013-2014 Bowery, Inc.
// Contains system specific routines.
package main

import (
	"os"
	"strconv"
	"syscall"
)

// FindPidsByPgid gets a list of pids that have a pgid that matches the given
// pgid. It excludes pids that match the pgid.
func FindPidsByPgid(pgid int) ([]int, error) {
	pids := make([]int, 0)
	proc, err := os.Open("/proc")
	if err != nil {
		return pids, err
	}
	defer proc.Close()

	names, err := proc.Readdirnames(0)
	if err != nil {
		return pids, err
	}

	for _, name := range names {
		pid, err := strconv.Atoi(name)
		if err != nil || pid == pgid {
			continue
		}

		ppgid, err := syscall.Getpgid(pid)
		if err != nil {
			return nil, err
		}

		if ppgid == pgid {
			pids = append(pids, pid)
		}
	}

	return pids, nil
}
