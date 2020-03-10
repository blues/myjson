// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"os"
	"fmt"
	"sync"
	"mime"
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
	
	// Get the body if supplied
	reqJSON, err := ioutil.ReadAll(httpReq.Body)
	if err != nil {
		reqJSON = []byte{}
	}

	// Get the target
	rawTarget, args := HTTPArgs(httpReq, "")
	if strings.HasSuffix(rawTarget, "/") {
		rawTarget = strings.TrimSuffix(rawTarget, "/")
	}
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
		if len(reqJSON) == 0 {
			httpRsp.Write([]byte("error: zero-length file"))
		} else {
			httpRsp.Write(uploadFile(target+"/"+uploadFilename, reqJSON))
		}
		return
	}
	if deleteFilename != ""	{
		httpRsp.Write(deleteFile(target+"/"+deleteFilename))
		return
	}

	if method == "GET" && strings.Contains(rawTarget, "/") && !strings.Contains(rawTarget, ":") {
		var ctype string
		c := strings.Split(rawTarget, ".")
		if len(c) > 1 {
			ctype = mime.TypeByExtension("."+c[len(c)-1])
			if ctype != "" {
				httpRsp.Header().Set("Content-Type", ctype)
				httpRsp.WriteHeader(http.StatusOK)
			}
		}
		fmt.Printf("xx: %s\n", rawTarget)
		contents, _ := getFile(rawTarget, ctype)
		httpRsp.Write(contents)
		return
	}

	if method == "GET" {
		path := rawTarget + "/index.html"
		ctype := mime.TypeByExtension(".html")
		contents, exists := getFile(path, ctype)
		if exists {
			httpRsp.Header().Set("Content-Type", ctype)
			httpRsp.WriteHeader(http.StatusOK)
			httpRsp.Write(contents)
		}
		return
	}

	if method == "GET" && target == "" {
		help(httpRsp)
		return
	}

	if method == "GET" && count != 0 {
		data := tail(target, count, false, &args)
		httpRsp.Write(data)
		return
	}

	if method == "GET" && clean != 0 {
		data := tail(target, clean, true, nil)
		httpRsp.Write(data)
		return
	}

	if method == "GET" {
		watch(httpRsp, httpReq, target)
		return
	}

	if (method == "POST" || method == "PUT") && len(reqJSON) > 0 {
		post(httpRsp, target, reqJSON)
		return
	}

	httpRsp.Write([]byte(method + " " + target + " ???"))
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
func getFile(filename string, ctype string) (contents []byte, exists bool) {
	pathname, bad := cleanFilename(filename)
	if bad {
		return
	}
	var err error
	fileLock.Lock()
	contents, err = ioutil.ReadFile(pathname)
	fileLock.Unlock()
	if err != nil {
		contents = []byte(fmt.Sprintf("%s", err))
	} else {
		exists = true
		fmt.Printf("FILE GET %s (%s)\n", filename, ctype)
	}
	return
}
