// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Serves as a test for Alias routes
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// Echo handler
func inboundWebEchoHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

	// Get the body if supplied
	reqBody, err := ioutil.ReadAll(httpReq.Body)
	if err != nil {
		reqBody = []byte(fmt.Sprintf("%s", err))
	}
	_ = reqBody

	// Mirror the content type and the content
	httpRsp.Header().Set("Content-Type", httpReq.Header.Get("Content-Type"))
	httpRsp.WriteHeader(http.StatusOK)
	httpRsp.Write(reqBody)

	return

}
