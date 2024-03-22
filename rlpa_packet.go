package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
)

const (
	TagMessagebox          = 0x00
	TagManagement          = 0x01
	TagDownloadProfile     = 0x02
	TagProcessNotification = 0x03

	TagReboot     = 0xFB
	TagClose      = 0xFC
	TagApduLock   = 0xFD
	TagApdu       = 0xFE
	TagApduUnlock = 0xFF
)

type RLPAPacket struct {
	Tag         uint8
	Value       []byte
	State       int
	NextReadLen uint16
	Buffer      bytes.Buffer
}

func NewRLPAPacket(tag uint8, value []byte) RLPAPacket {
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
	buf.Write(p.Value)

	return buf.Bytes(), nil
}

func (p *RLPAPacket) Recv(conn net.Conn) error {
	// 如果已经结束，不再处理
	if p.IsFinished() {
		return nil
	}
	// 读取至 nextReadLen 长度
	readExactly := func(conn net.Conn, nextReadLen int) ([]byte, error) {
		buffer := make([]byte, nextReadLen)
		totalRead := 0

		for totalRead < nextReadLen {
			n, err := conn.Read(buffer[totalRead:])
			if err != nil {
				if err == io.EOF {
					// 对面关闭连接，读到 EOF
					return buffer[:totalRead], io.ErrUnexpectedEOF
				}
				return buffer[:totalRead], err
			}
			totalRead += n
			slog.Debug(fmt.Sprint("Socket Read buffer: ", buffer[:totalRead]), "client", conn.RemoteAddr().String())
		}
		return buffer, nil
	}
	buf, err := readExactly(conn, int(p.NextReadLen))

	if err != nil {
		return err
	}

	// 长度为 0 说明读取失败
	if len(buf) == 0 {
		return errors.New("read buffer length 0")
	}
	// 写入
	p.NextReadLen -= uint16(len(buf))
	p.Buffer.Write(buf)

	unpackTag := func(reader io.Reader) (uint8, error) {
		// unpack('C', $this->_buffer)[1]
		var tag uint8
		// 无符号字符不区分大端序小端序
		// 读取第一个字节（Tag）
		errBinaryRead := binary.Read(reader, binary.LittleEndian, &tag)
		if errBinaryRead != nil {
			return 0, errBinaryRead
		}
		return tag, nil
	}

	unpackNextReadLen := func(reader io.Reader) (uint16, error) {
		// unpack('v', $this->_buffer)[1]
		// 'v' 为 16 位无符号小端序整数
		var nextReadLen uint16
		errBinaryRead := binary.Read(reader, binary.LittleEndian, &nextReadLen)
		if errBinaryRead != nil {
			return 0, errBinaryRead
		}
		return nextReadLen, nil
	}

	// State 0 初始阶段，读 tag
	// State 1 读长度
	// State 2 读数据
	// State 3 结束

	if p.NextReadLen == 0 {
		switch p.State {
		case 0:
			p.Tag, err = unpackTag(&p.Buffer)
			if err != nil {
				return err
			}
			slog.Debug(fmt.Sprint("Unpacked Tag: ", p.Tag), "client", conn.RemoteAddr().String())
			p.State = 1
			slog.Debug(fmt.Sprint("Switch to STATE: ", p.State), "client", conn.RemoteAddr().String())
			p.NextReadLen = 2
			p.Buffer.Reset()
			break
		case 1:
			p.State = 2
			slog.Debug(fmt.Sprint("Switch to STATE: ", p.State), "client", conn.RemoteAddr().String())
			p.NextReadLen, err = unpackNextReadLen(&p.Buffer)
			if err != nil {
				return err
			}
			slog.Debug(fmt.Sprint("Unpacked nextReadLen: ", p.NextReadLen), "client", conn.RemoteAddr().String())
			// NextReadLen 太长
			if p.NextReadLen >= (512 - 3) {
				return errors.New("next read len too long (>= 519)")
			}
			p.Buffer.Reset()
			// 剩余读取长度为 0，进入 State 3 结束
			if p.NextReadLen == 0 {
				p.State = 3
				p.Value = []byte{}
			}
			break
		case 2:
			p.State = 3
			p.Value = p.Buffer.Bytes()
			p.Buffer.Reset()
			break
		}
	}
	return nil
}
