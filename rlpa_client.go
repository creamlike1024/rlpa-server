package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/rs/zerolog/log"
	"net"
	"os/exec"
)

type RLPAClient struct {
	Socket     net.Conn
	Packet     RLPAPacket
	CMD        *exec.Cmd
	LpacStdin  bytes.Buffer
	LpacStdout bytes.Buffer
	LpacStderr bytes.Buffer
}

func (c *RLPAClient) RemoteAddr() string {
	return c.Socket.RemoteAddr().String()
}

func (c *RLPAClient) SendRLPAPacket(tag uint8, value string) error {
	packet := NewRLPAPacket(tag, value)
	packetData, err := packet.Pack()
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}
	_, err = c.Socket.Write(packetData)
	log.Debug().Str("client", c.RemoteAddr()).Msgf("Send packet: %s", string(packetData))
	if err != nil {
		return err
	}
	return nil
}

func (c *RLPAClient) MessageBox(msg string) error {
	err := c.SendRLPAPacket(TAG_MESSAGEBOX, msg)
	if err != nil {
		return err
	}
	return nil
}

func (c *RLPAClient) LockAPDU() error {
	err := c.SendRLPAPacket(TAG_APDU_LOCK, "")
	if err != nil {
		return err
	}
	return nil
}

func (c *RLPAClient) UnlockAPDU() error {
	err := c.SendRLPAPacket(TAG_APDU_UNLOCK, "")
	if err != nil {
		return err
	}
	return nil
}

func (c *RLPAClient) ProcessPacket() error {
	if c.Packet.Tag == TAG_APDU {
		jsonData, _ := json.Marshal(
			map[string]interface{}{
				"type": "apdu",
				"payload": map[string]interface{}{
					"ecode": 0,
					"data":  hex.EncodeToString([]byte(c.Packet.Value)),
				},
			})
		c.LpacStdin.WriteString(string(jsonData) + "\n")
		return nil
	}

	switch c.Packet.Tag {
	case TAG_MANAGEMNT:
		// TODO: shellMode
		log.Info().Str("client", c.RemoteAddr()).Msg("Enter ShellMode")
		c.MessageBox("Unimplemented command.")
		log.Error().Str("client", c.RemoteAddr()).Msg("shell mode unimplemented")
		return errors.New("Unimplemented command.")
		break
	case TAG_PROCESS_NOTIFICATION:
		// TODO: processNotificationMode
		log.Info().Str("client", c.RemoteAddr()).Msg("Enter Process Notification Mode")
		c.MessageBox("Unimplemented command.")
		log.Error().Str("client", c.RemoteAddr()).Msg("process notification mode unimplemented")
		return errors.New("Unimplemented command.")
		break
	case TAG_DOWNLOAD_PROFILE:
		// TODO: downloadProfileMode
		log.Info().Str("client", c.RemoteAddr()).Msg("Enter Download Profile Mode")
		c.MessageBox("Unimplemented command.")
		log.Error().Str("client", c.RemoteAddr()).Msg("download profile mode unimplemented")
		return errors.New("Unimplemented command.")
		break
	default:
		c.MessageBox("Unimplemented command.")
		log.Error().Str("client", c.RemoteAddr()).Msg("unimplemented mode")
		return errors.New("Unimplemented command.")
		break
	}
	return nil
}

func (c *RLPAClient) OnSocketRecv() {
	c.Packet.Recv(c.Socket)
	if !c.Packet.IsFinished() {
		return
	}
	c.ProcessPacket()
	c.Packet = NewRLPAPacket(0, "")
}

func (c *RLPAClient) Close() {
	c.UnlockAPDU()

	packet := NewRLPAPacket(TAG_CLOSE, "")
	packetData, _ := packet.Pack()
	c.Socket.Write(packetData)
	c.Socket.Close()
	log.Info().Str("client", c.RemoteAddr()).Msg("Disconnected")
	c.Socket = nil

	// c.processClose()
}

func (c *RLPAClient) processOpenLpac(args string) {
	// TODO
	// 先锁定 APDU，然后启动 lpac 并连接 stdio
	c.LockAPDU()
}

func (c *RLPAClient) processClose() {
	// TODO
	c.CMD.Process.Kill()
	c.UnlockAPDU()
}

func NewRLPAClient(conn net.Conn) RLPAClient {
	return RLPAClient{
		Socket: conn,
		Packet: NewRLPAPacket(0x00, ""),
	}
}
