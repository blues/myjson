// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Serves as a test for Alias routes
package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
)

// API loopback handler
func inboundWebAPIHandler(w http.ResponseWriter, r *http.Request) {

	// Get the body if supplied
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		reqBody = []byte(fmt.Sprintf("%s", err))
	}
	_ = reqBody

	fmt.Printf("sending to %s: %s\n", r.Header.Get("X-Url"), reqBody)

	client := &http.Client{}
	req, _ := http.NewRequest("POST", r.Header.Get("X-Url"), bytes.NewBuffer(reqBody))
	req.Header.Add("Content-Type", r.Header.Get("Content-Type"))
	req.Header.Add("X-Session-Token", r.Header.Get("X-Session-Token"))
	_, err = client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %s", err), http.StatusInternalServerError)
		return
	}

	// Done
	w.WriteHeader(http.StatusOK)
	w.Write(reqBody)

}
