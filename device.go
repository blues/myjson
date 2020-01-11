// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
    "fmt"
	"time"
    "bytes"
    "io/ioutil"
    "net/http"
)

// This maintains our server address, that we echo back to clients as the designated handler
var serverAddressIPv4 string

// Configuration for a device, generally stored in a database
type deviceConfig struct {
    DeviceUID string
    ProductUID string
    AppUID string
}

// Session state for a device, and when the state was last updated
type deviceState struct {
	config deviceConfig
    HubSessionHandler string
    HubSessionTicket string
    HubSessionTicketExpiresTimeNs int64
}

// Look up a device, provisioning it if necessary
func deviceGet(deviceUID string, productUID string) (device deviceState, err error) {

    // If we haven't yet looked up our own external address, do so
    if serverAddressIPv4 == "" {
        rsp, err2 := http.Get("http://checkip.amazonaws.com")
        if err2 != nil {
            err = fmt.Errorf("can't get our own IP address: %s", err2)
            return
        }
        defer rsp.Body.Close()
        buf, err2 := ioutil.ReadAll(rsp.Body)
        if err2 != nil {
            err = fmt.Errorf("error fetching IP addr: %s", err2)
            return
        }
        serverAddressIPv4 = string(bytes.TrimSpace(buf))
    }

	// If we were provisioning a new device here because we didn't find it
	// in the database, we'd look up which customer's application on the service
	// has reserved this productUID, and then we'd store it in the device data structure.
	device.config.AppUID = "app:test"
	
    // This is where we'd coordinate with all server instances to assign
	// a request handler, or to look up the assigned handler.  For our purposes
	// we will simply use the same server for the discovery server and handler server,
	// and will always refresh the expiration time so that it never expires.
	// Note that there's nothing special about "*" as a ticket name, except
	// to indicate that it is non-null and that we have assigned it.
    device.HubSessionHandler = "tcp:" + serverAddressIPv4 + configTCPPort
	device.HubSessionTicket = "*"
    device.HubSessionTicketExpiresTimeNs =
            time.Now().Add(time.Duration(configSessionTicketExpirationMinutes) * time.Minute).UnixNano()

	// We've successfully looked up or provisioned the device
	return
	
}
