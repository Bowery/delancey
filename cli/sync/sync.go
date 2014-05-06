// Copyright 2014 Bowery, Inc.
package sync

import (
	"os"
	"path/filepath"
	"sync"
)

type Event struct {
	Service string
	Status  string
	Path    string
}

type Watcher struct {
	Path    string
	Watcher *fsnotify.Watcher
	stats   map[string]os.FileInfo
	mutex   sync.RWMutex
}

func NewWatcher(path string) (*Watcher, error) {
	watcher := &Watcher{
		Path: path,
	}

	fswatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	watcher.Watcher = fswatcher
	watcher.stats = make(map[string]os.FileInfo)
	return watcher, nil
}

func (w *Watcher) Start() error {
	dirs, err := getDirs(w.Path)
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		if err = w.Watcher.Watch(dir); err != nil {
			return err
		}
	}

	return nil
}

func (w *Watcher) Watch(evChan chan *Event, errChan chan error) {
	fswatcher := watcher.Watcher

	for {
		select {
		case ev := <-fswatcher.Event:
			if ev == nil {
				break
			}

			rel, err := filepath.Rel(watcher.Path, ev.Name)
			if err != nil {
				errChan <- err
			}
			if len(rel) <= 0 {
				break
			}
			name := filepath.Base(rel)
		}
	}
}

func getDirs(path string) ([]string, error) {
	dirs := make([]string, 0)
	walker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			dirs = append(dirs, path)
		}

		return nil
	}

	if err := filepath.Walk(path, walker); err != nil {
		return nil, err
	}

	return dirs, nil
}
