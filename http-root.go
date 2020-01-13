// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"strings"
	"strconv"
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
	target, args := HTTPArgs(httpReq, "")
	target = cleanTarget(target)

	// Process args
	count, _ := strconv.Atoi(args["count"])
	if count == 0 {
		count, _ = strconv.Atoi(args["tail"])
	}
	clean, _ := strconv.Atoi(args["clean"])

	// Process appropriately
	if method == "GET" && target == "" {
		help(httpRsp)
	} else if method == "GET" && count != 0 {
		data := tail(target, count, false, &args)
	    httpRsp.Write(data)
	} else if method == "GET" && clean != 0 {
		data := tail(target, clean, true, nil)
	    httpRsp.Write(data)
	} else if method == "GET" {
		// Debug that outputs text to unstick a stuck browser
		emit, _ := strconv.Atoi(args["emit"])
		if emit != 0 {
			httpRsp.Write([]byte(strings.Repeat(" ", emit)+"\n"))
			f, ok := httpRsp.(http.Flusher)
			if ok {
				f.Flush()
			}
		}
		// Watch
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
