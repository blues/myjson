// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
    "time"
    "github.com/rayozzie/notelib"
    "github.com/rayozzie/note-go/note"
)

// NotehubDiscover is responsible for discovery of information about the services and apps
func NotehubDiscover(deviceUID string, deviceSN string, productUID string, initialDiscovery bool) (info notelib.DiscoverInfo, err error) {

    // Return basic info about this server, if that's all that we're looking for
    if (deviceUID == "") {
        info.HubEndpointID = note.DefaultHubEndpointID
        info.HubTimeNs = time.Now().UnixNano()
        return
    }

    // Look up the device
    device, err2 := deviceGet(deviceUID, productUID)
    if err2 != nil {
        err = err2
        return
    }

    // Return how long the handler will be assigned to this device
    info.HubSessionHandler = device.HubSessionHandler
    info.HubSessionTicket = device.HubSessionTicket
    info.HubSessionTicketExpiresTimeNs = device.HubSessionTicketExpiresTimeNs
    info.HubDeviceStorageObject = notelib.FileStorageObject(deviceUID)
    info.HubDeviceAppUID = device.config.AppUID
    info.HubEndpointID = note.DefaultHubEndpointID
    info.HubTimeNs = time.Now().UnixNano()

    // Done
    return

}
