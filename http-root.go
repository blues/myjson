// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"os"
	"fmt"
	"strings"
	"strconv"
	"io/ioutil"
    "net/http"
	"path/filepath"
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
		httpRsp.Write(uploadFile(target+"/"+uploadFilename, reqJSON))
	} else if deleteFilename != ""  {
		httpRsp.Write(deleteFile(target+"/"+deleteFilename))
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

// Clean a filename
func cleanFilename(in string) (out string, bad bool) {
	if strings.Contains(in, "..") {
		return "", true
	}
	if strings.Contains(in, "./") {
		return "", true
	}
	if strings.HasPrefix(in, "/") {
		return "", true
	}
	out = filepath.Join(configDataDirectory, in)
	return
}

// Upload a file
func uploadFile(filename string, contents []byte) (result []byte) {
	filename, bad := cleanFilename(filename)
	if bad {
		return
	}
	fmt.Printf("upload to '%s': %s\n", filename, contents)
	c := strings.Split(filename, "/")
	if len(c) > 1 {
		os.MkdirAll(strings.Join(c, "/"), 0777)
	}
	var err error
	err = ioutil.WriteFile(filename, contents, 0644)
	if err != nil {
		fmt.Printf("  err: %s\n", err)
		result = []byte(fmt.Sprintf("%s", err))
	}
	return
}

// Delete a file
func deleteFile(filename string) (contents []byte) {
	filename, bad := cleanFilename(filename)
	if bad {
		return
	}
	fmt.Printf("FILE DELETE %s\n", filename)
	var err error
    err = os.Remove(filename)
	if err != nil {
		fmt.Printf("  err: %s\n", err)
		contents = []byte(fmt.Sprintf("%s", err))
	}
	return
}

// Get a file
func getFile(filename string) (contents []byte) {
	filename, bad := cleanFilename(filename)
	if bad {
		return
	}
	fmt.Printf("FILE GET %s\n", filename)
	var err error
    contents, err = ioutil.ReadFile(filename)
	if err != nil {
		contents = []byte(fmt.Sprintf("%s", err))
	}
	return
}
