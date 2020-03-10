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
)

// Proxy handler so that we may make external references from local pages without CORS issues
func inboundWebProxyHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

    // Get the body if supplied
    reqBody, err := ioutil.ReadAll(httpReq.Body)
    if err != nil {
		reqBody = []byte("")
    }

	// Get the target
	_, args := HTTPArgs(httpReq, "")
	var proxyURL string
	proxyURL, err = url.QueryUnescape(args["url"])
	if err != nil {
		httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
		return
	}
	fmt.Printf("proxy: %s\n", proxyURL)
	
	// Perform the transaction
	req, err1 := http.NewRequest(httpReq.Method, proxyURL, bytes.NewBuffer(reqBody))
	if err1 != nil {
		fmt.Printf("proxy NR err: %s\n", err1)
		httpRsp.Write([]byte(fmt.Sprintf("%s", err1)))
		return
	}

	httpclient := &http.Client{	Timeout: time.Second * 15 }
	resp, err2 := httpclient.Do(req)
	if err2 != nil {
		fmt.Printf("proxy DO err: %s\n", err2)
		httpRsp.Write([]byte(fmt.Sprintf("%s", err2)))
		return
	}

	httpRsp.WriteHeader(resp.StatusCode)
	var rspbuf []byte
	rspbuf, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("proxy RD err: %s\n", err2)
		httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
		return
	}
	httpRsp.Write(rspbuf)
	
}
