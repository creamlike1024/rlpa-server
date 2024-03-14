package main

const (
	TypeExecute = 0
	TypeFinish  = 1

	ResultFinished         = 0
	ResultClientDisconnect = 1
	ResultError            = 2
)

type ShellRequest struct {
	Type    int    `json:"type"`
	Command string `json:"command"`
}

type ShellResponse struct {
	Result int                    `json:"result"`
	Stdout map[string]interface{} `json:"stdout"`
	Stderr string                 `json:"stderr"`
}
