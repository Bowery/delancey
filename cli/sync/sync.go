// Copyright 2013-2014 Bowery, Inc.
// Package sync implements routines to do file updates to services
// satellite instances.
package sync

import (
	"Bowery/crosswalk/cli/errors"
	"Bowery/crosswalk/cli/log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/howeyc/fsnotify"
)

// Event describes a file event and the associated service name.
type Event struct {
	Service string
	Status  string
	Path    string
}

func (ev *Event) String() string {
	return strings.Title(ev.Status) + "d " + ev.Path
}

// Watcher contains an fs watcher and handles the syncing to a service.
type Watcher struct {
	Path    string
	Watcher *fsnotify.Watcher
	stats   map[string]os.FileInfo
	mutex   sync.RWMutex
}

// NewWatcher creates a watcher.
func NewWatcher(path string) (*Watcher, error) {
	watcher := &Watcher{
		Path: path,
	}

	fswatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.NewStackError(watcher.wrapErr(err))
	}

	watcher.Watcher = fswatcher
	watcher.stats = make(map[string]os.FileInfo)
	return watcher, nil
}

// Start adds the watchers paths directories to the watcher.
func (watcher *Watcher) Start() error {
	dirs, err := getDirs(watcher.Path)
	if err != nil {
		return watcher.wrapErr(err)
	}

	for _, dir := range dirs {
		err = watcher.Watcher.Watch(dir)
		if err != nil {
			return errors.NewStackError(watcher.wrapErr(err))
		}
	}

	return nil
}

// Watch handles file events and uploads the changes.
func (watcher *Watcher) Watch(evChan chan *Event, errChan chan error) {
	fswatcher := watcher.Watcher

	for {
		select {
		case ev := <-fswatcher.Event:
			if ev == nil {
				break
			}

			rel, err := filepath.Rel(watcher.Path, ev.Name)
			if err != nil {
				errChan <- watcher.wrapErr(err)
				return
			}
			if len(rel) <= 0 {
				break
			}
			name := filepath.Base(rel)

			// If it's a tmp vim 4913 file ignore it.
			vim, _ := strconv.Atoi(name)
			if vim != 0 && vim >= 4913 && (vim == 4913 || (vim-4913)%123 == 0) {
				log.Debug("Ignoring vim tmp file", rel)
				break
			}

			status := "update"
			if ev.IsCreate() {
				status = "create"
			} else if ev.IsDelete() {
				status = "delete"
			} else if ev.IsRename() {
				// Ignore renames, since that triggers a create.
				log.Debug("Ignoring rename event", rel)
				break
			}

			// Get stat for the path if possible. Ignore error and just use stat
			// if exists.
			stat, _ := os.Lstat(ev.Name)

			// Ignore event if deleting but stat is successful, this happens
			// in Darwin.
			if stat != nil && status == "delete" {
				log.Debug("Ignoring false delete event", rel)
				break
			}

			// Ignore event if not deleting but a failed stat, this can occur
			// if a delete occurs immediately after.
			if stat == nil && status != "delete" {
				log.Debug("Ignoring false", status, "event", rel)
				break
			}

			if status == "create" && stat.IsDir() {
				err = fswatcher.Watch(ev.Name)
				if err != nil {
					errChan <- errors.NewStackError(watcher.wrapErr(err))
					return
				}
			}

			if status != "delete" && stat.IsDir() {
				log.Debug("Ignore directory", status, "event", rel)
				break
			}

			// Check the paths mtime to ensure changes occured.
			if stat != nil {
				var pmtime time.Time

				watcher.mutex.RLock()
				pstat, ok := watcher.stats[ev.Name]
				if ok {
					pmtime = pstat.ModTime()
				}
				watcher.mutex.RUnlock()

				cmtime := stat.ModTime()
				if ok && pmtime.Equal(cmtime) {
					log.Debug("Ignoring", status, "event because no change", rel)
					break
				}

				watcher.mutex.Lock()
				watcher.stats[ev.Name] = stat
				watcher.mutex.Unlock()
			}

			err = watcher.Update(rel, status)
			if err != nil {
				errChan <- err
				return
			}

			// Saves may be registered as create when it's a update in reality,
			// change it for the event so there's no possible confusion from users.
			if status == "create" {
				status = "update"
			}

			evChan <- &Event{"service", status, rel}
		case err := <-fswatcher.Error:
			if err == nil {
				break
			}

			errChan <- errors.NewStackError(watcher.wrapErr(err))
			return
		}
	}
}

// Upload sends the paths contents to the service compressed.
func (watcher *Watcher) Upload() error {
	return nil
}

// Update updates a path to the service.
func (watcher *Watcher) Update(name, status string) error {
	return nil
}

// Close closes the watcher and removes existing upload files.
func (watcher *Watcher) Close() error {
	err := watcher.Watcher.Close()
	if err != nil {
		return errors.NewStackError(watcher.wrapErr(err))
	}

	return nil
}

// wrapErr wraps an error with the given service name.
func (watcher *Watcher) wrapErr(err error) error {
	if err == nil {
		return nil
	}

	se := errors.IsStackError(err)
	if se != nil {
		se.Err = errors.Newf(errors.ErrSyncTmpl, "service", se.Err)
		return se
	}

	return errors.Newf(errors.ErrSyncTmpl, "service", err)
}

// Syncer syncs file changes to a list of given service satellite instances.
type Syncer struct {
	Event    chan *Event
	Upload   chan *string
	Error    chan error
	Watchers []*Watcher
}

// NewSyncer creates a syncer.
func NewSyncer() *Syncer {
	return &Syncer{
		Event:    make(chan *Event),
		Upload:   make(chan *string),
		Error:    make(chan error),
		Watchers: make([]*Watcher, 0),
	}
}

// Watch starts watching the given path and updates changes to the service.
func (syncer *Syncer) Watch(path string) error {
	watcher, err := NewWatcher(path)
	if err != nil {
		return err
	}
	syncer.Watchers = append(syncer.Watchers, watcher)

	// Do the actual event management, and the inital upload.
	go func() {
		watcher.Watch(syncer.Event, syncer.Error)
	}()

	return watcher.Start()
}

// Close closes all the watchers.
func (syncer *Syncer) Close() error {
	for _, watcher := range syncer.Watchers {
		err := watcher.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// getDirs retrieves the directories in the given path excluding directories
// that should be ignored.
func getDirs(path string) ([]string, error) {
	dirs := make([]string, 0)

	// Add paths that are directories to the dirs list, excluding hidden files.
	walker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			dirs = append(dirs, path)
		}
		return nil
	}

	err := filepath.Walk(path, walker)
	if err != nil {
		return nil, err
	}

	return dirs, nil
}
