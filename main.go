// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"os"
	"time"
)

// Directory in the home directory that will be used for data
const configDataDirectory = "/json"

// Default amount of time when the device needs to come back to the discovery server for a new handler
const configSessionTicketExpirationMinutes = (48*24*60)

// TCP port for listener
const configTCPPort string = ":80"


// Main service entry point
func main() {

    // Initialize callbacks
	notelib.FileSetStorageLocation(os.Getenv("HOME") + configDataDirectory)
	notelib.HubSetDiscover(NotehubDiscover)

    // Spawn the console input handler
    go inputHandler()

	// Wait forever
    for {
        time.Sleep(5 * time.Minute)
    }

}
