// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"sync"
	"github.com/google/uuid"
)

// The active watcher data structure
type activeWatcher struct {
	watcherID		string
	target			string
}
var watchers = []activeWatcher{}
var watcherLock sync.RWMutex

// Create a new watcher
func watcherCreate(target string) (watcherID string) {

	watcherID = uuid.New().String()

	watcher := activeWatcher{}
	watcher.watcherID = watcherID
	watcher.target = target

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

