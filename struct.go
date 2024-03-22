package main

import "encoding/json"

type Payload struct {
	Code  int             `json:"code"`
	Func  string          `json:"func"`
	Param string          `json:"param"`
	Ecode int             `json:"ecode"`
	Data  json.RawMessage `json:"data"`
}

type Request struct {
	Type    string  `json:"type"`
	Payload Payload `json:"payload"`
}

type Notification struct {
	SeqNumber                  int    `json:"seqNumber"`
	ProfileManagementOperation string `json:"profileManagementOperation"`
	NotificationAddress        string `json:"notificationAddress"`
	Iccid                      string `json:"iccid"`
}

type PullInfo struct {
	SMDP        string
	MatchID     string
	ObjectID    string
	ConfirmCode string
	IMEI        string
}
