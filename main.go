// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"os"
	"time"
)

// Notehub URL to use
const notehubURL = "https://api.notefile.net"

// Directory in the home directory that will be used for data
const configDataDirectoryBase = "/data/"

var configDataDirectory = ""

// Maximum number of retained posts per target
const configMaxPosts = 1000

// Main service entry point
func main() {

	// Read creds
	ServiceReadConfig()

	// Compute folder location
	configDataDirectory = os.Getenv("HOME") + configDataDirectoryBase

	// Spawn the console input handler
	go inputHandler()

	// Init our web request inbound server
	go HTTPInboundHandler(":80")

	// Purge hourly
	for {
		targets := tailTargets()
		for _, target := range targets {
			tail(target, configMaxPosts, true, nil)
		}
		time.Sleep(60 * time.Minute)
	}

}
