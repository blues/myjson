// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"net/http"
)

const helpText = "http://myjson.live/<any-string> to watch live JSON objects POSTed to that same URL"

// Help handler
func help(httpRsp http.ResponseWriter) {

	httpRsp.Write([]byte(helpText))

	return

}
