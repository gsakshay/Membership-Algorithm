package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

type Config struct {
	TCPPort             string
	UDPPort             string
	StarterDelay        float64
	crashAfterJoinDelay float64
	CustomTestcase      bool
	HostsFile           string
	Hostname            string
	HeartbeatTimeout    time.Duration
}

var config Config

func parseFlagsAndAssignConstants() {
	flag.StringVar(&config.HostsFile, "h", "", "Path to hosts file")
	flag.Float64Var(&config.StarterDelay, "d", 0.0, "Starter Delay")
	flag.Float64Var(&config.crashAfterJoinDelay, "c", 0.0, "Delay to wait for before crashing once join msg sent")
	flag.BoolVar(&config.CustomTestcase, "t", false, "Execute custom test case")
	flag.Parse()

	config.TCPPort = "8888"
	config.UDPPort = "9999"
	config.HeartbeatTimeout = 1 * time.Second

	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Error getting hostname: %v", err)
	}

	config.Hostname = hostname
}

func readHostsFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var peers []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			peers = append(peers, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return peers, nil
}

func initializePeerManager() *PeerManager {
	pm := NewPeerManager()
	peers, err := readHostsFile(config.HostsFile)

	if err != nil {
		log.Fatalf("error reading hosts file: %v", err)
	}

	for i, hostname := range peers {
		pm.KnowPeer(i+1, hostname)
		if hostname == config.Hostname {
			pm.SetSelf(i + 1)
		} else {
			pm.AddPeer(i + 1)
		}
	}

	// Assume the first one as leader
	pm.SetLeader(1)

	selfIndex := pm.GetSelfID()
	if selfIndex == 0 {
		log.Fatalf("hostname not found in hosts file")
	}

	return pm
}

func getReady(readyCh chan bool) {
	if config.StarterDelay != 0.0 {
		time.Sleep(time.Duration(config.StarterDelay * float64(time.Second)))
		readyCh <- true
	} else {
		readyCh <- true
	}
}

func establishConnections(peerManager *PeerManager, connectionsToBeEstablishedCh chan *Peer, connectionEstablishedCh chan bool) {
	for {
		select {
		case peer := <-connectionsToBeEstablishedCh:
			if peer.Conn == nil {
				conn, err := net.Dial("tcp", peer.Address)
				connErr := peerManager.SetConnection(peer.ID, conn)
				if err != nil || connErr != nil {
					println("Failed to establish connection")
				} else {
					connectionEstablishedCh <- true
				}
			}
		}
	}
}

func handleJoinRequests(incomingChannel chan *Message, informCompletedChannel chan bool, stateManager *StateManager, peerManager *PeerManager) {
	for {
		select {
		case msg := <-incomingChannel:
			currentState := stateManager.GetCurrentState()
			if len(currentState.MemberList) == 1 {
				// This is the first member and hence proceed with adding
				stateManager.UpdateView(&MembershipMessage{
					ViewId:        stateManager.GetViewId() + 1,
					PeerId:        int(msg.Header.SenderID),
					OperationType: ADD,
				})

				// Send NEWVIEW Message
				if peer, ok := peerManager.GetPeer(int(msg.Header.SenderID)); ok {
					SendNewView(peer.Conn, peerManager.GetSelfID(), &MembershipMessage{
						ViewId:         stateManager.GetViewId(),
						MembershipList: stateManager.GetMembers(),
						OperationType:  ADD,
					})
					// Start sending heartbeats
					// peer.SendHeartbeatCh <- true
				}
				println("{peer_id: ", peerManager.GetSelfID(), ", view_id: ", stateManager.GetViewId(), ", leader: ", peerManager.GetLeader(), ", memb_list: ", arrayToString(stateManager.GetMembers()), "}")
			} else {
				// Send request message
				requestId := stateManager.GetNextRequestId()
				for _, peerId := range stateManager.GetMembers() {
					if peer, ok := peerManager.GetPeer(peerId); ok && peerId != peerManager.GetSelfID() {
						newRequestMessage := &MembershipMessage{
							OperationType: ADD,
							RequestId:     requestId,
							ViewId:        currentState.ViewId,
							PeerId:        int(msg.Header.SenderID),
						}
						err := SendReq(peer.Conn, peerManager.GetSelfID(), newRequestMessage)
						if err != nil {
							println("Cound not send REQ Message to peer: %v", peerId)
						} else {
							stateManager.AddRequestEntry(requestId, newRequestMessage)
						}
					}
				}
			}
			informCompletedChannel <- true
		}
	}
}

func handleNEWVIEWMessage(incomingChannel chan *Message, informCompletedChannel chan bool, stateManager *StateManager, peerManager *PeerManager) {
	for {
		select {
		case msg := <-incomingChannel:
			memberShipMessage, err := decodeMembershipMessage(msg.Payload)
			println("NEWVIEW Message details", memberShipMessage.ViewId, memberShipMessage.OperationType, memberShipMessage.PeerId, memberShipMessage.MembershipList)
			if err != nil {
				println("Error decoding Membership Message")
			}
			updateErr := stateManager.UpdateView(memberShipMessage)
			if updateErr != nil {
				println("Error updating the membership")
			} else {
				println("{peer_id: ", peerManager.GetSelfID(), ", view_id: ", stateManager.GetViewId(), ", leader: ", peerManager.GetLeader(), ", memb_list: ", arrayToString(stateManager.GetMembers()), "}")
				informCompletedChannel <- true
				if memberShipMessage.RequestId != 0 {
					stateManager.DeleteRequestEntry(memberShipMessage.RequestId)
				}
			}

		}
	}
}

func handleREQMessages(incomingChannel chan *Message, informCompletedChannel chan bool, stateManager *StateManager, peerManager *PeerManager) {
	for {
		select {
		case msg := <-incomingChannel:
			// Save the operation
			memberShipMessage, err := decodeMembershipMessage(msg.Payload)
			if err != nil {
				println("Error decoding Membership Message")
			}
			stateManager.AddRequestEntry(memberShipMessage.RequestId, memberShipMessage)
			// stateManager.SetRequestId(memberShipMessage.RequestId)
			// Send back OK message
			if leader, ok := peerManager.GetPeer(int(msg.Header.SenderID)); ok {
				SendOk(leader.Conn, peerManager.GetSelfID(), &MembershipMessage{
					RequestId: memberShipMessage.RequestId,
					ViewId:    stateManager.GetViewId(),
				})
			}
			informCompletedChannel <- true
		}
	}
}

func handleOKMessages(incomingChannel chan *Message, informCompletedChannel chan bool, stateManager *StateManager, peerManager *PeerManager) {
	for {
		select {
		case msg := <-incomingChannel:
			memberShipMessage, err := decodeMembershipMessage(msg.Payload)
			if err != nil {
				println("Error decoding Membership Message")
			}
			if stateManager.GetCurrentState().ViewId != memberShipMessage.ViewId {
				println("Peer: ", msg.Header.SenderID, " is lagging behind with viewID: ", memberShipMessage.ViewId, " whereas my viewID is ", stateManager.GetCurrentState().ViewId)
				return
			}
			currentOks := stateManager.UpdateOkEntries(memberShipMessage.RequestId)
			if retrievedRequest, ok := stateManager.GetRequestEntry(memberShipMessage.RequestId); ok {
				if retrievedRequest.Message.OperationType == ADD && currentOks == len(stateManager.GetCurrentState().MemberList)-1 {
					// Increment viewId and Membership
					stateManager.UpdateView(&MembershipMessage{
						ViewId:         retrievedRequest.Message.ViewId + 1,
						OperationType:  retrievedRequest.Message.OperationType,
						PeerId:         retrievedRequest.Message.PeerId,
						MembershipList: retrievedRequest.Message.MembershipList,
					})
					// Send back the NEWVIEW message to all the peers
					for _, peerId := range stateManager.GetMembers() {
						if peer, ok := peerManager.GetPeer(peerId); ok && peerId != peerManager.GetSelfID() {
							SendNewView(peer.Conn, peerManager.GetSelfID(), &MembershipMessage{
								ViewId:         stateManager.GetViewId(),
								MembershipList: stateManager.GetMembers(),
								RequestId:      retrievedRequest.Message.RequestId,
								OperationType:  retrievedRequest.Message.OperationType,
							})
						}
					}
					println("{peer_id: ", peerManager.GetSelfID(), ", view_id: ", stateManager.GetViewId(), ", leader: ", peerManager.GetLeader(), ", memb_list: ", arrayToString(stateManager.GetMembers()), "}")
					stateManager.DeleteRequestEntry(memberShipMessage.RequestId)
					informCompletedChannel <- true
				} else if retrievedRequest.Message.OperationType == DELETE && currentOks == len(stateManager.GetCurrentState().MemberList)-2 {
					// Increment viewId and update Membership
					stateManager.UpdateView(&MembershipMessage{
						ViewId:         retrievedRequest.Message.ViewId + 1,
						OperationType:  retrievedRequest.Message.OperationType,
						PeerId:         retrievedRequest.Message.PeerId,
						MembershipList: retrievedRequest.Message.MembershipList,
					})
					// Send back the NEWVIEW message to all the peers
					for _, peerId := range stateManager.GetMembers() {
						if peer, ok := peerManager.GetPeer(peerId); ok && peerId != peerManager.GetSelfID() && peerId != retrievedRequest.Message.PeerId {
							SendNewView(peer.Conn, peerManager.GetSelfID(), &MembershipMessage{
								ViewId:        stateManager.GetViewId(),
								PeerId:        retrievedRequest.Message.PeerId,
								RequestId:     retrievedRequest.Message.RequestId,
								OperationType: retrievedRequest.Message.OperationType,
							})
						}
					}
					println("{peer_id: ", peerManager.GetSelfID(), ", view_id: ", stateManager.GetViewId(), ", leader: ", peerManager.GetLeader(), ", memb_list: ", arrayToString(stateManager.GetMembers()), "}")
					stateManager.DeleteRequestEntry(memberShipMessage.RequestId)
					informCompletedChannel <- true
				} else {
					println("OK MISMATCH", currentOks, len(stateManager.GetCurrentState().MemberList)-1, " with req ", memberShipMessage.RequestId)
					informCompletedChannel <- true
				}
			}

		}
	}
}

func handleSendingHeartbeats(peer Peer, peerManager *PeerManager) {
	for {
		select {
		case action := <-peer.SendHeartbeatCh:
			if action {
				err := SendHeartbeat(peerManager.GetSelfID(), peer)
				if err == nil {
					// println("Error sending the HEARTBEAT Message to ", peer.ID, err.Error())
					time.Sleep(config.HeartbeatTimeout)
				}
				peer.SendHeartbeatCh <- true
			} else {
				println("Stopping the heartbeat for ", peer.ID)
			}
		}
	}
}

func handleReceivingHeartbeats(incomingChannel <-chan HeartbeatMessage, stateManager *StateManager, peerManager *PeerManager) {
	for {
		select {
		case msg := <-incomingChannel:
			stateManager.IncrementPeerStatus(int(msg.SenderID))
		}
	}
}

func handleCheckingFailures(stateManager *StateManager, peerManager *PeerManager) {
	ticker := time.NewTicker(config.HeartbeatTimeout)
	self := peerManager.GetSelfID()
	leader := peerManager.GetLeader()
	for {
		select {
		case <-ticker.C:
			allMembers := stateManager.GetMembers()
			for _, peerId := range allMembers {
				if peerId != self {
					if stateManager.DecrementPeerStatus(peerId) == 0 {
						currentState := stateManager.GetCurrentState()
						if leader == peerId {
							println("{peer_id:", self, ", view_id: ", currentState.ViewId, ", leader: ", leader, ", message:\"peer ", peerId, " (leader) unreachable\"}")
						} else {
							println("{peer_id:", self, ", view_id: ", currentState.ViewId, ", leader: ", leader, ", message:\"peer ", peerId, " unreachable\"}")
						}
						// Initiate deletion of peer from membership
						if self == leader {
							if len(allMembers) == 2 { // Meaning its just by itself and one another peer who just crashed
								// Update the view and print the status
								stateManager.UpdateView(&MembershipMessage{
									ViewId:        currentState.ViewId + 1,
									OperationType: DELETE,
									PeerId:        peerId,
								})
								println("{peer_id: ", self, ", view_id: ", stateManager.GetViewId(), ", leader: ", leader, ", memb_list: ", arrayToString(stateManager.GetMembers()), "}")
							} else {
								// Send DELETE request message to all the other peers
								requestId := stateManager.GetNextRequestId()
								for _, memberId := range allMembers {
									if peer, ok := peerManager.GetPeer(memberId); ok && memberId != self && memberId != peerId {
										newRequestMessage := &MembershipMessage{
											OperationType: DELETE,
											RequestId:     requestId,
											ViewId:        currentState.ViewId,
											PeerId:        peerId,
										}
										err := SendReq(peer.Conn, peerManager.GetSelfID(), newRequestMessage)
										if err != nil {
											println("Cound not send REQ Message to peer: %v", peerId)
										} else {
											stateManager.AddRequestEntry(requestId, newRequestMessage)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

func handleCustomCrashing(sentJoinAndReadyToCrashCh chan bool, peerManager *PeerManager, stateManager *StateManager) {
	if config.crashAfterJoinDelay == 0.0 {
		return
	} else {
		for {
			select {
			case <-sentJoinAndReadyToCrashCh:
				time.Sleep(time.Duration(config.crashAfterJoinDelay * float64(time.Second)))
				println("{peer_id:", peerManager.GetSelfID(), ", view_id: ", stateManager.GetCurrentState().ViewId, ", leader: ", peerManager.GetLeader(), ", message:\"crashing\"}")
				os.Exit(1)
			}
		}
	}
}

func main() {
	parseFlagsAndAssignConstants()

	peerManager := initializePeerManager()
	stateManager := NewStateManager(peerManager)

	// Sleep for initial delay
	readyCh := make(chan bool)
	go getReady(readyCh)
	<-readyCh // Block until received ready

	stateManager.addMember(peerManager.GetSelfID()) // Get counted for membership
	println("{peer_id: ", peerManager.GetSelfID(), ", view_id: ", stateManager.GetViewId(), ", leader: ", peerManager.GetLeader(), ", memb_list: ", arrayToString(stateManager.GetMembers()), "}")

	// Send heartbeats when triggered
	for _, peer := range peerManager.GetPeers() {
		if peer.ID != peerManager.GetSelfID() {
			go handleSendingHeartbeats(peer, peerManager)
		}
	}
	// Hande receiving heartbeats
	HEARTBEATReceiverCh := make(chan HeartbeatMessage, 100)
	go ReceiveHeartbeats(HEARTBEATReceiverCh)
	go handleReceivingHeartbeats(HEARTBEATReceiverCh, stateManager, peerManager)
	// Check for failures
	go handleCheckingFailures(stateManager, peerManager)

	// Start Listening
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", config.Hostname, config.TCPPort))
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	messageReceiverCh := make(chan *Message, 100)
	go func() { // Keep listening for connections
		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Error accepting connection: %v\n", err)
				continue
			}
			go HandleConnection(conn, messageReceiverCh)
		}
	}()

	// Establish connections as and when needed
	connectionsToBeEstablishedCh := make(chan *Peer, 100)
	connectionEstablishedCh := make(chan bool, 1)
	go establishConnections(peerManager, connectionsToBeEstablishedCh, connectionEstablishedCh)

	// HandleCrash - Custom crashing for testing
	sentJoinAndReadyToCrashCh := make(chan bool, 1)
	go handleCustomCrashing(sentJoinAndReadyToCrashCh, peerManager, stateManager)

	// Send JOIN message to leader
	if peerManager.GetSelfID() != peerManager.GetLeader() {
		leader, exists := peerManager.GetPeer(peerManager.GetLeader())
		if !exists {
			log.Fatalf("Leader not decided from the startup")
		}
		connectionsToBeEstablishedCh <- &leader
		established := <-connectionEstablishedCh // Wait for the connection to be established with leader
		if established {
			leader, _ := peerManager.GetPeer(peerManager.GetLeader())
			err := SendJoin(leader.Conn, peerManager.GetSelfID(), &MembershipMessage{
				PeerId: peerManager.GetSelfID(),
			})
			if err != nil {
				println("Error sending the JOIN Message", err.Error())
			} else {
				sentJoinAndReadyToCrashCh <- true
			}
		}
	}

	JOINMessagesCh := make(chan *Message, 100)
	REQMessagesCh := make(chan *Message, 100)
	OKMessagesCh := make(chan *Message, 100)
	NEWVIEWMessageCh := make(chan *Message, 100)

	JOINMessagesActionCompleteCh := make(chan bool, 1)
	REQMessagesActionCompleteCh := make(chan bool, 1)
	OKMessagesActionCompleteCh := make(chan bool, 1)
	NEWVIEWMessageActionCompleteCh := make(chan bool, 1)

	go handleJoinRequests(JOINMessagesCh, JOINMessagesActionCompleteCh, stateManager, peerManager)
	go handleREQMessages(REQMessagesCh, REQMessagesActionCompleteCh, stateManager, peerManager)
	go handleOKMessages(OKMessagesCh, OKMessagesActionCompleteCh, stateManager, peerManager)
	go handleNEWVIEWMessage(NEWVIEWMessageCh, NEWVIEWMessageActionCompleteCh, stateManager, peerManager)

	for {
		select {
		case msg := <-messageReceiverCh:
			switch msg.Header.MessageType {
			case JOIN:
				println("Received JOIN message from ", msg.Header.SenderID)
				// Establish connection with peer
				if peer, ok := peerManager.GetPeer(int(msg.Header.SenderID)); ok {
					connectionsToBeEstablishedCh <- &peer
				}
				established := <-connectionEstablishedCh
				if established {
					println("Connection established with PEER that sent JOIN request")
				}
				JOINMessagesCh <- msg
				<-JOINMessagesActionCompleteCh
			case REQ:
				println("Received REQ message from ", msg.Header.SenderID)
				REQMessagesCh <- msg
				<-REQMessagesActionCompleteCh
			case OK:
				decoded, _ := decodeMembershipMessage(msg.Payload)
				println("Received OK message from ", msg.Header.SenderID, " with request ID ", decoded.RequestId)
				OKMessagesCh <- msg
				<-OKMessagesActionCompleteCh
			case NEWVIEW:
				decoded, _ := decodeMembershipMessage(msg.Payload)
				println("Received NEWVIEW message from ", msg.Header.SenderID, " with view ", decoded.ViewId)
				NEWVIEWMessageCh <- msg
				<-NEWVIEWMessageActionCompleteCh
			case NEWLEADER:
				println("MSG received")
			default:
				log.Printf("Received unknown message type: %T\n", msg.Header.MessageType)
			}
		}
	}
}
