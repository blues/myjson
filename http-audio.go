// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

// Audio handler
func inboundWebAudioHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

	// Get the payload if supplied
	payload, _ := ioutil.ReadAll(httpReq.Body)

	// Extract key parameters
	header := "X-ResponseDevice"
	responseDeviceUID := httpReq.Header.Get(header)
	if responseDeviceUID == "" {
		errmsg := fmt.Sprintf("%s not specified", header)
		fmt.Printf("audio: %s\n", errmsg)
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(errmsg))
		return
	}
	header = "X-ResponseProduct"
	responseProductUID := httpReq.Header.Get(header)
	if responseProductUID == "" {
		errmsg := fmt.Sprintf("%s not specified", header)
		fmt.Printf("audio: %s\n", errmsg)
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(errmsg))
		return
	}
	header = "X-ResponseNotefile"
	responseNotefileID := httpReq.Header.Get(header)
	if responseNotefileID == "" {
		errmsg := fmt.Sprintf("%s not specified", header)
		fmt.Printf("audio: %s\n", errmsg)
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(errmsg))
		return
	}
	header = "X-ResponseToken"
	responseApiToken := httpReq.Header.Get(header)
	if responseApiToken == "" {
		errmsg := fmt.Sprintf("%s not specified", header)
		fmt.Printf("audio: %s\n", errmsg)
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(errmsg))
		return
	}

	// Convert the audio to text
	fmt.Printf("audio: %d bytes to be sent to %s %s %s\n", len(payload), responseDeviceUID, responseProductUID, responseNotefileID)

	// Done
	httpRsp.WriteHeader(http.StatusOK)

}
