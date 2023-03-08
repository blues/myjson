// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"
	"time"
)

// Watch a target, "live"
func watch(httpRsp http.ResponseWriter, httpReq *http.Request, target string) {

	fmt.Printf("watch %s\n", target)

	// Browser clients buffer output before display UNLESS this is the content type
	httpRsp.Header().Set("Content-Type", "application/json")

	// Begin
	data := []byte(time.Now().UTC().Format("2006-01-02T15:04:05Z") + " watching " + target + "\n")
	httpRsp.Write(data)

	// Generate a unique watcher ID
	watcherID := watcherCreate(target)

	// Data watching loop
	for {

		// This is an obscure but critical function that flushes partial results
		// back to the client, so that it may display these partial results
		// immediately rather than wait until the end of the transaction.
		f, ok := httpRsp.(http.Flusher)
		if ok {
			f.Flush()
		} else {
			break
		}

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
		data = append(data, []byte("\n")...)
		_, err = httpRsp.Write(data)
		if err != nil {
			break
		}

	}

	// Done
	watcherDelete(watcherID)
	return

}
