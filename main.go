package main

import (
	"flag"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net"
)

func main() {
	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	const addr = "0.0.0.0:1888"
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal().Err(err)
	}

	log.Info().Msg("Start listening on tcp://" + addr)
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		log.Info().Msgf("Accepted %s", conn.RemoteAddr().String())
		if err != nil {
			log.Err(err)
			continue
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	client := NewRLPAClient(conn)

	client.MessageBox("Welcome, " + client.Socket.RemoteAddr().String())

	for {
		// 接受 Packet
		err := client.Packet.Recv(conn)
		if err != nil {
			log.Err(err).Str("client", client.Socket.RemoteAddr().String())
			client.Close()
			return
		}
		if !client.Packet.IsFinished() {
			continue
		}
		// 处理
		err = client.ProcessPacket()
		if err != nil {
			log.Err(err).Str("client", client.Socket.RemoteAddr().String())
			client.Close()
			return
		}
		// 重置
		client.Packet = NewRLPAPacket(0, "")
	}
}
