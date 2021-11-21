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
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// AlertMessage is the format of a message coming in from the route
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

	// Send twilio SMS messages
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
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			fmt.Printf("send: %s: %s\n", toSMS, bodyBytes)
		} else {
			fmt.Printf("send: %s: %s\n", toSMS, resp.Status)
		}
	}

	// Send twilio/sendgrid Email messages
	// https://docs.sendgrid.com/for-developers/sending-email/v3-go-code-example
	for _, toEmail := range alert.Email {
		from := mail.NewEmail(Config.TwilioFrom, Config.TwilioEmail)
		subject := alert.Text
		to := mail.NewEmail("", toEmail)
		if subject == "" {
			subject = "(no alert text specified)"
		}
		plainTextContent := alert.Body
		if plainTextContent == "" {
			plainTextContent = subject
		}
		htmlContent := ""
		message := mail.NewSingleEmail(from, subject, to, plainTextContent, htmlContent)
		client := sendgrid.NewSendClient(Config.TwilioSendgridAPIKey)
		response, err := client.Send(message)
		if err != nil {
			fmt.Printf("send email to %s: %s\n", toEmail, err)
		} else {
			fmt.Printf("send email to %s: %d %s %s\n", toEmail, response.StatusCode, response.Body, response.Headers)
		}
	}

	return

}
