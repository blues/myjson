// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Audio handler
func inboundWebAudioHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

	// Handle file download
	if httpReq.Method == http.MethodGet {
		const prefix = "/audio/"
		if !strings.HasPrefix(httpReq.URL.Path, prefix) {
			http.Error(httpRsp, "Invalid URL", http.StatusBadRequest)
			return
		}
		filename := httpReq.URL.Path[len(prefix):]
		fullPath := filepath.Join(configDataDirectory+"audio/", filename)
		file, err := os.Open(fullPath)
		if err != nil {
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
			return
		}
		defer file.Close()
		stat, err := file.Stat()
		if err == nil {
			httpRsp.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
		} else {
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
			return
		}
		httpRsp.Header().Set("Content-Type", "application/octet-stream")
		httpRsp.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

		_, err = io.Copy(httpRsp, file)
		if err != nil {
			httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
			return
		}

		fmt.Printf("https://myjson.live/audio/%s\n", filename)
		httpRsp.WriteHeader(http.StatusOK)
		return
	}

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

	// Remove the 1 byte of padding added by the notecard to "complete" the last page
	if len(payload) != 0 {
		payload = payload[:len(payload)-1]
	}

	// Exit if no audio, which is what's sent to synchronize when we are beginning a new transaction
	if len(payload) == 0 {
		httpRsp.WriteHeader(http.StatusOK)
		return
	}

	// Convert payload to []int16
	pcm := make([]int16, len(payload)/2)
	for i := 0; i < len(pcm); i++ {
		pcm[i] = int16(binary.LittleEndian.Uint16(payload[i*2 : i*2+2]))
	}

	// Convert the audio to text
	fmt.Printf("audio: %d bytes to be sent to %s %s %s:\n", len(payload), responseDeviceUID, responseProductUID, responseNotefileID)

	// Print the pcm slice in an 8-column display.
	// Each value is printed as a right-aligned, fixed-width (10 characters) decimal.
	for i, sample := range pcm {
		fmt.Printf("%10d", sample)
		// After every 8 values, add a new line.
		if (i+1)%8 == 0 {
			fmt.Println()
			if i > 64 {
				break
			}
		}
	}

	// Write the audio to a file
	filename := fmt.Sprintf("%saudio/%d.raw", configDataDirectory, time.Now().UTC().Unix())
	err := os.WriteFile(filename, payload, 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
	} else {
		fmt.Println("File written:", filename)
	}

	// Done
	httpRsp.WriteHeader(http.StatusOK)

}
