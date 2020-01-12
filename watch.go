// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"time"
    "net/http"
)

// Watch a target, "live"
func watch(httpRsp http.ResponseWriter, target string) {

	// Generate a unique watcher ID
	watcherID := watcherCreate(target)

	// Data watching loop
	for {

		// Get more data from the watcher, using a timeout computed by trial and
		// error as a reasonable amount of time to catch an error on the Write
		// when the client has gone away.  Longer than that, sometimes the response
		// time in picking up an error becomes quite unpredictable and long.
		data, err := watcherGet(watcherID, 16*time.Second)
		if err != nil {
			break
		}
		if len(data) == 0 {
			data = []byte(time.Now().UTC().Format("2006-01-02T15:04:05Z") + " idle")
		}

		// Write either the accumulated notification text, or the idle message,
		// counting on the fact that one or the other will eventually fail when
		// the HTTP client goes away
		fmt.Printf("OZZIE %s write\n", watcherID)
		_, err = httpRsp.Write(append(data, []byte("\n\n")...))
		if err != nil {
		fmt.Printf("OZZIE %s EXIT from write\n", watcherID)
			break
		}
		fmt.Printf("OZZIE %s back from write\n", watcherID)
		
		// This is an obscure but critical function that flushes partial results
		// back to the client, so that it may display these partial results
		// immediately rather than wait until the end of the transaction.
		f, ok := httpRsp.(http.Flusher)
		if ok {
			f.Flush()
		} else {
			break
		}

	}	

	// Done
	watcherDelete(watcherID)
    return

}
