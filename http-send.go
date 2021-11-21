// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Serves Health Checks
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

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

	// Send twilio messages
	// https://www.twilio.com/blog/2014/06/sending-sms-from-your-go-app.html
	for _, toSMS := range alert.SMS {
		accountSid := Config.TwilioSID
		authToken := Config.TwilioSAK
		urlStr := "https://api.twilio.com/2010-04-01/Accounts/" + accountSid + "/Messages.json"
		v := url.Values{}
		v.Set("To", toSMS)
		v.Set("From", Config.TwilioSMS)
		if alert.Text != "" {
			v.Set("Body", alert.Text)
		} else {
			v.Set("Body", alert.Body)
		}
		rb := *strings.NewReader(v.Encode())
		client := &http.Client{}
		req, _ := http.NewRequest("POST", urlStr, &rb)
		req.SetBasicAuth(accountSid, authToken)
		req.Header.Add("Accept", "application/json")
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		resp, _ := client.Do(req)
		fmt.Printf("send: %s: %s\n", toSMS, resp.Status)
	}

	return

}
