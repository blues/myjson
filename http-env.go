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

// Notehub to use
const notehub = "https://api.notefile.net"

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
	if product == "" {
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
	return

}

// Call the notehub to get env vars
func envGet(httpReq *http.Request, product string, device string) (env []byte, statusCode int, err error) {

	// Formulate the request
	body := map[string]interface{}{}
	body["product"] = product
	body["device"] = device
	body["req"] = "env.get"
	body["scope"] = "device"

	// Marshal the request
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return
	}

	// Create the new HTTP request
	var req *http.Request
	req, err = http.NewRequest("POST", notehub, bytes.NewBuffer(bodyJSON))
	if err != nil {
		err = fmt.Errorf("nr err: %s", err)
		return
	}

	// Perform the HTTP I/O
	httpclient := &http.Client{Timeout: time.Second * 15}
	resp, err := httpclient.Do(req)
	if err != nil {
		err = fmt.Errorf("do err: %s", err)
		return
	}
	statusCode = resp.StatusCode

	// Read the response
	env, err = ioutil.ReadAll(resp.Body)

	// Done
	return

}

// Call the notehub to set env vars
func envSet(httpReq *http.Request, product string, device string, envJSON []byte) (statusCode int, err error) {

	// Unmarshal the env
	env := map[string]interface{}{}
	err = json.Unmarshal(envJSON, &env)
	if err != nil {
		return
	}

	// Formulate the request
	body := map[string]interface{}{}
	body["product"] = product
	body["device"] = device
	body["req"] = "env.set"
	body["scope"] = "device"
	body["env"] = env

	// Marshal the request
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return
	}

	// Create the new HTTP request
	var req *http.Request
	req, err = http.NewRequest("POST", notehub, bytes.NewBuffer(bodyJSON))
	if err != nil {
		err = fmt.Errorf("nr err: %s", err)
		return
	}

	// Perform the HTTP I/O
	httpclient := &http.Client{Timeout: time.Second * 15}
	resp, err := httpclient.Do(req)
	if err != nil {
		err = fmt.Errorf("do err: %s", err)
		return
	}
	statusCode = resp.StatusCode

	// Done
	return

}
