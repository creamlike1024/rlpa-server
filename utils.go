package main

import (
	"errors"
	_ "image/jpeg"
	"strings"
)

func DecodeLpaActivationCode(code string) (info PullInfo, confirmCodeNeeded bool, err error) {
	// ref: https://www.gsma.com/esim/wp-content/uploads/2020/06/SGP.22-v2.2.2.pdf#page=111
	err = errors.New("LPA Activation Code format error")
	code = strings.TrimSpace(code)
	var ok bool
	if code, ok = strings.CutPrefix(code, "LPA:"); !ok {
		return
	}
	switch parts := strings.Split(code, "$"); parts[0] {
	case "1": // Activation Code Format
		var codeNeeded string
		bindings := []*string{&info.SMDP, &info.MatchID, &info.ObjectID, &codeNeeded}
		for index, value := range parts[1:] {
			*bindings[index] = strings.TrimSpace(value)
		}
		confirmCodeNeeded = codeNeeded == "1"
		if info.SMDP != "" {
			err = nil
		}
	}
	return
}

func CompleteActivationCode(input string) string {
	// 如果输入已经以 LPA:1$ 开始，则认为它是完整的
	if strings.HasPrefix(input, "LPA:1$") {
		return input
	}
	// 1$rspAddr$matchID
	if strings.HasPrefix(input, "1$") {
		return "LPA:" + input
	}
	// $rspAddr$matchID
	if strings.HasPrefix(input, "$") {
		return "LPA:1" + input
	}
	return input
}
