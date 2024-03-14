package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"os/exec"
	"time"
)

const keepaliveDuration time.Duration = 60 * time.Second

// 为了避免混淆，剔除 0, O, o, 1, l, I, 2, Z, B, 8
const letters = "abcdefghijkmnpqrstuvwxyzACDEFGHJKLMNPQRSTUVWXY345679"
const digits = "0123456789"

const (
	WaitingMode             = 0
	DownloadMode            = 1
	ProcessNotificationMode = 2
	ShellMode               = 3
)

var Credentials map[string]string

type RLPAClient struct {
	IsClosing       bool
	ID              string
	WorkMode        int
	Socket          net.Conn
	Packet          RLPAPacket
	CMD             *exec.Cmd
	LpacStdin       io.WriteCloser
	LpacStdout      io.ReadCloser
	LpacStderr      io.ReadCloser
	ResponseWaiting bool
	ResponseChan    chan []byte
	APILocked       bool
	KeepAliveTimer  *time.Timer
}

var Clients []*RLPAClient

func NewRLPAClient(conn net.Conn) *RLPAClient {
	return &RLPAClient{
		Socket:       conn,
		Packet:       NewRLPAPacket(0x00, []byte{}),
		ResponseChan: make(chan []byte),
	}
}

func (c *RLPAClient) GenCredential() string {
	for {
		id := generateID(2)
		if _, exists := Credentials[id]; !exists {
			passwd := generatePasswd(2)
			Credentials[id] = passwd
			c.ID = id
			return passwd
		}
	}
}

// generateID 生成 n 位随机字符
func generateID(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// generatePasswd 生成 n 位随机数字
func generatePasswd(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = digits[rand.Intn(len(digits))]
	}
	return string(b)
}

func FindClient(id string) (*RLPAClient, error) {
	for _, c := range Clients {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, errors.New("client not found")
}

func (c *RLPAClient) RemoteAddr() string {
	return c.Socket.RemoteAddr().String()
}

func (c *RLPAClient) SendRLPAPacket(tag uint8, value []byte) error {
	packet := NewRLPAPacket(tag, value)
	packetData, err := packet.Pack()
	if err != nil {
		return err
	}
	_, err = c.Socket.Write(packetData)
	c.DebugLog(fmt.Sprint("Send packet: ", packetData))
	if err != nil {
		return err
	}
	return nil
}

func (c *RLPAClient) MessageBox(msg string) error {
	err := c.SendRLPAPacket(TagMessagebox, []byte(msg))
	if err != nil {
		return err
	}
	return nil
}

func (c *RLPAClient) LockAPDU() error {
	err := c.SendRLPAPacket(TagApduLock, []byte{})
	if err != nil {
		return err
	}
	return nil
}

func (c *RLPAClient) UnlockAPDU() error {
	err := c.SendRLPAPacket(TagApduUnlock, []byte{})
	if err != nil {
		return err
	}
	return nil
}

func (c *RLPAClient) ProcessPacket() error {
	if c.Packet.Tag == TagApdu {
		jsonData, err := json.Marshal(
			map[string]interface{}{
				"type": "apdu",
				"payload": map[string]interface{}{
					"ecode": 0,
					"data":  hex.EncodeToString(c.Packet.Value),
				},
			})
		if err != nil {
			return err
		}
		err = c.WriteLpacStdin(jsonData)
		if err != nil {
			return err
		}
		c.DebugLogWriteLpacStdin(jsonData)

		return nil
	}

	if c.WorkMode != WaitingMode {
		return nil
	}

	switch c.Packet.Tag {
	case TagManagement:
		// TODO: shellMode
		c.WorkMode = ShellMode
		passwd := c.GenCredential()
		err := c.MessageBox(fmt.Sprintf("ManageID: %s\nPassword: %s", c.ID, passwd))
		if err != nil {
			c.ErrLog("Failed to send manageID and password")
			return err
		}
		c.InfoLog("Enter ShellMode")
		break
	case TagProcessNotification:
		// TODO: processNotificationMode
		c.WorkMode = ProcessNotificationMode
		c.InfoLog("Enter Process Notification Mode")
		err := c.MessageBox("Unimplemented command.")
		if err != nil {
			return err
		}
		c.ErrLog("process notification mode unimplemented")
		return errors.New("unimplemented command")
		break
	case TagDownloadProfile:
		// TODO: downloadProfileMode
		c.WorkMode = TagDownloadProfile
		c.InfoLog("Enter Download Profile Mode")
		err := c.MessageBox("Unimplemented command.")
		if err != nil {
			return err
		}
		c.ErrLog("download profile mode unimplemented")
		return errors.New("unimplemented command")
		break
	default:
		err := c.MessageBox("Unimplemented command.")
		if err != nil {
			return err
		}
		c.ErrLog("unimplemented mode")
		return errors.New("unimplemented command")
		break
	}
	return nil
}

func (c *RLPAClient) Close(result int) {
	if c.IsClosing {
		return
	}
	c.IsClosing = true
	if c.CMD != nil {
		if c.CMD.ProcessState == nil {
			err := c.CMD.Process.Kill()
			if err != nil {
				c.ErrLog("Failed to kill lpac process")
			}
		}
	}

	delete(Credentials, c.ID)
	if c.ResponseWaiting {
		switch result {
		case ResultFinished:
			c.ResponseChan <- []byte("OK")
			break
		case ResultClientDisconnect:
			c.ResponseChan <- []byte("client disconnected")
			break
		case ResultError:
			c.ResponseChan <- []byte("lpac error")
			break
		}
	}

	err := c.UnlockAPDU()
	if err != nil {
		c.ErrLog("Failed to unlock APDU")
	}
	packet := NewRLPAPacket(TagClose, []byte{})
	packetData, _ := packet.Pack()
	_, err2 := c.Socket.Write(packetData)
	if err2 != nil {
		c.ErrLog("Failed to send socket packet: " + string(packetData))
	}

	err3 := c.Socket.Close()
	if err3 != nil {
		c.ErrLog("Failed to close socket")
	}
	c.InfoLog("Disconnected")
	c.Socket = nil
}

func (c *RLPAClient) processOpenLpac(args []string) error {
	// TODO
	err := c.LockAPDU()
	if err != nil {
		return err
	}
	c.CMD = exec.Command(CFG.LpacPath, args...)
	c.CMD.Env = []string{
		"APDU_INTERFACE=" + CFG.APDUInterface,
		"HTTP_INTERFACE=" + CFG.HTTPInterface,
	}
	// 连接 stdio
	c.LpacStdin, err = c.CMD.StdinPipe()
	if err != nil {
		return err
	}
	c.LpacStdout, err = c.CMD.StdoutPipe()
	if err != nil {
		return err
	}
	c.LpacStderr, err = c.CMD.StderrPipe()
	if err != nil {
		return err
	}

	err = c.CMD.Start()
	if err != nil {
		return err
	}
	go func() {
		_ = c.CMD.Wait()
		if !c.IsClosing {
			err := c.UnlockAPDU()
			if err != nil {
				c.Close(ResultError)
			}
		}
	}()
	// stderr 和 stdout 处理函数如果遇到结果将发送给 responseChan
	go func() {
		errStdout := c.OnLpacStdout()
		if errStdout != nil {
			c.Close(ResultError)
		}
	}()
	go c.OnLpacStderr()
	return nil
}

func (c *RLPAClient) OnLpacStdout() error {
	scanner := bufio.NewScanner(c.LpacStdout)
	// 当 lpac 进程结束，管道会写入 EOF 自动关闭，函数退出
	for scanner.Scan() {
		line := scanner.Bytes()
		c.DebugLogReadLpacStdout(line)
		var req Request
		err := json.Unmarshal(line, &req)
		if err != nil {
			return err
		}
		switch req.Type {
		case "apdu":
			switch req.Payload.Func {
			case "connect":
				jsonData, errMarshal := json.Marshal(map[string]interface{}{
					"type": "apdu",
					"payload": map[string]interface{}{
						"ecode": 0,
					},
				})
				if errMarshal != nil {
					return errMarshal
				}
				errWrite := c.WriteLpacStdin(jsonData)
				if errWrite != nil {
					return errWrite
				}
				c.DebugLogWriteLpacStdin(jsonData)
			case "logic_channel_open":
				jsonData, errMarshal := json.Marshal(map[string]interface{}{
					"type": "apdu",
					"payload": map[string]interface{}{
						"ecode": 0,
					},
				})
				if errMarshal != nil {
					return errMarshal
				}
				errWrite := c.WriteLpacStdin(jsonData)
				if errWrite != nil {
					return errWrite
				}
				c.DebugLogWriteLpacStdin(jsonData)
			case "transmit":
				hexBytes, errHexDecode := hex.DecodeString(req.Payload.Param)
				if errHexDecode != nil {
					return errHexDecode
				}
				errSendPacket := c.SendRLPAPacket(TagApdu, hexBytes)
				if errHexDecode != nil {
					return errSendPacket
				}
			}
		case "lpa":
			c.DebugLog("run lpac finished")
			// 完成，发送结果 json
			if c.ResponseWaiting {
				c.ResponseChan <- line
			}
			return nil
		default:
			return errors.New("undefined output struct")
		}
	}
	return nil
}

func (c *RLPAClient) OnLpacStderr() {
	scanner := bufio.NewScanner(c.LpacStderr)
	for scanner.Scan() {
		line := scanner.Bytes()
		if c.ResponseWaiting {
			c.ResponseChan <- line
		}
		c.Close(ResultError)
		return
	}
}

func (c *RLPAClient) WriteLpacStdin(data []byte) error {
	if c.LpacStdin == nil {
		return nil
	}
	_, err := c.LpacStdin.Write(append(data, '\n'))
	if err != nil {
		return err
	}
	return nil
}

func (c *RLPAClient) StartOrResetTimer() {
	if c.KeepAliveTimer == nil {
		c.KeepAliveTimer = time.AfterFunc(keepaliveDuration, func() {
			c.DisconnectAPI()
		})
	} else {
		c.KeepAliveTimer.Reset(keepaliveDuration)
	}
}

func (c *RLPAClient) DisconnectAPI() {
	// TODO
	if c.ResponseWaiting {
		c.ResponseChan <- []byte("timeout")
	}
	c.APILocked = false
}

func (c *RLPAClient) DebugLog(msg string) {
	slog.Debug(msg, "client", c.RemoteAddr())
}

func (c *RLPAClient) DebugLogWriteLpacStdin(data []byte) {
	slog.Debug("Write lpac stdin "+string(data), "client", c.RemoteAddr())
}

func (c *RLPAClient) DebugLogReadLpacStdout(data []byte) {
	slog.Debug("Read lpac stdin "+string(data), "client", c.RemoteAddr())
}

func (c *RLPAClient) InfoLog(msg string) {
	slog.Info(msg, "client", c.RemoteAddr())
}

func (c *RLPAClient) ErrLog(msg string) {
	slog.Error(msg, "client", c.RemoteAddr())
}
