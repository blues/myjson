// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Serves Health Checks
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// Ping handler
func inboundWebSendHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

	// Get the body if supplied
	reqJSON, err := ioutil.ReadAll(httpReq.Body)
	if err != nil {
		reqJSON = []byte("{}")
	}
	_ = reqJSON

	// Debug
	fmt.Printf("*****\n")
	fmt.Printf("%s\n", reqJSON)
	fmt.Printf("*****\n")

	return

}
