// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
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
	rawTarget, args := HTTPArgs(httpReq, "")
	target := cleanTarget(rawTarget)

	// Exit if just the favicon
	if rawTarget == "favicon.ico" {
		return
	}

	// Process args
	count, _ := strconv.Atoi(args["count"])
	if count == 0 {
		count, _ = strconv.Atoi(args["tail"])
	}
	clean, _ := strconv.Atoi(args["clean"])
	uploadFilename := args["upload"]
	deleteFilename := args["delete"]

	// Process appropriately
	if (method == "POST" || method == "PUT") && uploadFilename != "" {
		uploadFile(target+"/"+uploadFilename, reqJSON)
	} else if deleteFilename != ""  {
		deleteFile(target+"/"+deleteFilename)
	} else if method == "GET" && strings.Contains(rawTarget, "/") {
		httpRsp.Write(getFile(rawTarget))
	} else if method == "GET" && target == "" {
		help(httpRsp)
	} else if method == "GET" && count != 0 {
		data := tail(target, count, false, &args)
	    httpRsp.Write(data)
	} else if method == "GET" && clean != 0 {
		data := tail(target, clean, true, nil)
	    httpRsp.Write(data)
	} else if method == "GET" {
		watch(httpRsp, httpReq, target)
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

// Upload a file
func uploadFile(filename string, contents []byte) {
	fmt.Printf("upload to '%s': %s\n", filename, contents)
}

// Delete a file
func deleteFile(filename string) {
	fmt.Printf("delete '%s'\n", filename)
}

// Get a file
func getFile(filename string) (contents []byte) {
	fmt.Printf("get '%s'\n", filename)
	contents = []byte("hi there")
	return
}
