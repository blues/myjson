// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"os"
	"fmt"
	"sync"
	"mime"
	"time"
	"strings"
	"strconv"
	"io/ioutil"
	"net/http"
	"path/filepath"
)

// Ensure file integrity
var fileLock sync.RWMutex

// Root handler
func inboundWebRootHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

	// Process the request URI, looking for things that will indicate "dev"
	method := httpReq.Method
	if method == "" {
		method = "GET"
	}

	// Try to fix Greg's Linux problem by waiting for full upload
	time.Sleep(1 * time.Second)
	
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
	} else if deleteFilename != ""	{
		httpRsp.Write(deleteFile(target+"/"+deleteFilename))
	} else if method == "GET" && strings.Contains(rawTarget, "/") && !strings.Contains(rawTarget, ":") {
		var ctype string
		c := strings.Split(rawTarget, ".")
		if len(c) > 1 {
			ctype = mime.TypeByExtension("."+c[len(c)-1])
			if ctype != "" {
				httpRsp.Header().Set("Content-Type", ctype)
				httpRsp.WriteHeader(http.StatusOK)
			}
		}
		httpRsp.Write(getFile(rawTarget, ctype))
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
	pathname, bad := cleanFilename(filename)
	if bad {
		return
	}
	fmt.Printf("upload %d bytes to '%s'\n", len(contents), filename)
	c := strings.Split(pathname, "/")
	if len(c) > 1 {
		fileLock.Lock()
		os.MkdirAll(strings.Join(c[0:len(c)-1], "/"), 0777)
		fileLock.Unlock()
	}
	var err error
	fileLock.Lock()
	err = ioutil.WriteFile(pathname, contents, 0644)
	fileLock.Unlock()
	if err != nil {
		fmt.Printf("  err: %s\n", err)
		result = []byte(fmt.Sprintf("%s", err))
	}
	return
}

// Delete a file
func deleteFile(filename string) (contents []byte) {
	pathname, bad := cleanFilename(filename)
	if bad {
		return
	}
	fmt.Printf("FILE DELETE %s\n", filename)
	var err error
	fileLock.Lock()
	err = os.Remove(pathname)
	fileLock.Unlock()
	if err != nil {
		fmt.Printf("  err: %s\n", err)
		contents = []byte(fmt.Sprintf("%s", err))
	}
	return
}

// Get a file
func getFile(filename string, ctype string) (contents []byte) {
	pathname, bad := cleanFilename(filename)
	if bad {
		return
	}
	fmt.Printf("FILE GET %s (%s)\n", filename, ctype)
	var err error
	fileLock.Lock()
	contents, err = ioutil.ReadFile(pathname)
	fileLock.Unlock()
	if err != nil {
		contents = []byte(fmt.Sprintf("%s", err))
	}
	return
}
