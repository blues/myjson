// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Serves Health Checks
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/blues/note-go/note"
)

// AlertMessage
type AlertMessage struct {
	SMS   []string   `json:"sms"`
	Email []string   `json:"email"`
	Text  string     `json:"text"`
	Body  string     `json:"body"`
	Event note.Event `json:"event"`
}

// Ping handler
func inboundWebSendHandler(httpRsp http.ResponseWriter, httpReq *http.Request) {

	// Get the body if supplied
	alertJSON, err := ioutil.ReadAll(httpReq.Body)
	if err != nil {
		alertJSON = []byte("{}")
	}
	_ = alertJSON

	// Debug
	var alert AlertMessage
	err = note.JSONUnmarshal(alertJSON, &alert)
	if err != nil {
		httpRsp.Write([]byte(fmt.Sprintf("%s", err)))
	}

	fmt.Printf("*****\n")
	fmt.Printf("%s\n", alertJSON)
	fmt.Printf("%v\n", alert)
	fmt.Printf("*****\n")

	return

}
