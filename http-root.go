// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"time"
	"io/ioutil"
    "net/http"
)

// Root handler
func inboundWebRootHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

    // Process the request URI, looking for things that will indicate "dev"
	method := httpReq.Method
    if method == "" {
        method = "GET"
    }
	
    // Get the body if supplied
    reqJSON, err := ioutil.ReadAll(httpReq.Body)
    if err != nil {
		reqJSON = []byte("{}")
    }
	_ = reqJSON

	// Write reply JSON
	target, _ := HTTPArgs(httpReq, "")
	rspJSON := []byte(method+"("+target+")"+time.Now().UTC().Format("2006-01-02T15:04:05Z"))
    httpRsp.Write(rspJSON)

    return

}
