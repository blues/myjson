// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"time"
	"strings"
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
		reqJSON = []byte{}
    }
	_ = reqJSON

	// Get the target
	target, _ := HTTPArgs(httpReq, "")
	target = cleanTarget(target)

	// Process appropriately
	fmt.Printf("%s %s %s\n", time.Now().UTC().Format("2006-01-02T15:04:05Z"), method, target)
	if method == "GET" && target == "" {
		help(httpRsp)
	} else if method == "GET" {
		watch(httpRsp, target)
	} else if (method == "POST" || method == "PUT") && len(reqJSON) > 0 {
		post(httpRsp, target, reqJSON)
	} else {
	    httpRsp.Write([]byte(method + " " + target + " ???"))
	}
	
    return

}

// Clean a target so that it contains only the chars legal in a filename
func cleanTarget(in string) (out string) {
	for _, r := range strings.ToLower(in) {
		c := string(r)
		if (c >= "a" && c <= "z") || (c >= "0" && c <= "9") || (c == "_" || c == "-") {
			out = out + c
		} else {
			out = out + "-"
		}
	}
	return
}
