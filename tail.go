// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"os"
	"fmt"
	"sort"
	"bytes"
	"strings"
	"io/ioutil"
	"path/filepath"
)

// Enumerate a list of the targets that have been used
func tailTargets() (targets []string) {

	// Get a list of all folders in the data directory
	files, err := ioutil.ReadDir(configDataDirectory)
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
func tail(target string, count int, clean bool) (data []byte) {

	// Don't allow purge of certain hard-wired targets
	if target == "health" {
		clean = false;
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
	files, err := ioutil.ReadDir(targetDir)
	if err != nil {
		return
	}

	// Append to the list of files
	var filenames []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filenames = append(filenames, file.Name())
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
	for i:=numFilenames-1; i>=0; i=i-1 {
		filename := filepath.Join(targetDir, filenames[i])

		// If we're cleaning and  we're done, delete the file
		if clean && done {
			fmt.Printf("purging %s %s\n", target, filenames[i])
			os.Remove(filename)
			continue
		}

		// Open the file
		fmt.Printf("OZZIE %s\n", filename)
		contents, err := ioutil.ReadFile(filename)
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
		for j:=arrayLen-1; j>=0 && !done; j=j-1 {
			if (len(arrayJSON[j]) > 0) {
				thisdata := arrayJSON[j]
				if len(data) == 0 {
					data = thisdata
				} else {
					thisdata = append(thisdata, []byte("\n")...)
					data = append(thisdata, data...)
				}
				appended = appended+1
				if appended >= count {
					done = true
				}
			}
		}

	}

	// Done
	return

}
