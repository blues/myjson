// Copyright © 2016 The Things Network
// Use of this source code is governed by the MIT license found with the
// source code from where this was derived:
// https://github.com/TheThingsNetwork/ttn/core/types

package main

type ttnApplicationIDs struct {
	ApplicationID string `json:"application_id,omitempty"`
}

type ttnEndDeviceIDs struct {
	DeviceID       string            `json:"device_id,omitempty"`
	ApplicationIDs ttnApplicationIDs `json:"application_ids,omitempty"`
	DevEUI         string            `json:"dev_eui,omitempty"`
	JoinEUI        string            `json:"join_eui,omitempty"`
	DevAddr        string            `json:"dev_addr,omitempty"`
}

type ttnGatewayIDs struct {
	GatewayID string `json:"gateway_id,omitempty"`
	EUI       string `json:"eui,omitempty"`
}

type ttnRxMetadata struct {
	GatewayIDs  ttnGatewayIDs `json:"gateway_ids,omitempty"`
	RxTime      string        `json:"time,omitempty"`
	RxTimestamp int64         `json:"timestamp,omitempty"`
	RSSI        float64       `json:"rssi,omitempty"`
	ChannelRSSI float64       `json:"channel_rssi,omitempty"`
	SNR         float64       `json:"snr,omitempty"`
	UplinkToken []byte        `json:"uplink_token,omitempty"`
}

type ttnSettingDataRateLoRa struct {
	Bandwidth       uint32 `json:"bandwidth,omitempty"`
	SpreadingFactor uint32 `json:"spreading_factor,omitempty"`
}

type ttnSettingDataRate struct {
	LoraDataRate ttnSettingDataRateLoRa `json:"lora,omitempty"`
}

type ttnSettings struct {
	DataRate      ttnSettingDataRate `json:"data_rate,omitempty"`
	DataRateIndex uint32             `json:"data_rate_index,omitempty"`
	CodingRate    string             `json:"coding_rate,omitempty"`
	Frequency     string             `json:"frequency,omitempty"`
	Timestamp     int64              `json:"timestamp,omitempty"`
	TimeStr       string             `json:"time,omitempty"`
}

type ttnNetworkIDs struct {
	NetID     string `json:"net_id,omitempty"`
	TenantID  string `json:"tenant_id,omitempty"`
	ClusterID string `json:"cluster_id,omitempty"`
}

type ttnUplink struct {
	SessionKey      string          `json:"session_key_id,omitempty"`
	FPort           uint32          `json:"f_port"`
	FCount          uint32          `json:"f_cnt"`
	Payload         []byte          `json:"frm_payload,omitempty"`
	RxMetadata      []ttnRxMetadata `json:"rx_metadata,omitempty"`
	Settings        ttnSettings     `json:"settings,omitempty"`
	ReceivedAt      string          `json:"received_at,omitempty"`
	ConsumedAirtime string          `json:"consumed_airtime,omitempty"`
	NetworkIDs      ttnNetworkIDs   `json:"network_ids,omitempty"`
}

type ttnUplinkMessage struct {
	EndDeviceIDs   ttnEndDeviceIDs `json:"end_device_ids,omitempty"`
	CorrelationIDs []string        `json:"correlation_ids,omitempty"`
	ReceivedAt     string          `json:"received_at,omitempty"`
	UplinkMessage  ttnUplink       `json:"uplink_message,omitempty"`
}
