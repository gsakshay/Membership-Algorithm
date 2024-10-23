package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
)

const (
	HEARTBEAT = 1
)

type HeartbeatMessage struct {
	SenderID int32
}

func SendHeartbeat(senderID int, peer Peer) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%v", peer.Hostname, config.UDPPort))
	if err != nil {
		// log.Printf("Error resolving address for %s: %v", peer.Hostname, err)
		return err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		// log.Printf("Error dialing %s: %v", peer.Hostname, err)
		return err
	}
	defer conn.Close()

	msg := HeartbeatMessage{
		SenderID: int32(senderID),
	}

	buf := make([]byte, 5) // 1 byte for type, 4 bytes for SenderID
	buf[0] = HEARTBEAT
	binary.BigEndian.PutUint32(buf[1:5], uint32(msg.SenderID))

	_, err = conn.Write(buf)
	if err != nil {
		// log.Printf("Error sending to %s: %v", peer.Hostname, err)
		return err
	}

	return nil
}

func ReceiveHeartbeats(receiveCh chan<- HeartbeatMessage) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%v", config.UDPPort))
	if err != nil {
		log.Fatalf("Error resolving UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatalf("Error listening: %v", err)
	}
	defer conn.Close()

	for {
		buf := make([]byte, 5)
		n, _, err := conn.ReadFromUDP(buf)

		if err != nil {
			println("Error reading UDP message: %v\n", err)
			continue
		}

		if n != 5 || buf[0] != HEARTBEAT {
			println("Received invalid message")
			continue
		}

		msg := HeartbeatMessage{
			SenderID: int32(binary.BigEndian.Uint32(buf[1:5])),
		}

		receiveCh <- msg
	}
}
