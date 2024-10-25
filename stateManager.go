package main

import (
	"fmt"
	"sort"
	"sync"
)

type OperationType string

const (
	ADD     OperationType = "ADD"
	DELETE  OperationType = "DELETE"
	PENDING OperationType = "PENDING"
	NOTHING OperationType = "NOTHING"
)

type State struct {
	ViewId     int
	MemberList []int
}

type MembershipMessage struct {
	OperationType    OperationType
	PeerId           int
	ViewId           int
	RequestId        int
	PendingRequestId int
	MembershipList   []int
}

type RequestEntry struct {
	Message *MembershipMessage
}

type StateManager struct {
	currentState                State
	peerManager                 *PeerManager
	requestEntries              map[int]*RequestEntry
	numberOfOkReceived          map[int]int
	numberOfNewLeaderOkReceived map[int]int
	peerStatus                  map[int]int
	nextRequestId               int
	mu                          sync.RWMutex
}

func NewStateManager(peerManager *PeerManager) *StateManager {
	return &StateManager{
		currentState:                State{ViewId: 1},
		peerManager:                 peerManager,
		requestEntries:              make(map[int]*RequestEntry),
		numberOfOkReceived:          make(map[int]int),
		numberOfNewLeaderOkReceived: make(map[int]int),
		peerStatus:                  make(map[int]int),
		nextRequestId:               1,
	}
}

func (sm *StateManager) IncrementPeerStatus(peerId int) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sm.peerStatus[peerId] = sm.peerStatus[peerId] + 1
	return sm.peerStatus[peerId]
}

func (sm *StateManager) DecrementPeerStatus(peerId int) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sm.peerStatus[peerId] = sm.peerStatus[peerId] - 1
	return sm.peerStatus[peerId]
}

func (sm *StateManager) GetPeerStatus(peerId int) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.peerStatus[peerId]
}

func (sm *StateManager) GetCurrentState() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState
}

func (sm *StateManager) UpdateOkEntries(requestId int) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.numberOfOkReceived[requestId] = sm.numberOfOkReceived[requestId] + 1
	return sm.numberOfOkReceived[requestId]
}

func (sm *StateManager) UpdateNewLeaderOkEntries(requestId int) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.numberOfNewLeaderOkReceived[requestId] = sm.numberOfNewLeaderOkReceived[requestId] + 1
	return sm.numberOfNewLeaderOkReceived[requestId]
}

func (sm *StateManager) IncrementViewId() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.currentState.ViewId++
}

func (sm *StateManager) addMember(peerId int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if the peerId already exists in the MemberList
	for _, id := range sm.currentState.MemberList {
		if id == peerId {
			// fmt.Println("Received a duplicate ADD request ", id)
			return // Peer ID already exists, no need to add
		}
	}

	// Add the new peer ID to the MemberList
	sm.currentState.MemberList = append(sm.currentState.MemberList, peerId)
	sm.peerStatus[peerId] = 2
}

func (sm *StateManager) GetMembers() []int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	members := make([]int, 0, len(sm.currentState.MemberList))
	for _, mem := range sm.currentState.MemberList {
		members = append(members, mem)
	}

	sort.Ints(members) // Sort members in ascending order before returning
	return members
}

func (sm *StateManager) removeMember(peerId int) {
	// sm.mu.Lock()
	// defer sm.mu.Unlock() // This is not needed because the caller already has the lock and is expected to be a private method
	// Find and remove the peer ID from the MemberList
	for i, id := range sm.currentState.MemberList {
		if id == peerId {
			// Remove the peerId by slicing out the element
			sm.currentState.MemberList = append(sm.currentState.MemberList[:i], sm.currentState.MemberList[i+1:]...)
			delete(sm.peerStatus, peerId)
			return
		}
	}
}

func (sm *StateManager) GetNextRequestId() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	nameHash := sm.peerManager.GetSelfHash()
	uniqueId := nameHash + sm.nextRequestId
	sm.nextRequestId = sm.nextRequestId + 1

	if sm.nextRequestId >= MaxRequestId {
		sm.nextRequestId = 1
	}

	return uniqueId
}

func (sm *StateManager) SetRequestId(requestId int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.nextRequestId = requestId
}

func (sm *StateManager) AddRequestEntry(requestId int, message *MembershipMessage) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.requestEntries[requestId] = &RequestEntry{Message: message}
	// println("Added request entry with request ID: ", requestId, "Current total request entries: ", len(sm.requestEntries))
}

func (sm *StateManager) GetRequestEntry(requestId int) (*RequestEntry, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	entry, exists := sm.requestEntries[requestId]
	return entry, exists
}

func (sm *StateManager) DeleteRequestEntry(requestId int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.requestEntries, requestId)
	// println("Deleted request entry with request ID: ", requestId, "Current total request entries: ", len(sm.requestEntries))
}

func (sm *StateManager) UpdateView(message *MembershipMessage) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.currentState.ViewId = message.ViewId

	if message == nil {
		return fmt.Errorf("MembershipMessage is nil")
	}

	switch message.OperationType {
	case ADD:
		// Update the view
		if len(message.MembershipList) == 0 && message.PeerId == 0 {
			return fmt.Errorf("No members in the Membership List or PeerID")
		}
		if len(message.MembershipList) > 0 {
			sm.currentState.MemberList = message.MembershipList
			for _, id := range sm.currentState.MemberList {
				sm.peerStatus[id] = 2
			}
			// Start sending heartbeats
			for _, peerId := range message.MembershipList {
				if peer, ok := sm.peerManager.GetPeer(peerId); ok {
					peer.SendHeartbeatCh <- true
				}
			}
		} else if message.PeerId != 0 {
			for _, id := range sm.currentState.MemberList {
				if id == message.PeerId {
					// fmt.Println("Received a duplicate ADD request ", id)
					return nil // Peer ID already exists, no need to add
				}
			}

			// Add the new peer ID to the MemberList
			sm.currentState.MemberList = append(sm.currentState.MemberList, message.PeerId)
			sm.peerStatus[message.PeerId] = 2
			// Start sending heartbeats
			if peer, ok := sm.peerManager.GetPeer(message.PeerId); ok {
				peer.SendHeartbeatCh <- true
			}
		}
		return nil
	case DELETE:
		if message.PeerId != 0 {
			sm.removeMember(message.PeerId)
			// Stop sending heartbeats
			if peer, ok := sm.peerManager.GetPeer(message.PeerId); ok {
				peer.SendHeartbeatCh <- false
			}
			// Check if it was the leader and update the new leader
			leader := sm.peerManager.GetLeader()
			if message.PeerId == leader {
				// fmt.Println("Leader is leaving, updating new leader")
				if nextLeaderTobe, exists := nextLeader(leader, sm.currentState.MemberList); exists {
					sm.peerManager.SetLeader(nextLeaderTobe)
					// fmt.Println("New leader is: ", nextLeaderTobe)
				} else {
					fmt.Println("No new leader found")
				}
			}
		} else {
			return fmt.Errorf("no peer ID provided for DELETE operation")
		}
	default:
		return fmt.Errorf("unknown operation type: %s", message.OperationType)
	}

	return nil
}

func (sm *StateManager) CreateMembershipMessage(operationType OperationType, peerId int, newMember *Peer, includeMembershipList bool) *MembershipMessage {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	message := &MembershipMessage{
		OperationType: operationType,
		PeerId:        peerId,
		ViewId:        sm.currentState.ViewId,
	}

	var peerIds []int

	for _, peer := range sm.peerManager.GetPeers() {
		peerIds = append(peerIds, peer.ID)
	}

	if includeMembershipList {
		message.MembershipList = peerIds
	}

	return message
}

func (sm *StateManager) GetViewId() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentState.ViewId
}
