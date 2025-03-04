// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Enumerate a list of the targets that have been used
func tailTargets() (targets []string) {

	// Get a list of all folders in the data directory
	files, err := os.ReadDir(configDataDirectory)
	if err != nil {
		return
	}

	// Append to list of targets if it's a non-special folder
	for _, file := range files {
		if !file.IsDir() {
			continue
		}
		target := file.Name()
		if strings.HasPrefix(target, ".") {
			continue
		}
		targets = append(targets, target)
	}

	return
}

// Do a tail of the posted results, optionally cleaning results prior to that tail
func tail(target string, count int, clean bool, pargs *map[string]string) (data []byte) {

	// Default
	var args map[string]string
	if pargs != nil {
		args = *pargs
	}
	bodyText := args["text"] != ""
	addNewline := args["nl"] != ""

	// Don't allow purge of certain hard-wired targets
	if target == "health" {
		clean = false
	}

	// Bounds check so that we never to doo much work
	if count <= 0 {
		return
	}
	if count > configMaxPosts {
		count = configMaxPosts
	}

	// Get the list of files for the target
	targetDir := filepath.Join(configDataDirectory, target)
	files, err := os.ReadDir(targetDir)
	if err != nil {
		return
	}

	// Show that we're reading this
	if !clean {
		fmt.Printf("tail %s %d\n", target, count)
	}

	// Append to the list of files
	var filenames []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Only purge files that are of the form yyyy-mm-dd.json
		name := file.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		if len(strings.Split(name, "-")) != 3 {
			continue
		}

		// Append the name to the list of files to be deleted
		filenames = append(filenames, name)

	}
	if len(filenames) == 0 {
		return
	}

	// Sort the filenames
	sort.Strings(filenames)

	// Start gathering results in most-recent order
	numFilenames := len(filenames)
	appended := 0
	done := false
	for i := numFilenames - 1; i >= 0; i = i - 1 {
		filename := filepath.Join(targetDir, filenames[i])

		// If we're cleaning and  we're done, delete the file
		if clean && done {
			fmt.Printf("purging %s %s\n", target, filenames[i])
			os.Remove(filename)
			continue
		}

		// Open the file
		contents, err := os.ReadFile(filename)
		if err != nil {
			fmt.Printf("can't read %s: %s\n", filenames[i], err)
			continue
		}

		// Split the file into the json parts
		arrayJSON := bytes.Split(contents, []byte{'\n'})
		arrayLen := len(arrayJSON)
		if arrayLen == 0 {
			continue
		}

		// Append to the data, noting if we're done
		for j := arrayLen - 1; j >= 0 && !done; j = j - 1 {
			if len(arrayJSON[j]) > 0 {
				thisdata := arrayJSON[j]
				// Do special processing of data if requested
				if bodyText {
					thisdata = extractBodyText(thisdata)
				}
				// Place data at the beginning
				if len(data) == 0 {
					data = thisdata
				} else {
					thisdata = append(thisdata, []byte("\n")...)
					if addNewline {
						thisdata = append(thisdata, []byte("\n")...)
					}
					data = append(thisdata, data...)
				}
				// Next
				appended = appended + 1
				if appended >= count {
					done = true
				}
			}
		}

	}

	// Done
	return

}

// Extract just the "text" item of the body field, with some extra
func extractBodyText(in []byte) (out []byte) {

	// Default output to input for error returns
	out = in

	// Unmarshal to an object
	var jobj map[string]interface{}
	err := json.Unmarshal(in, &jobj)
	if err != nil {
		return
	}

	// Extract just the field we're interested in
	var ok bool
	var body map[string]interface{}
	var bodyText string
	body, ok = jobj["body"].(map[string]interface{})
	if ok {
		bodyText, _ = body["text"].(string)
	}
	var project map[string]interface{}
	var projectName string
	project, ok = jobj["project"].(map[string]interface{})
	if ok {
		projectName, _ = project["name"].(string)
	}
	var sn string
	sn, _ = jobj["sn"].(string)
	var deviceUID string
	deviceUID, _ = jobj["device"].(string)
	if sn == "" {
		sn = deviceUID
	}
	var routed float64
	routed, _ = jobj["routed"].(float64)
	routedDate := time.Unix(int64(routed), 0).Format("01/02")
	routedTime := time.Unix(int64(routed), 0).Format("15:04")
	todayDate := time.Now().UTC().Format("01/02")
	if routedDate == todayDate {
		routedDate += "*"
	}

	// Create output line
	out = []byte(routedDate + " " + routedTime + " " + bodyText + " (" + projectName + " / " + sn + ")")
	return

}
