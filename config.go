package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type Config struct {
	SocketPort  uint16
	APIPort     uint16
	LpacExeName string
	LpacPath    string
}

var CFG Config

func InitConfig() error {
	switch runtime.GOOS {
	case "windows":
		CFG.LpacExeName = "lpac.exe"
	default:
		CFG.LpacExeName = "lpac"
	}
	pwd, err := os.Getwd()
	if err != nil {
		return errors.New(fmt.Sprint("Failed to get pwd: ", err))
	}
	CFG.LpacPath = filepath.Join(pwd, CFG.LpacExeName)
	socketPort := strings.TrimSpace(os.Getenv("SOCKET_PORT"))
	if socketPort == "" {
		CFG.SocketPort = 1888
	} else {
		sPort, err := strconv.ParseUint(socketPort, 16, 16)
		if err != nil {
			return errors.New("Failed to parse SOCKET_PORT: " + err.Error())
		}
		CFG.SocketPort = uint16(sPort)
	}
	apiPort := strings.TrimSpace(os.Getenv("API_PORT"))
	if apiPort == "" {
		CFG.APIPort = 8008
	} else {
		aPort, err := strconv.ParseUint(apiPort, 16, 16)
		if err != nil {
			return errors.New("Failed to parse API_PORT: " + err.Error())
		}
		CFG.APIPort = uint16(aPort)
	}
	return nil
}
