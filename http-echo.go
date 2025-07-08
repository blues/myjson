// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Serves as a test for Alias routes
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var lastTime float64
var lastCount int
var maxdiff1, maxdiff2 float64
var reset = true

// Echo handler
func inboundWebEchoHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

	// See if a milliseconds delay is specified
	const prefix = "/echo/"
	if strings.HasPrefix(httpReq.URL.Path, prefix) {
		msStr := strings.TrimPrefix(httpReq.URL.Path, prefix)
		ms, err := strconv.Atoi(msStr)
		if err != nil {
			http.Error(httpRsp, fmt.Sprintf("%s", err), http.StatusBadRequest)
			return
		} else if ms <= 0 {
			http.Error(httpRsp, "milliseconds must be positive", http.StatusBadRequest)
			return
		}
		fmt.Printf("ECHO: delay %d ms\n", ms)
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}

	// Get the body if supplied
	reqBody, err := io.ReadAll(httpReq.Body)
	if err != nil {
		reqBody = []byte(fmt.Sprintf("%s", err))
	}
	_ = reqBody

	// If we're echoing URL instead, wrap it
	if httpReq.RequestURI != "/echo" {
		reqBody = []byte(fmt.Sprintf("{\"url\":\"%s\",\"event\":%s}", httpReq.RequestURI, string(reqBody)))
	}

	// Extract the "seconds" query parameter
	secondsStr := httpReq.URL.Query().Get("seconds")
	seconds := 0
	if secondsStr != "" {
		seconds, _ = strconv.Atoi(secondsStr)
	}

	// Enforce a delay if requested
	if seconds != 0 {
		time.Sleep(time.Duration(seconds) * time.Second)
	}

	// Echo
	ct := httpReq.Header.Get("Content-Type")
	if len(reqBody) < 500 && ct == "application/json" {
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
				if diff2 > 10 || reset {
					reset = false
					lastTime = now
					diff2 = 0
					lastCount = 0
					maxdiff1 = 0
					maxdiff2 = 0
				}
				if diff1 > maxdiff1 && lastCount != 0 {
					maxdiff1 = diff1
				}
				if diff2 > maxdiff2 && lastCount != 0 {
					maxdiff2 = diff2
				}
				lastTime = now
				interval := 250
				extra2 := ""
				if lastCount != 0 && diff1 >= 1 || diff2 >= 1 {
					extra2 = " <<<<<<<<<<<<<<<<<<"
				}
				extra = fmt.Sprintf(" (%d client->server %0.3f, since last %0.3f)%s", interval-lastCount, diff1, diff2, extra2)
				lastCount++
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
		reset = true
		fmt.Printf("ECHO %d bytes of %s\n", len(reqBody), ct)
	}

	// Mirror the content type and the content
	httpRsp.Header().Set("Content-Type", httpReq.Header.Get("Content-Type"))
	httpRsp.WriteHeader(http.StatusOK)
	httpRsp.Write(reqBody)

}
