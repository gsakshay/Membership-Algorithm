package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
)

const (
	// Message Types
	JOIN      = 1
	REQ       = 2
	OK        = 3
	NEWVIEW   = 4
	NEWLEADER = 5
	// Flags for encoding and decoding Messages - iota is a built-in identifier in Go that is used to create constant sequences
	FLAG_OPERATION_TYPE  = 1 << iota // 1 << 0 = 00000001 = 1
	FLAG_PEER_ID                     // 1 << 1 = 00000010 = 2
	FLAG_VIEW_ID                     // 1 << 2 = 00000100 = 4
	FLAG_REQUEST_ID                  // 1 << 3 = 00001000 = 8
	FLAG_MEMBERSHIP_LIST             // 1 << 4 = 0010000 = 16
)

type MessageHeader struct {
	SenderID    int32
	MessageType int32
	PayloadSize int32
}

type Message struct {
	Header  MessageHeader
	Payload []byte
}

func SendMessage(conn net.Conn, senderID int, messageType int, membMsg *MembershipMessage) error {
	if conn == nil {
		return errors.New("connection is nil")
	}

	payload, err := encodeMembershipMessage(membMsg)
	if err != nil {
		return fmt.Errorf("error encoding membership message: %v", err)
	}

	header := MessageHeader{
		SenderID:    int32(senderID),
		MessageType: int32(messageType),
		PayloadSize: int32(len(payload)),
	}

	err = binary.Write(conn, binary.BigEndian, &header)
	if err != nil {
		return fmt.Errorf("error writing header: %v", err)
	}

	_, err = conn.Write(payload)
	if err != nil {
		return fmt.Errorf("error writing payload: %v", err)
	}

	return nil
}

func ReadMessage(conn net.Conn) (*Message, error) {
	if conn == nil {
		return nil, errors.New("connection is nil")
	}

	var header MessageHeader
	err := binary.Read(conn, binary.BigEndian, &header)
	if err != nil {
		return nil, fmt.Errorf("error reading header: %v", err)
	}

	payload := make([]byte, header.PayloadSize)
	_, err = io.ReadFull(conn, payload)
	if err != nil {
		return nil, fmt.Errorf("error reading payload: %v", err)
	}

	return &Message{Header: header, Payload: payload}, nil
}

func HandleConnection(conn net.Conn, receiveCh chan<- *Message) {
	defer conn.Close()

	for {
		msg, err := ReadMessage(conn)
		if err != nil {
			if err != io.EOF {
				// fmt.Printf("Error reading message: %v\n", err)
			}
			return
		}

		receiveCh <- msg
	}
}

func encodeMembershipMessage(membMsg *MembershipMessage) ([]byte, error) {
	if membMsg == nil {
		return nil, errors.New("membership message is nil")
	}

	buf := make([]byte, 0, 1024) // Initial capacity, will grow if needed
	flags := uint16(0)

	// Determine which fields are present and set flags
	if membMsg.OperationType != "" {
		flags |= FLAG_OPERATION_TYPE
	}
	if membMsg.PeerId != 0 {
		flags |= FLAG_PEER_ID
	}
	if membMsg.ViewId != 0 {
		flags |= FLAG_VIEW_ID
	}
	if membMsg.RequestId != 0 {
		flags |= FLAG_REQUEST_ID
	}
	if len(membMsg.MembershipList) > 0 {
		flags |= FLAG_MEMBERSHIP_LIST
	}

	// Write flags
	tempBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(tempBuf, flags)
	buf = append(buf, tempBuf...)

	// Encode fields based on flags
	if flags&FLAG_OPERATION_TYPE != 0 {
		opTypeBytes := []byte(membMsg.OperationType)
		tempBuf = make([]byte, 2)
		binary.BigEndian.PutUint16(tempBuf, uint16(len(opTypeBytes)))
		buf = append(buf, tempBuf...)
		buf = append(buf, opTypeBytes...)
	}

	if flags&FLAG_PEER_ID != 0 {
		tempBuf = make([]byte, 4)
		binary.BigEndian.PutUint32(tempBuf, uint32(membMsg.PeerId))
		buf = append(buf, tempBuf...)
	}

	if flags&FLAG_VIEW_ID != 0 {
		tempBuf = make([]byte, 4)
		binary.BigEndian.PutUint32(tempBuf, uint32(membMsg.ViewId))
		buf = append(buf, tempBuf...)
	}

	if flags&FLAG_REQUEST_ID != 0 {
		tempBuf = make([]byte, 4)
		binary.BigEndian.PutUint32(tempBuf, uint32(membMsg.RequestId))
		buf = append(buf, tempBuf...)
	}

	if flags&FLAG_MEMBERSHIP_LIST != 0 {
		// Encode the length of the MembershipList
		tempBuf = make([]byte, 4)
		binary.BigEndian.PutUint32(tempBuf, uint32(len(membMsg.MembershipList)))
		buf = append(buf, tempBuf...)

		// Encode each member ID as uint32
		for _, peerId := range membMsg.MembershipList {
			tempBuf = make([]byte, 4)
			binary.BigEndian.PutUint32(tempBuf, uint32(peerId))
			buf = append(buf, tempBuf...)
		}
	}

	return buf, nil
}

func decodeMembershipMessage(payload []byte) (*MembershipMessage, error) {
	if len(payload) < 2 {
		return nil, errors.New("payload too short")
	}

	membMsg := &MembershipMessage{}
	offset := 0

	// Read flags
	if offset+2 > len(payload) {
		return nil, errors.New("payload too short for flags")
	}
	flags := binary.BigEndian.Uint16(payload[offset:])
	offset += 2

	// Decode fields based on flags
	if flags&FLAG_OPERATION_TYPE != 0 {
		if offset+2 > len(payload) {
			return nil, errors.New("payload too short for operation type length")
		}
		opTypeLen := int(binary.BigEndian.Uint16(payload[offset:]))
		offset += 2
		if offset+opTypeLen > len(payload) {
			return nil, errors.New("payload too short for operation type")
		}
		membMsg.OperationType = OperationType(payload[offset : offset+opTypeLen])
		offset += opTypeLen
	}

	if flags&FLAG_PEER_ID != 0 {
		if offset+4 > len(payload) {
			return nil, errors.New("payload too short for peer ID")
		}
		membMsg.PeerId = int(binary.BigEndian.Uint32(payload[offset:]))
		offset += 4
	}

	if flags&FLAG_VIEW_ID != 0 {
		if offset+4 > len(payload) {
			return nil, errors.New("payload too short for view ID")
		}
		membMsg.ViewId = int(binary.BigEndian.Uint32(payload[offset:]))
		offset += 4
	}

	if flags&FLAG_REQUEST_ID != 0 {
		if offset+4 > len(payload) {
			return nil, errors.New("payload too short for request ID")
		}
		membMsg.RequestId = int(binary.BigEndian.Uint32(payload[offset:]))
		offset += 4
	}

	if flags&FLAG_MEMBERSHIP_LIST != 0 {
		if offset+4 > len(payload) {
			return nil, errors.New("payload too short for membership list length")
		}

		// Read the length of the MembershipList
		listLen := int(binary.BigEndian.Uint32(payload[offset:]))
		offset += 4

		// Allocate space for MembershipList
		membMsg.MembershipList = make([]int, listLen)

		// Read each member ID
		for i := 0; i < listLen; i++ {
			if offset+4 > len(payload) {
				return nil, errors.New("payload too short for membership list member ID")
			}
			membMsg.MembershipList[i] = int(binary.BigEndian.Uint32(payload[offset:]))
			offset += 4
		}
	}

	return membMsg, nil
}

func encodePeer(buf []byte, peer *Peer) []byte {
	// Encode Peer ID
	tempBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(tempBuf, uint32(peer.ID))
	buf = append(buf, tempBuf...)

	// Encode Hostname
	hostnameBytes := []byte(peer.Hostname)
	tempBuf = make([]byte, 2)
	binary.BigEndian.PutUint16(tempBuf, uint16(len(hostnameBytes)))
	buf = append(buf, tempBuf...)
	buf = append(buf, hostnameBytes...)

	// Encode Address
	addressBytes := []byte(peer.Address)
	tempBuf = make([]byte, 2)
	binary.BigEndian.PutUint16(tempBuf, uint16(len(addressBytes)))
	buf = append(buf, tempBuf...)
	buf = append(buf, addressBytes...)

	return buf
}

func decodePeer(payload []byte, offset int, peer *Peer) (int, error) {
	if offset+4 > len(payload) {
		return 0, errors.New("payload too short for peer ID")
	}
	peer.ID = int(binary.BigEndian.Uint32(payload[offset:]))
	offset += 4

	// Decode Hostname
	if offset+2 > len(payload) {
		return 0, errors.New("payload too short for hostname length")
	}
	hostnameLen := int(binary.BigEndian.Uint16(payload[offset:]))
	offset += 2
	if offset+hostnameLen > len(payload) {
		return 0, errors.New("payload too short for hostname")
	}
	peer.Hostname = string(payload[offset : offset+hostnameLen])
	offset += hostnameLen

	// Decode Address
	if offset+2 > len(payload) {
		return 0, errors.New("payload too short for address length")
	}
	addressLen := int(binary.BigEndian.Uint16(payload[offset:]))
	offset += 2
	if offset+addressLen > len(payload) {
		return 0, errors.New("payload too short for address")
	}
	peer.Address = string(payload[offset : offset+addressLen])
	offset += addressLen

	return offset, nil
}

func SendJoin(conn net.Conn, senderID int, membMsg *MembershipMessage) error {
	println("Sending JOIN from ", senderID)
	return SendMessage(conn, senderID, JOIN, membMsg)
}

func SendReq(conn net.Conn, senderID int, membMsg *MembershipMessage) error {
	println("Sending REQ from ", senderID, " with req ID ", membMsg.RequestId)
	return SendMessage(conn, senderID, REQ, membMsg)
}

func SendOk(conn net.Conn, senderID int, membMsg *MembershipMessage) error {
	println("Sending OK from ", senderID, " with req ID ", membMsg.RequestId)
	return SendMessage(conn, senderID, OK, membMsg)
}

func SendNewView(conn net.Conn, senderID int, membMsg *MembershipMessage) error {
	println("Sending NEWVIEW from ", senderID, " with view ", membMsg.ViewId)
	return SendMessage(conn, senderID, NEWVIEW, membMsg)
}

func SendNewLeader(conn net.Conn, senderID int, membMsg *MembershipMessage) error {
	println("Sending NEWLEADER from ", senderID)
	return SendMessage(conn, senderID, NEWLEADER, membMsg)
}
