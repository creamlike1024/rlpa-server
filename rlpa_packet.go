package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/rs/zerolog/log"
	"io"
	"net"
)

const (
	TAG_MESSAGEBOX           = 0x00
	TAG_MANAGEMNT            = 0x01
	TAG_DOWNLOAD_PROFILE     = 0x02
	TAG_PROCESS_NOTIFICATION = 0x03

	TAG_REBOOT      = 0xFB
	TAG_CLOSE       = 0xFC
	TAG_APDU_LOCK   = 0xFD
	TAG_APDU        = 0xFE
	TAG_APDU_UNLOCK = 0xFF
)

type RLPAPacket struct {
	Tag         uint8
	Value       string
	State       int
	NextReadLen uint16
	Buffer      bytes.Buffer
}

func NewRLPAPacket(tag uint8, value string) RLPAPacket {
	return RLPAPacket{
		Tag:         tag,
		Value:       value,
		State:       0,
		NextReadLen: 1,
		Buffer:      bytes.Buffer{},
	}
}

func (p *RLPAPacket) IsFinished() bool {
	return p.State == 3
}

func (p *RLPAPacket) Pack() ([]byte, error) {
	buf := new(bytes.Buffer)

	// 写入 tag，对应 php 的 pack('C', $this->tag)
	err := binary.Write(buf, binary.LittleEndian, p.Tag)
	if err != nil {
		return nil, err
	}

	// 写入 value 长度，对应 php 的 pack('v', strlen($this->value));
	valLen := uint16(len(p.Value))
	err = binary.Write(buf, binary.LittleEndian, valLen)
	if err != nil {
		return nil, err
	}

	// 写入value内容
	buf.WriteString(p.Value)

	return buf.Bytes(), nil
}

func (p *RLPAPacket) Recv(conn net.Conn) error {
	// 如果已经结束，不再处理
	if p.IsFinished() {
		return nil
	}
	// 设定 buf 长度即可读取指定长度
	buf := make([]byte, p.NextReadLen)
	conn.Read(buf)
	log.Debug().Str("client", conn.RemoteAddr().String()).Msg("Socket Read buffer: " + string(buf))
	// 长度为 0 说明读取失败
	if len(buf) == 0 {
		return errors.New("read buffer length 0")
	}
	// 写入
	p.NextReadLen -= uint16(len(buf))
	p.Buffer.Write(buf)

	unpackTag := func(reader io.Reader) uint8 {
		// unpack('C', $this->_buffer)[1]
		var tag uint8
		// 无符号字符不区分大端序小端序
		// 读取第一个字节（Tag）
		binary.Read(reader, binary.LittleEndian, &tag)
		return tag
	}

	unpackNextReadLen := func(reader io.Reader) uint16 {
		// unpack('v', $this->_buffer)[1]
		// 'v' 为 16 位无符号小端序整数
		var nextReadLen uint16
		binary.Read(reader, binary.LittleEndian, &nextReadLen)
		return nextReadLen
	}

	// State 0 初始阶段，读 tag
	// State 1 读长度
	// State 2 读数据
	// State 3 结束

	if p.NextReadLen == 0 {
		switch p.State {
		case 0:
			p.Tag = unpackTag(&p.Buffer)
			log.Debug().Str("client", conn.RemoteAddr().String()).Uint8("Tag", p.Tag).Msgf("Unpacked Tag: %d", p.Tag)
			p.State = 1
			log.Debug().Str("client", conn.RemoteAddr().String()).Int("State", p.State).Msgf("Switch to STATE: %d", p.State)
			p.NextReadLen = 2
			p.Buffer.Reset()
			break
		case 1:
			p.State = 2
			log.Debug().Str("client", conn.RemoteAddr().String()).Int("State", p.State).Msgf("Switch to STATE: %d", p.State)
			p.NextReadLen = unpackNextReadLen(&p.Buffer)
			log.Debug().Str("client", conn.RemoteAddr().String()).Uint16("nextReadLen", p.NextReadLen).Msgf("Unpacked nextReadLen: %d", p.NextReadLen)
			// NextReadLen 太长
			if p.NextReadLen >= (512 - 3) {
				return errors.New("next read len too long (>= 519)")
			}
			p.Buffer.Reset()
			// 剩余读取长度为 0，进入 State 3 结束
			if p.NextReadLen == 0 {
				p.State = 3
				p.Value = ""
			}
			break
		case 2:
			p.State = 3
			p.Value = p.Buffer.String()
			p.Buffer.Reset()
			break
		}
	}
	return nil
}
