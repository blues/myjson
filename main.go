// Copyright 2020 Blues Inc.  All rights reserved. 
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"os"
	"time"
)

// Directory in the home directory that will be used for data
const configDataDirectoryBase = "/data/"
var configDataDirectory = ""

// Main service entry point
func main() {

	// Compute folder location
	configDataDirectory = os.Getenv("HOME") + configDataDirectoryBase

    // Spawn the console input handler
    go inputHandler()

    // Init our web request inbound server
    go HTTPInboundHandler(":80")

	// Wait forever
    for {
        time.Sleep(5 * time.Minute)
    }

}
