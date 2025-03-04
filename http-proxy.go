// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Rate limiting because of Balena's proxy
const throttleMs = 250

var throttleTime int64

// Proxy handler so that we may make external references from local pages without CORS issues.	Note that
// this ONLY is supported for JSON queries.
func inboundWebProxyHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

	// Throttle because of Balena's rate limit
	msSinceLastTransaction := (time.Now().UnixNano() - throttleTime) / 1000000
	if msSinceLastTransaction < throttleMs {
		time.Sleep(time.Duration(throttleMs-msSinceLastTransaction) * time.Millisecond)
	}
	throttleTime = time.Now().UnixNano()

	// Get the body if supplied
	reqBody, err := io.ReadAll(httpReq.Body)
	if err != nil {
		reqBody = []byte("")
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
	var maxRetries = 5
	for i := 0; ; i++ {

		var req *http.Request
		req, err = http.NewRequest(httpReq.Method, proxyURL, bytes.NewBuffer(reqBody))
		if err != nil {
			fmt.Printf("proxy NR err: %s\n", err)
			httpRsp.Write([]byte(fmt.Sprintf("{\"err\":\"%s\"}", err)))
			return
		}

		httpclient := &http.Client{Timeout: time.Second * 15}
		resp, err = httpclient.Do(req)
		if err != nil {
			fmt.Printf("proxy DO err: %s\n", err)
			httpRsp.Write([]byte(fmt.Sprintf("{\"err\":\"%s\"}", err)))
			return
		}

		rspbuf, err = io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("proxy RD err: %s\n", err)
			httpRsp.Write([]byte(fmt.Sprintf("{\"err\":\"%s\"}", err)))
			return
		}

		// Validate that it's compliant JSON
		var jobj map[string]interface{}
		err = json.Unmarshal(rspbuf, &jobj)
		if err == nil {

			// See if there was an I/O error to the card, and retry if so
			if !strings.Contains(string(rspbuf), "{io}") {
				break
			}
			if i > maxRetries {
				httpRsp.Write([]byte("{\"err\":\"proxy: cannot communicate with notecard {io}\"}"))
				return
			}

			fmt.Printf("proxy: I/O error on try #%d/%d\n", i+1, maxRetries)

		} else {

			fmt.Printf("proxy: received non-JSON on try #%d/%d\n", i+1, maxRetries)
			if i > maxRetries {
				fmt.Printf("%s\n", string(rspbuf))
				fmt.Printf("%s\n", err)
				httpRsp.Write([]byte(fmt.Sprintf("{\"err\":\"%s\"}", err)))
				return
			}

		}

		// Essential to coming out of Balena's penalty box
		time.Sleep(2 * time.Second)

	}

	httpRsp.WriteHeader(resp.StatusCode)
	httpRsp.Write(rspbuf)

}
