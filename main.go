package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"strings"
)

func init() {
	Credentials = make(map[string]string)
}

func main() {
	debug := flag.Bool("debug", false, "sets log level to debug")
	showHelp := flag.Bool("help", false, "show help info")
	flag.Parse()
	if *showHelp {
		help := `rlpa-server

Arguments:
	-debug	enable debug output
	-help	show help info

Environment Variables:
	LPAC_FOLDER	lpac binary folder name
	SOCKET_PORT	rlpa socket port
	API_PORT	http management api port
`
		print(help)
		return
	}
	slog.SetLogLoggerLevel(slog.LevelInfo)
	if *debug {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	err := InitConfig()
	if err != nil {
		panic(err)
	}

	go HttpServer()

	addr := fmt.Sprint("0.0.0.0:", CFG.SocketPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}

	slog.Info("Start listening on tcp://" + addr)
	defer func(listener net.Listener) {
		errClose := listener.Close()
		if errClose != nil {
			slog.Error("Failed to close socket listener")
		}
	}(listener)

	for {
		conn, err := listener.Accept()
		slog.Info("Accepted " + conn.RemoteAddr().String())
		if err != nil {
			slog.Error(err.Error())
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	client := NewRLPAClient(conn)

	for {
		// 接受 Packet
		err := client.Packet.Recv(conn)
		if err != nil {
			if strings.Contains(err.Error(), "EOF") {
				client.Close(ResultClientDisconnect)
			} else {
				slog.Error("packet recv: "+err.Error(), "client", client.RemoteAddr())
				client.Close(ResultError)
			}
			return
		}
		if !client.Packet.IsFinished() {
			continue
		}
		// 处理
		err = client.ProcessPacket()
		if err != nil {
			slog.Error("packet process: "+err.Error(), "client", client.RemoteAddr())
			client.Close(ResultError)
			return
		}
		// 重置
		client.Packet = NewRLPAPacket(0, []byte{})
	}
}
