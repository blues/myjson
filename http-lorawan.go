// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Serves Health Checks
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/blues/note-go/notehub"
)

// Known header formats
const hdrFormatHelium = "helium"
const hdrFormatTTN = "ttn"

// LoRaWAN handler
func inboundWebLoRaWANHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

	// Only accept POST
	method := httpReq.Method
	if method != "POST" {
		httpRsp.WriteHeader(http.StatusMethodNotAllowed)
	}

	// Extract headers
	hdrAPIKey := httpReq.Header.Get("X-Session-Token")
	hdrFormat := httpReq.Header.Get("X-Format")
	hdrHub := httpReq.Header.Get("X-Hub")
	hdrProject := httpReq.Header.Get("X-Project")
	hdrFile := httpReq.Header.Get("X-File")
	hdrTemplate := httpReq.Header.Get("X-Template")

	// Validate formats
	if hdrFormat != hdrFormatHelium && hdrFormat != hdrFormatTTN {
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte("X-Format must uniquely identify the JSON schema of inbound LoRaWAN messages\r\n"))
		return
	}
	if hdrHub == "" {
		hdrHub = notehubURL
	}
	if !strings.HasPrefix(hdrProject, "app:") {
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte("X-App must be Notehub Project UID\r\n"))
		return
	}
	if hdrFile == "" {
		hdrFile = "data.qo"
	}
	if hdrTemplate == "" {
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte("X-Template must be the JSON Notefile Template describing the LoRaWAN payload\r\n"))
		return
	}
	payloadTemplate := map[string]interface{}{}
	err := json.Unmarshal([]byte(hdrTemplate), &payloadTemplate)
	if err != nil {
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(fmt.Sprintf("X-Template doesn't appear to be valid JSON: %s\r\n", err)))
		return
	}

	// Extract the essential fields from the request
	deviceUID := ""
	payload := []byte{}
	uplinkMessageJSON, err := ioutil.ReadAll(httpReq.Body)
	if err != nil {
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(fmt.Sprintf("can't read uplink message: %s\r\n", err)))
		return
	}
	if hdrFormat == hdrFormatHelium {
		msg := heliumUplinkMessage{}
		err = json.Unmarshal(uplinkMessageJSON, &msg)
		if err != nil {
			httpRsp.WriteHeader(http.StatusBadRequest)
			httpRsp.Write([]byte(fmt.Sprintf("can't decode Helium uplink message: %s\r\n", err)))
			return
		}
		payload = msg.Payload
		deviceUID = "dev:" + strings.ToLower(msg.DeviceEUI)
	}
	if hdrFormat == hdrFormatTTN {
		msg := ttnUplinkMessage{}
		err = json.Unmarshal(uplinkMessageJSON, &msg)
		if err != nil {
			httpRsp.WriteHeader(http.StatusBadRequest)
			httpRsp.Write([]byte(fmt.Sprintf("can't decode TTN uplink message: %s\r\n", err)))
			return
		}
		payload = msg.PayloadRaw
		deviceUID = "dev:" + strings.ToLower(msg.DevID)
	}

	// Convert payload to body
	body := map[string]interface{}{}
	body["payload"] = fmt.Sprintf("%x", payload)

	// Create the 'add note' request
	hubreq := notehub.HubRequest{}
	hubreq.Req = "note.add"
	hubreq.Body = &body
	hubreq.NotefileID = hdrFile
	hubreq.AppUID = hdrProject
	hubreq.DeviceUID = deviceUID
	hubreqJSON, err := json.Marshal(hubreq)
	fmt.Printf("OZZIE: to %s\n%s\n", hdrHub, string(hubreqJSON))

	// Add the note to the notehub
	hreq, _ := http.NewRequest("POST", hdrHub, bytes.NewBuffer(hubreqJSON))
	hreq.Header.Set("User-Agent", "notecard.live")
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("X-Session-Token", hdrAPIKey)
	httpClient := &http.Client{Timeout: time.Second * 10}
	hrsp, err := httpClient.Do(hreq)
	if err != nil {
		httpRsp.WriteHeader(http.StatusBadRequest)
		httpRsp.Write([]byte(fmt.Sprintf("%s says: %s", hdrHub, err)))
		return
	}
	hrspJSON, _ := ioutil.ReadAll(hrsp.Body)
	fmt.Printf("%s\n", string(hrspJSON)) // OZZIE
	httpRsp.Write(hrspJSON)

	return

}
