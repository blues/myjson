// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"time"
	"sync"
	"github.com/google/uuid"
)

// The active watcher data structure
type activeWatcher struct {
	watcherID		string
	target			string
	event			*Event
	buf				[]byte
}
var watchers = []activeWatcher{}
var watcherLock sync.RWMutex

// Create a new watcher
func watcherCreate(target string) (watcherID string) {

	watcherID = uuid.New().String()

	watcher := activeWatcher{}
	watcher.watcherID = watcherID
	watcher.target = target
	watcher.event = EventNew()
	
	watcherLock.Lock()
	watchers = append(watchers, watcher)
	fmt.Printf("watchers: %s added (now %d)\n", watcher.target, len(watchers))
	watcherLock.Unlock()
	
	return
}

// Delete a watcher
func watcherDelete(watcherID string) {

	watcherLock.Lock()
	numWatchers := len(watchers)
	for i, watcher := range(watchers) {
		if watcher.watcherID == watcherID {
			if i == numWatchers-1 {
				watchers = watchers[0:i]
			} else {
				watchers = append(watchers[0:i], watchers[i+1:]...)
			}
			fmt.Printf("watchers: %s removed (now %d)\n", watcher.target, len(watchers))
			break
		}
	}
	watcherLock.Unlock()

	return

}

// Get data from a watcher
func watcherGet(watcherID string, timeout time.Duration) (data []byte, err error) {
	var watcher activeWatcher

	// Find the watcher
	watcherLock.Lock()
	for _, watcher = range(watchers) {
		if watcher.watcherID == watcherID {
			break
		}
	}
	watcherLock.Unlock()

	// If not found, we're done
	if watcher.watcherID != watcherID {
		err = fmt.Errorf("watcher not found")
		return
	}

	// Wait with timeout
	fmt.Printf("OZZIE %s wait\n", watcherID)
	watcher.event.Wait(timeout)
	fmt.Printf("OZZIE %s back from wait\n", watcherID)

	// Get the buffer
	watcherLock.Lock()
	for i := range(watchers) {
		if watchers[i].watcherID == watcherID {
			data = watchers[i].buf
			watchers[i].buf = []byte{}
			break
		}
	}
	watcherLock.Unlock()

	return

}

// Append data from a watcher
func watcherPut(target string, data []byte) {

	// Scan all watchers
	watcherLock.Lock()
	for i := range(watchers) {
		if watchers[i].target == target {
			watchers[i].buf = append(watchers[i].buf, data...)
			watchers[i].event.Signal()
		}
	}
	watcherLock.Unlock()

	return

}

