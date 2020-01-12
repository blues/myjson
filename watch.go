// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"time"
    "net/http"
)

// Watch a target, "live"
func watch(httpRsp http.ResponseWriter, target string) {

	httpRsp.Write([]byte(time.Now().UTC().Format("2006-01-02T15:04:05Z") + "show " + target))
	
    return

}