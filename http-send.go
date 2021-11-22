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
	"sync"
	"time"

	"github.com/blues/note-go/note"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

// AlertMessage is the format of a message coming in from the route
type AlertMessage struct {
	SMS     []string   `json:"sms"`
	Email   []string   `json:"email"`
	Text    string     `json:"text"`
	Body    string     `json:"body"`
	Minutes uint32     `json:"minutes"`
	Event   note.Event `json:"event"`
}

// We retain an in-memory array of future messages to suppress
type suppressMessage struct {
	sms     string
	email   string
	expires time.Time
	text    string
}

// In-memory array, plus integrity protection
var smLock sync.RWMutex
var suppressMessages []suppressMessage

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

		// Ensure that we don't send duplicates
		if shouldBeSuppressed(toSMS, "", alert.Text, alert.Minutes) {
			continue
		}

		// Send the SMS
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
			fmt.Printf("send: %s (%s): %s\n", toSMS, resp.Status, bodyBytes)
		} else {
			fmt.Printf("send: %s: %s\n", toSMS, resp.Status)
		}

	}

	// Send twilio/sendgrid Email messages
	// https://docs.sendgrid.com/for-developers/sending-email/v3-go-code-example
	for _, toEmail := range alert.Email {

		// Ensure that we don't send duplicates
		if shouldBeSuppressed("", toEmail, alert.Text, alert.Minutes) {
			continue
		}

		// Send the email
		from := mail.NewEmail(Config.TwilioFrom, Config.TwilioEmail)
		fmt.Printf("%s %s %v\n", Config.TwilioFrom, Config.TwilioEmail, from)
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

// See if a message should be suppressed
func shouldBeSuppressed(toSMS string, toEmail string, text string, minutes uint32) (suppress bool) {

	// Rebuild the list of messages to be suppressed
	smLock.Lock()
	newSM := []suppressMessage{}

	// See if we can find an unexpired entry, and garbage collect
	now := time.Now()
	for _, sm := range suppressMessages {
		if now.Before(sm.expires) {
			newSM = append(newSM, sm)
			if sm.text == text {
				if sm.sms != "" && sm.sms == toSMS {
					suppress = true
				}
				if sm.email != "" && sm.email == toEmail {
					suppress = true
				}
			}
		}
	}

	// If wwe shouldn't suppress, suppress future texts
	if !suppress {
		var sm suppressMessage
		sm.sms = toSMS
		sm.email = toEmail
		sm.expires = now.Add(time.Minute * time.Duration(minutes))
		sm.text = text
		newSM = append(newSM, sm)
	}

	// Update the list and exit
	suppressMessages = newSM
	smLock.Unlock()
	return

}
