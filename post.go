// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Post to a target
func post(httpRsp http.ResponseWriter, target string, payload []byte) {

	// Ensure that it's JSON, and unmarshal it both to normal and indented forms
	var payloadObject map[string]interface{}
	err := json.Unmarshal(payload, &payloadObject)
	if err != nil {
		http.Error(httpRsp, err.Error(), http.StatusInternalServerError)
		return
	}
	payloadJSON, err := json.Marshal(payloadObject)
	if err != nil {
		http.Error(httpRsp, err.Error(), http.StatusInternalServerError)
		return
	}
	payloadJSON = append(payloadJSON, []byte("\n")...)
	payloadJSONIndented, err := json.MarshalIndent(payloadObject, "", "    ")
	if err != nil {
		http.Error(httpRsp, err.Error(), http.StatusInternalServerError)
		return
	}

	// Show that we're posting
	fmt.Printf("post %s\n", target)

	// Append to the appropriate object
	targetDir := filepath.Join(configDataDirectory, target)
	err = os.MkdirAll(targetDir, 0777)
	if err != nil {
		http.Error(httpRsp, err.Error(), http.StatusInternalServerError)
		return
	}
	targetFile := filepath.Join(targetDir, time.Now().UTC().Format("2006-01-02")+".json")
	f, err := os.OpenFile(targetFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		http.Error(httpRsp, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = f.Write(payloadJSON)
	if err != nil {
		http.Error(httpRsp, err.Error(), http.StatusInternalServerError)
		return
	}
	err = f.Close()
	if err != nil {
		http.Error(httpRsp, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send the intended json to the live monitor, if anyone is watching
	watcherPut(target, payloadJSONIndented)

}
