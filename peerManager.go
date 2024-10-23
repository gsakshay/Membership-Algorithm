package main

import (
	"fmt"
	"net"
	"sync"
)

type Peer struct {
	ID              int
	Hostname        string
	Address         string
	Conn            net.Conn
	SendHeartbeatCh chan bool
}

type PeerManager struct {
	peers     map[int]Peer
	peerMap   map[int]string
	peerCount int
	selfID    int
	leaderId  int
	mu        sync.RWMutex
}

func NewPeerManager() *PeerManager {
	return &PeerManager{
		peers:   make(map[int]Peer),
		peerMap: make(map[int]string),
	}
}

func (pm *PeerManager) KnowPeer(peerId int, address string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.peerMap[peerId] = address
}

func (pm *PeerManager) GetPeerName(id int) (string, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	peer, ok := pm.peerMap[id]
	return peer, ok
}

func (pm *PeerManager) AddPeer(id int) int {
	hostname, ok := pm.GetPeerName(id)
	if !ok {
		fmt.Printf("Unknown peer: %d", id)
		return -1
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	address := fmt.Sprintf("%s:%s", hostname, config.TCPPort)
	peer := Peer{ID: id, Hostname: hostname, Address: address, SendHeartbeatCh: make(chan bool, 100)}

	pm.peers[id] = peer
	pm.peerCount++

	return id
}

func (pm *PeerManager) DeletePeer(id int) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if _, exists := pm.peers[id]; !exists {
		return fmt.Errorf("peer with ID %d not found", id)
	}

	delete(pm.peers, id)
	pm.peerCount--

	if id == pm.leaderId {
		pm.relectLeader()
	}

	return nil
}

func (pm *PeerManager) relectLeader() {
	pm.leaderId = 0

	if pm.peerCount == 0 {
		pm.leaderId = pm.selfID
		return
	}

	for i := pm.leaderId + 1; i < 10; i++ { // TODO - Write this better
		if _, exists := pm.peers[i]; exists {
			pm.leaderId = i
			return
		}
	}

	if pm.leaderId == 0 {
		fmt.Println("Leader could not be elected")
	}
}

func (pm *PeerManager) GetPeers() []Peer {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	peers := make([]Peer, 0, len(pm.peers))
	for _, peer := range pm.peers {
		if peer.ID != pm.selfID {
			peers = append(peers, peer)
		}
	}
	return peers
}

func (pm *PeerManager) GetPeerCount() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.peerCount
}

func (pm *PeerManager) GetPeer(id int) (Peer, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	peer, ok := pm.peers[id]
	return peer, ok
}

func (pm *PeerManager) SetSelf(selfID int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.selfID = selfID
}

func (pm *PeerManager) GetSelfID() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.selfID
}

func (pm *PeerManager) GetSelf() (Peer, bool) {
	return pm.GetPeer(pm.GetSelfID())
}

func (pm *PeerManager) SetLeader(leaderId int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.leaderId = leaderId
}

func (pm *PeerManager) GetLeader() int {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.leaderId
}

func (pm *PeerManager) SetConnection(hostPeerId int, conn net.Conn) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("connect object is empty")
	}

	if peer, ok := pm.peers[hostPeerId]; ok {
		peer.Conn = conn
		pm.peers[peer.ID] = peer
		return nil
	}

	return fmt.Errorf("Peer with ID %d not found", hostPeerId)
}
