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
	"strconv"
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
	hdrProduct := httpReq.Header.Get("X-Product")
	hdrFile := httpReq.Header.Get("X-File")
	hdrTemplate := httpReq.Header.Get("X-Template")
	hdrTemplateFlagBytes := httpReq.Header.Get("X-TemplateFlagBytes")

	fmt.Printf("lorawan: received uplink message\n")

	// Validate formats
	if hdrFormat != hdrFormatHelium && hdrFormat != hdrFormatTTN {
		httpRsp.WriteHeader(http.StatusBadRequest)
		msg := "X-Format must uniquely identify the JSON schema of inbound LoRaWAN messages\r\n"
		httpRsp.Write([]byte(msg))
		fmt.Printf("lorawan: %s\n", msg)
		return
	}
	if hdrHub == "" {
		hdrHub = notehubURL
	}
	if !strings.Contains(hdrHub, "://") {
		hdrHub = "https://" + hdrHub
	}
	if hdrProduct == "" {
		httpRsp.WriteHeader(http.StatusBadRequest)
		msg := "X-Product must be Notehub ProductUID\r\n"
		httpRsp.Write([]byte(msg))
		fmt.Printf("lorawan: %s\n", msg)
		return
	}
	if !strings.HasPrefix(hdrProduct, "product:") {
		hdrProduct = "product:" + hdrProduct
	}
	if hdrFile == "" {
		hdrFile = "data.qo"
	}
	if hdrTemplate == "" {
		httpRsp.WriteHeader(http.StatusBadRequest)
		msg := "X-Template must be the JSON Notefile Template describing the LoRaWAN payload\r\n"
		httpRsp.Write([]byte(msg))
		fmt.Printf("lorawan: %s\n", msg)
		return
	}
	payloadTemplate := map[string]interface{}{}
	err := json.Unmarshal([]byte(hdrTemplate), &payloadTemplate)
	if err != nil {
		httpRsp.WriteHeader(http.StatusBadRequest)
		msg := fmt.Sprintf("X-Template doesn't appear to be valid JSON: %s\r\n", err)
		httpRsp.Write([]byte(msg))
		fmt.Printf("lorawan: %s\n", msg)
		return
	}

	// Extract the essential fields from the request
	deviceUID := ""
	payload := []byte{}
	uplinkMessageJSON, err := ioutil.ReadAll(httpReq.Body)
	if err != nil {
		httpRsp.WriteHeader(http.StatusBadRequest)
		msg := fmt.Sprintf("can't read uplink message: %s\r\n", err)
		httpRsp.Write([]byte(msg))
		fmt.Printf("lorawan: %s\n", msg)
		return
	}
	if hdrFormat == hdrFormatHelium {
		msg := heliumUplinkMessage{}
		err = json.Unmarshal(uplinkMessageJSON, &msg)
		if err != nil {
			httpRsp.WriteHeader(http.StatusBadRequest)
			msg := fmt.Sprintf("can't decode Helium uplink message: %s\r\n%s\r\n", err, uplinkMessageJSON)
			httpRsp.Write([]byte(msg))
			fmt.Printf("lorawan: %s\n", msg)
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
			msg := fmt.Sprintf("can't decode TTN uplink message: %s\r\n", err)
			httpRsp.Write([]byte(msg))
			fmt.Printf("lorawan: %s\n", msg)
			return
		}
		payload = msg.PayloadRaw
		deviceUID = "dev:" + strings.ToLower(msg.DevID)
	}

	// Convert payload to body
	flagBytes, _ := strconv.Atoi(hdrTemplateFlagBytes)
	body, err := binDecodeFromTemplate(payload, hdrTemplate, flagBytes)
	if err != nil {
		httpRsp.WriteHeader(http.StatusBadRequest)
		msg := fmt.Sprintf("can't decode uplink payload from template: %s\r\n%s\r\n", err, hdrTemplate)
		httpRsp.Write([]byte(msg))
		fmt.Printf("lorawan: %s\n", msg)
		return
	}

	// Repeat this in case we need to create the device
	var hubrspJSON []byte
	for i := 0; i < 2; i++ {

		// Create the 'add note' request
		hubreq := notehub.HubRequest{}
		hubreq.Req = "note.add"
		hubreq.Body = &body
		hubreq.NotefileID = hdrFile
		hubreq.ProductUID = hdrProduct
		hubreq.DeviceUID = deviceUID
		hubreqJSON, err := json.Marshal(hubreq)

		// Add the note to the notehub
		hreq, _ := http.NewRequest("POST", hdrHub, bytes.NewBuffer(hubreqJSON))
		hreq.Header.Set("User-Agent", "notecard.live")
		hreq.Header.Set("Content-Type", "application/json")
		hreq.Header.Set("X-Session-Token", hdrAPIKey)
		httpClient := &http.Client{Timeout: time.Second * 10}
		hrsp, err := httpClient.Do(hreq)
		if err != nil {
			httpRsp.WriteHeader(http.StatusBadRequest)
			msg := fmt.Sprintf("%s says: %s", hdrHub, err)
			httpRsp.Write([]byte(msg))
			fmt.Printf("lorawan: %s\n", msg)
			return
		}
		hubrspJSON, _ = ioutil.ReadAll(hrsp.Body)
		hubrsp := notehub.HubRequest{}
		err = json.Unmarshal(hubrspJSON, &hubrsp)
		if err != nil {
			httpRsp.WriteHeader(http.StatusBadRequest)
			msg := "invalid JSON response from notehub"
			httpRsp.Write([]byte(msg))
			fmt.Printf("lorawan: %s\n", msg)
			return
		}

		// Create the device if it doesn't exist
		if !strings.Contains(hubrsp.Err, "{device-noexist}") {
			break
		}

		// Provision the device
		hubreq = notehub.HubRequest{}
		hubreq.Req = "hub.env.set"
		hubreq.Scope = "device"
		hubreq.Provision = true
		hubreq.DeviceUID = deviceUID
		hubreq.ProductUID = hdrProduct
		hubreqJSON, err = json.Marshal(hubreq)
		hreq, _ = http.NewRequest("POST", hdrHub, bytes.NewBuffer(hubreqJSON))
		hreq.Header.Set("User-Agent", "notecard.live")
		hreq.Header.Set("Content-Type", "application/json")
		hreq.Header.Set("X-Session-Token", hdrAPIKey)
		httpClient = &http.Client{Timeout: time.Second * 10}
		hrsp, err = httpClient.Do(hreq)
		if err != nil {
			httpRsp.WriteHeader(http.StatusBadRequest)
			msg := fmt.Sprintf("on device provisioning, %s says: %s", hdrHub, err)
			httpRsp.Write([]byte(msg))
			fmt.Printf("lorawan: %s\n", msg)
			return
		}
		hubrspJSON, _ = ioutil.ReadAll(hrsp.Body)
		hubrsp = notehub.HubRequest{}
		err = json.Unmarshal(hubrspJSON, &hubrsp)
		if err != nil {
			httpRsp.WriteHeader(http.StatusBadRequest)
			msg := "invalid JSON response from notehub"
			httpRsp.Write([]byte(msg))
			fmt.Printf("lorawan: %s\n", msg)
			return
		}
		if hubrsp.Err != "" {
			break
		}

	}

	httpRsp.Write(hubrspJSON)
	fmt.Printf("lorawan: %s\n", hubrspJSON)

	return

}
