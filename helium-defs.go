// Copyright 2020 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

type heliumLabel struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	OrganizationID string `json:"organization_id"`
}

type heliumMetadata struct {
	ADRAllowed     bool          `json:"adr_allowed"`
	CFListEnabled  bool          `json:"cf_list_enabled"`
	MultiBuy       int32         `json:"multi_buy"`
	OrganizationID string        `json:"organization_id"`
	Labels         []heliumLabel `json:"labels"`
}

type heliumHotspot struct {
	Channel           uint32  `json:"channel"`
	Frequency         float64 `json:"frequency"`
	HoldTime          uint32  `json:"hold_time"`
	ID                string  `json:"id"`
	Lat               float64 `json:"lat"`
	Lon               float64 `json:"long"`
	Name              string  `json:"name"`
	ReportedAtEpochMs int64   `json:"reported_at"`
	RSSI              int32   `json:"rssi"`
	SNR               int32   `json:"snr"`
	Spreading         string  `json:"spreading"`
	Status            string  `json:"status"`
}

type heliumUplinkMessage struct {
	AppEUI            string          `json:"app_eui"`
	DeviceEUI         string          `json:"dev_eui"`
	DeviceAddr        string          `json:"devaddr"`
	DownlinkURL       string          `json:"downlink_url"`
	FrameCount        uint32          `json:"fcnt"`
	Hotspots          []heliumHotspot `json:"hotspots"`
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Payload           []byte          `json:"payload"`
	PayloadLen        uint32          `json:"payload_size"`
	Port              uint32          `json:"port"`
	ReportedAtEpochMs int64           `json:"reported_at"`
	UUID              string          `json:"uuid"`
	Metadata          heliumMetadata  `json:"metadata"`
}
