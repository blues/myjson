// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	reqJSON, err := io.ReadAll(httpReq.Body)
	if err != nil {
		reqJSON = []byte{}
	}

	// Get the target
	rawTarget, args := HTTPArgs(httpReq, "")
	rawTarget = strings.TrimSuffix(rawTarget, "/")
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
	append := false
	uploadFilename := args["append"]
	if uploadFilename != "" {
		append = true
	} else {
		uploadFilename = args["upload"]
	}
	deleteFilename := args["delete"]

	// Map the delete verb
	if method == "DELETE" && strings.Contains(rawTarget, "/") && !strings.Contains(rawTarget, ":") {
		httpRsp.Write(deleteFile(rawTarget))
		return
	}

	// Process appropriately
	if (method == "POST" || method == "PUT") && uploadFilename != "" {
		if len(reqJSON) == 0 {
			httpRsp.Write([]byte("error: zero-length file"))
		} else {
			httpRsp.Write(uploadFile(target+"/"+uploadFilename, append, reqJSON))
		}
		return
	}
	if deleteFilename != "" {
		httpRsp.Write(deleteFile(target + "/" + deleteFilename))
		return
	}

	if method == "GET" && strings.Contains(rawTarget, "/") && !strings.Contains(rawTarget, ":") {
		var ctype string
		c := strings.Split(rawTarget, ".")
		if len(c) > 1 {
			ctype = mime.TypeByExtension("." + c[len(c)-1])
			if ctype != "" {
				httpRsp.Header().Set("Content-Type", ctype)
				httpRsp.WriteHeader(http.StatusOK)
			}
		}
		contents, _ := getFile(rawTarget, ctype)
		httpRsp.Write(contents)
		return
	}

	if method == "GET" {
		path := rawTarget + "/index.html"
		ctype := mime.TypeByExtension(".html")
		_, exists := getFile(path, ctype)
		if exists {
			fmt.Printf("redirect to %s\n", path)
			http.Redirect(httpRsp, httpReq, path, http.StatusTemporaryRedirect)
			return
		}
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
func uploadFile(filename string, append bool, contents []byte) (result []byte) {

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

	flags := os.O_CREATE | os.O_WRONLY
	if append {
		flags = flags | os.O_APPEND
	}
	f, err := os.OpenFile(pathname, flags, 0644)
	if err == nil {
		_, err = f.Write(contents)
		f.Close()
	}

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
	contents, err = os.ReadFile(pathname)
	fileLock.Unlock()
	if err != nil {
		contents = []byte(fmt.Sprintf("%s", err))
	} else {
		exists = true
		fmt.Printf("FILE GET %s (%s)\n", filename, ctype)
	}
	return
}
