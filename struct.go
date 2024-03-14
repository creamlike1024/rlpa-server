package main

type Payload struct {
	Func  string `json:"func"`
	Param string `json:"param"`
	Ecode int    `json:"ecode"`
}

type Request struct {
	Type    string  `json:"type"`
	Payload Payload `json:"payload"`
}
