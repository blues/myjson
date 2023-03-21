// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Serves as a test for Alias routes
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

var lastTime float64
var lastCount int
var maxdiff1, maxdiff2 float64

// Echo handler
func inboundWebEchoHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

	// Get the body if supplied
	reqBody, err := ioutil.ReadAll(httpReq.Body)
	if err != nil {
		reqBody = []byte(fmt.Sprintf("%s", err))
	}
	_ = reqBody

	// If we're echoing URL instead, wrap it
	if httpReq.RequestURI != "/echo" {
		reqBody = []byte(fmt.Sprintf("{\"url\":\"%s\",\"event\":%s}", httpReq.RequestURI, string(reqBody)))
	}

	// Echo
	ct := httpReq.Header.Get("Content-Type")
	if len(reqBody) < 100 && ct == "application/json" {
		extra := ""
		var body map[string]interface{}
		err = json.Unmarshal(reqBody, &body)
		if err == nil {
			v, present := body["time"]
			if present {
				t := v.(float64)
				now := float64(time.Now().UTC().UnixNano()/1000000) / 1000
				if lastTime == 0 {
					lastTime = now
				}
				diff1 := now - t
				diff2 := now - lastTime
				if diff2 > 15 {
					lastTime = now
					lastCount = 0
					maxdiff1 = 0
					maxdiff2 = 0
				}
				if diff1 > maxdiff1 {
					maxdiff1 = diff1
				}
				if diff2 > maxdiff2 {
					maxdiff2 = diff2
				}
				lastTime = now
				extra = fmt.Sprintf(" (client->server %0.3f, since last %0.3f)", diff1, diff2)
				lastCount++
				interval := 1000
				if lastCount >= interval {
					fmt.Printf("\n*** %d MAX client->server %0.3f, MAX since last %0.3f\n\n", interval, maxdiff1, maxdiff2)
					lastCount = 0
					maxdiff1 = 0
					maxdiff2 = 0
				}
			}
		}
		fmt.Printf("ECHO %s%s\n", string(reqBody), extra)
	} else {
		fmt.Printf("ECHO %d bytes of %s\n", len(reqBody), ct)
	}

	// Mirror the content type and the content
	httpRsp.Header().Set("Content-Type", httpReq.Header.Get("Content-Type"))
	httpRsp.WriteHeader(http.StatusOK)
	httpRsp.Write(reqBody)

}
