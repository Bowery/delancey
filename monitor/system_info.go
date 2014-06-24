// Copyright 2014 Bowery, Inc.
package main

import (
	sigar "github.com/cloudfoundry/gosigar"
)

// Gets and sets the available and total memory/disk space
// of the host machine. It also pronounces itself as available
// or unavailable for requests.
type SystemInfo struct {
	Address         string `json:"address" bson:"address"`                 // ip of machine
	AvailableMemory uint64 `json:"availableMemory" bson:"availableMemory"` // memory available (in bytes)
	TotalMemory     uint64 `json:"totalMemory" bson:"totalMemory"`         // total memory of machine (in bytes)
	AvailableDisk   uint64 `json:"availableDisk" bson:"availableDisk"`     // disk space aviailable (in bytes)
	TotalDisk       uint64 `json:"totalDisk" bson:"totalDisk"`             // total disk space of machine (in bytes)
	IsAvailable     bool   `json:"isAvailable" bson:"isAvailable"`         // whether the machine is available
}

// Creates a new SystemInfo and updates with the latest
// system stats.
func NewSystemInfo(address string) *SystemInfo {
	s := &SystemInfo{
		Address:     address,
		IsAvailable: true,
	}
	s.UpdateInfo()
	return s
}

// Set total disk space.
func (s *SystemInfo) setTotalDiskSpace() {
	usage := sigar.FileSystemUsage{}
	usage.Get("/")
	s.TotalDisk = usage.Total
}

// Get available disk space.
func (s *SystemInfo) getAvailableDiskSpace() {
	usage := sigar.FileSystemUsage{}
	usage.Get("/")
	s.AvailableDisk = usage.Avail
}

// Set total memory.
func (s *SystemInfo) setTotalMemory() {
	mem := sigar.Mem{}
	mem.Get()
	s.TotalMemory = mem.Total
}

// Get available memory.
func (s *SystemInfo) getAvailableMemory() {
	mem := sigar.Mem{}
	mem.Get()
	s.AvailableMemory = mem.Free
}

// Update all system stats.
func (s *SystemInfo) UpdateInfo() {
	s.setTotalDiskSpace()
	s.getAvailableDiskSpace()
	s.setTotalMemory()
	s.getAvailableMemory()
}
