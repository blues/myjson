// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
	"bytes"
	"time"
	"io/ioutil"
	"net/http"
	"net/url"
	"encoding/json"
)

// Proxy handler so that we may make external references from local pages without CORS issues.	Note that
// this ONLY is supported for JSON queries.
func inboundWebProxyHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

	// Get the body if supplied
	reqBody, err := ioutil.ReadAll(httpReq.Body)
	if err != nil {
		reqBody = []byte("")
	}

	// Verify that it is compliant JSON
	var jobj map[string]interface{}
	err = json.Unmarshal(reqBody, &jobj)
	if err != nil {
		httpRsp.Write([]byte(fmt.Sprintf("{\"err\":\"%s\"}", err)))
		return
	}

	// Get the target
	_, args := HTTPArgs(httpReq, "")
	var proxyURL string
	proxyURL, err = url.QueryUnescape(args["url"])
	if err != nil {
		httpRsp.Write([]byte(fmt.Sprintf("{\"err\":\"%s\"}", err)))
		return
	}
	fmt.Printf("proxy: %s\n", proxyURL)

	// Perform the transaction several times to cover the Balena problem that yields
	// a strange web page on a semi-random basis.
	var rspbuf []byte
	var resp *http.Response
	for i:=0;;i++ {
		
		var req *http.Request
		req, err = http.NewRequest(httpReq.Method, proxyURL, bytes.NewBuffer(reqBody))
		if err != nil {
			fmt.Printf("proxy NR err: %s\n", err)
			httpRsp.Write([]byte(fmt.Sprintf("{\"err\":\"%s\"}", err)))
			return
		}

		httpclient := &http.Client{	Timeout: time.Second * 15 }
		resp, err = httpclient.Do(req)
		if err != nil {
			fmt.Printf("proxy DO err: %s\n", err)
			httpRsp.Write([]byte(fmt.Sprintf("{\"err\":\"%s\"}", err)))
			return
		}

		rspbuf, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("proxy RD err: %s\n", err)
			httpRsp.Write([]byte(fmt.Sprintf("{\"err\":\"%s\"}", err)))
			return
		}

		// Validate that it's compliant JSON
		err = json.Unmarshal(rspbuf, &jobj)
		if err == nil {
			break
		}

		// Exit after N retries
		var maxRetries = 5
		if i > maxRetries {
			err = fmt.Errorf("proxy: server isn't returning JSON")
			fmt.Printf("%s\n", err)
			httpRsp.Write([]byte(fmt.Sprintf("{\"err\":\"%s\"}", err)))
			return
		}

		fmt.Printf("proxy: received non-JSON on try #%d/%d\n", i+1, maxRetries)

	}

	httpRsp.WriteHeader(resp.StatusCode)
	httpRsp.Write(rspbuf)

}
