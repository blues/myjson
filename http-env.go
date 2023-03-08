// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// Proxy handler so that we may make external references from local pages without CORS issues.	Note that
// this ONLY is supported for JSON queries.
func inboundWebEnvHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

	// Get the required args
	_, args := HTTPArgs(httpReq, "")
	product := args["product"]
	if product == "" {
		fmt.Fprintf(httpRsp, "product not specified")
		return
	}
	device := args["device"]
	if device == "" {
		fmt.Fprintf(httpRsp, "device not specified")
		return
	}

	switch httpReq.Method {

	case "GET":
		envJSON, statusCode, err := envGet(httpReq, product, device)
		if err != nil {
			fmt.Fprintf(httpRsp, "%s", err)
			return
		}
		httpRsp.WriteHeader(statusCode)
		httpRsp.Write(envJSON)
		return

	case "POST":
		envJSON, err := ioutil.ReadAll(httpReq.Body)
		if err != nil {
			fmt.Fprintf(httpRsp, "%s", err)
			return
		}
		statusCode, err := envSet(httpReq, product, device, envJSON)
		if err != nil {
			fmt.Fprintf(httpRsp, "%s", err)
			return
		}
		httpRsp.WriteHeader(statusCode)
		return
	}

	fmt.Fprintf(httpRsp, "only GET and POST methods are supported")

}

// Call the notehub to get env vars
func envGet(httpReq *http.Request, product string, device string) (rsp []byte, statusCode int, err error) {

	// Formulate the request
	req := map[string]interface{}{}
	req["product"] = product
	req["device"] = device
	req["req"] = "hub.env.get"
	req["scope"] = "device"

	// Marshal the request
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return
	}

	// Create the new HTTP request
	var httpreq *http.Request
	httpreq, err = http.NewRequest("POST", notehubURL, bytes.NewBuffer(reqJSON))
	if err != nil {
		err = fmt.Errorf("nr err: %s", err)
		return
	}

	// Perform the HTTP I/O
	httpclient := &http.Client{Timeout: time.Second * 15}
	httpresp, err := httpclient.Do(httpreq)
	if err != nil {
		err = fmt.Errorf("do err: %s", err)
		return
	}
	statusCode = httpresp.StatusCode

	// Read the response
	rsp, err = ioutil.ReadAll(httpresp.Body)

	// Done
	return

}

// Call the notehub to set env vars
func envSet(httpReq *http.Request, product string, device string, reqJSON []byte) (statusCode int, err error) {

	// Unmarshal the request
	req := map[string]interface{}{}
	err = json.Unmarshal(reqJSON, &req)
	if err != nil {
		return
	}

	// Re-formulate it with constraints
	delete(req, "app")
	req["product"] = product
	req["device"] = device
	req["req"] = "hub.env.set"
	req["scope"] = "device"

	// Marshal the request
	reqJSON, err = json.Marshal(req)
	if err != nil {
		return
	}

	// Create the new HTTP request
	var httpreq *http.Request
	httpreq, err = http.NewRequest("POST", notehubURL, bytes.NewBuffer(reqJSON))
	if err != nil {
		err = fmt.Errorf("nr err: %s", err)
		return
	}

	// Perform the HTTP I/O
	httpclient := &http.Client{Timeout: time.Second * 15}
	httpresp, err := httpclient.Do(httpreq)
	if err != nil {
		err = fmt.Errorf("do err: %s", err)
		return
	}
	statusCode = httpresp.StatusCode

	// Done
	return

}
