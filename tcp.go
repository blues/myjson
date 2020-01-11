// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Inbound TCP support
package main

import (
    "net"
	"fmt"
    "github.com/rayozzie/notelib"
)

// tcpListenHandler kicks off TCP request server
func tcpListenHandler(portNumber string) {

    fmt.Printf("Now handling inbound requests on tcp%s\n", portNumber)

    ServerAddr, err := net.ResolveTCPAddr("tcp", portNumber)
    if err != nil {
        fmt.Printf("tcp: error resolving TCP port: %v\n", err)
        return
    }

    connServer, err := net.ListenTCP("tcp", ServerAddr)
    if err != nil {
        fmt.Printf("tcp: error listening on TCP port: %v\n", err)
        return
    }
    defer connServer.Close()

    for {

        // Accept the TCP connection
        connSession, err := connServer.AcceptTCP()
        if err != nil {
            fmt.Printf("tcp: error accepting TCP session: %v\n", err)
            continue
        }

        // The scope of a TCP connection may be many requests, so dispatch
        // this to a goroutine which will deal with the session until it is closed.
        go tcpSessionHandler(connSession)

    }

}

// Process requests for the duration of a session being open
func tcpSessionHandler(connSession net.Conn) {

    // Keep track of this, from a resource consumption perspective
    fmt.Printf("Opened session\n")

    // Always start with a blank, inactive Session Context
    sessionContext := notelib.HubSessionContext{}
    for {
        var requestPresent bool
        var request, response []byte
        var err error

        // Extract a request from the wire, and exit if error
        _, requestPresent, request, err = notelib.WireReadRequest(connSession, true)
        if err != nil {
			if (!notelib.ErrorContains(err, "{closed}")) {
	            fmt.Printf("tcp: error reading request: %s\n", err)
			}
            break
        }

        // Now we know we're going to process it
        if sessionContext.DeviceUID == "" {
            fmt.Printf("\nReceived %d-byte message\n", len(request))
        } else {
            fmt.Printf("\nReceived %d-byte message from %s\n", len(request), sessionContext.DeviceUID)
        }

		// Process the response
        if !requestPresent {

	        // Data was sent, but it didn't contain a request protobuf.  This is a special "echo"
			// feature used for session connectivity and robustness testing.
            response = request

        } else {

            // Process the request in the protobuf.  Note that if we get an error, there's no possibility
            // of returning it, so we just print it on the server console.
            response, err = notelib.HubRequest(&sessionContext, request, notehubNotify, nil)
            if err != nil {
                fmt.Printf("tcp: error processing request: %s\n", err)
                break
            }

        }

        // On TCP we must ALWAYS respond so the client doesn't hang waiting
        connSession.Write(response)

    }

    // Close the connection
    connSession.Close()
    fmt.Printf("\nClosed session\n\n")

}
