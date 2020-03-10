// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"os"
	"fmt"
	"sync"
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
			ctype = contentType(c[len(c)-1])
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
	fmt.Printf("FILE GET %s %s\n", filename, ctype)
	var err error
	fileLock.Lock()
	contents, err = ioutil.ReadFile(pathname)
	fileLock.Unlock()
	if err != nil {
		contents = []byte(fmt.Sprintf("%s", err))
	}
	return
}

// Get the content type from extension
func contentType(extension string) (ctype string) {
	switch extension {
	case "aac":
		ctype = "audio/aac"
	case "bmp":
		ctype = "image/bmp"
	case "css":
		ctype = "text/css"
	case "csv":
		ctype = "text/csv"
	case "doc":
		ctype = "application/msword"
	case "docx":
		ctype = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "gz":
		ctype = "application/gzip"
	case "gif":
		ctype = "image/gif"
	case "htm":
		ctype = "text/html"
	case "html":
		ctype = "text/html"
	case "ico":
		ctype = "image/vnd.microsoft.icon"
	case "ics":
		ctype = "text/calendar"
	case "jar":
		ctype = "application/java-archive"
	case "jpeg":
		ctype = "image/jpeg"
	case "jpg":
		ctype = "image/jpeg"
	case "js":
		ctype = "text/javascript"
	case "json":
		ctype = "application/json"
	case "jsonld":
		ctype = "application/ld+json"
	case "mid":
		ctype = "audio/midi audio/x-midi"
	case "midi":
		ctype = "audio/midi audio/x-midi"
	case "mjs":
		ctype = "text/javascript"
	case "mp3":
		ctype = "audio/mpeg"
	case "mpeg":
		ctype = "video/mpeg"
	case "opus":
		ctype = "audio/opus"
	case "otf":
		ctype = "font/otf"
	case "png":
		ctype = "image/png"
	case "pdf":
		ctype = "application/pdf"
	case "php":
		ctype = "application/php"
	case "ppt":
		ctype = "application/vnd.ms-powerpoint"
	case "pptx":
		ctype = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case "rar":
		ctype = "application/vnd.rar"
	case "rtf":
		ctype = "application/rtf"
	case "sh":
		ctype = "application/x-sh"
	case "svg":
		ctype = "image/svg+xml"
	case "swf":
		ctype = "application/x-shockwave-flash"
	case "tar":
		ctype = "application/x-tar"
	case "tif":
		ctype = "image/tiff"
	case "tiff":
		ctype = "image/tiff"
	case "ttf":
		ctype = "font/ttf"
	case "txt":
		ctype = "text/plain"
	case "wav":
		ctype = "audio/wav"
	case "weba":
		ctype = "audio/webm"
	case "webm":
		ctype = "video/webm"
	case "webp":
		ctype = "image/webp"
	case "xhtml":
		ctype = "application/xhtml+xml"
	case "xls":
		ctype = "application/vnd.ms-excel"
	case "xlsx":
		ctype = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "xml":
		ctype = "application/xml"
	case "zip":
		ctype = "application/zip"
	case "3gp":
		ctype = "video/3gpp"
	case "7z":
		ctype = "application/x-7z-compressed"
	}
	return
}
