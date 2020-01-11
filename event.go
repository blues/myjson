// Copyright 2017 Inca Roads LLC.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"fmt"
    "encoding/json"
    "github.com/rayozzie/notelib"
)

// Notify procedure
func notehubNotify(context interface{}, local bool, file *notelib.Notefile, req *notelib.Event) (err error) {

	reqJSON, err := json.Marshal(req)
	if err != nil {
		return err
	}

	fmt.Printf("\nCHANGE NOTIFICATION RECEIVED:\n%s\n", string(reqJSON))

	return

}
