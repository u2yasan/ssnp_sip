package store

import (
	"sync"
)

type Node struct {
	NodeID                    string
	ActiveAgentKeyFingerprint string
	AgentPublicKey            string
	EnrollmentGeneration      int
	LastHeartbeatSequence     int
	LastPolicyVersion         string
}

type CheckEvent struct {
	EventID       string
	NodeID        string
	OverallPassed bool
	CheckedAt     string
}

type TelemetryEvent struct {
	NodeID             string
	TelemetryTimestamp string
	WarningCode        string
}

type Store struct {
	mu              sync.RWMutex
	nodes           map[string]Node
	checkEvents     map[string]CheckEvent
	telemetryEvents []TelemetryEvent
}

func New(seedNodes []Node) *Store {
	nodes := make(map[string]Node, len(seedNodes))
	for _, node := range seedNodes {
		nodes[node.NodeID] = node
	}
	return &Store{
		nodes:       nodes,
		checkEvents: map[string]CheckEvent{},
	}
}

func (s *Store) GetNode(nodeID string) (Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	node, ok := s.nodes[nodeID]
	return node, ok
}

func (s *Store) SaveNode(node Node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[node.NodeID] = node
}

func (s *Store) SaveCheckEvent(event CheckEvent) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.checkEvents[event.EventID]; exists {
		return false
	}
	s.checkEvents[event.EventID] = event
	return true
}

func (s *Store) AddTelemetryEvent(event TelemetryEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.telemetryEvents = append(s.telemetryEvents, event)
}
