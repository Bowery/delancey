// Copyright 2013-2014 Bowery, Inc.
// Contains system specific routines.
package main

import (
	"fmt"
	"os"
	"strconv"
)

// GetPidTree gets the processes tree.
func GetPidTree(cpid int) (*Proc, error) {
	ppid, err := getPpid(cpid)
	if err != nil {
		return nil, err
	}
	proc := &Proc{Pid: cpid, Ppid: ppid, Children: make([]*Proc, 0)}

	pids, err := pidList()
	if err != nil {
		return nil, err
	}

	for _, pid := range pids {
		if pid == cpid {
			continue
		}

		ppid, err := getPpid(pid)
		if err != nil {
			return nil, err
		}

		if ppid == cpid {
			p, err := GetPidTree(pid)
			if err != nil {
				return nil, err
			}

			proc.Children = append(proc.Children, p)
		}
	}

	return proc, nil
}

// Get the ppid for a pid.
func getPpid(pid int) (int, error) {
	var (
		comm  string
		state byte
		ppid  int
	)

	stat, err := os.Open("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, err
	}
	defer stat.Close()

	_, err = fmt.Fscanf(stat, "%d %s %c %d", &pid, &comm, &state, &ppid)
	if err != nil {
		return 0, err
	}

	return ppid, nil
}

// pidList retrieves all the pids.
func pidList() ([]int, error) {
	procfs, err := os.Open("/proc")
	if err != nil {
		return nil, err
	}

	names, err := procfs.Readdirnames(0)
	if err != nil {
		return nil, err
	}

	pids := make([]int, 0)
	for _, name := range names {
		pid, err := strconv.Atoi(name)
		if err != nil {
			continue
		}

		pids = append(pids, pid)
	}

	return pids, nil
}
