// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
    "net/http"
)

// Watch a target, "live"
func watch(httpRsp http.ResponseWriter, target string) {

	// Generate a unique watcher ID
	watcherID := watcherCreate(target)

	// Done
	watcherDelete(watcherID)
    return

}
