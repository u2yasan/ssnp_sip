package store

import (
	"sort"
	"sync"
)

type Node struct {
	NodeID                    string
	ActiveAgentKeyFingerprint string
	AgentPublicKey            string
	EnrollmentGeneration      int
	LastHeartbeatSequence     int
	LastHeartbeatTimestamp    string
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

type LatestTelemetry struct {
	NodeID             string
	WarningCode        string
	TelemetryTimestamp string
}

type AlertState struct {
	NodeID      string
	AlertCode   string
	Severity    string
	LastSentAt  string
	LastChannel string
	Recipient   string
}

type NotificationDelivery struct {
	NodeID      string
	AlertCode   string
	Severity    string
	Channel     string
	Recipient   string
	OccurredAt  string
	SentAt      string
	Status      string
	ErrorDetail string
}

type OperationalEvent struct {
	NodeID         string
	EventCode      string
	Severity       string
	EventTimestamp string
	Detail         string
}

type Store struct {
	mu              sync.RWMutex
	nodes           map[string]Node
	checkEvents     map[string]CheckEvent
	telemetryEvents []TelemetryEvent
	latestTelemetry map[string]LatestTelemetry
	alertStates     map[string]AlertState
	deliveries      []NotificationDelivery
	operational     []OperationalEvent
}

func New(seedNodes []Node) *Store {
	nodes := make(map[string]Node, len(seedNodes))
	for _, node := range seedNodes {
		nodes[node.NodeID] = node
	}
	return &Store{
		nodes:           nodes,
		checkEvents:     map[string]CheckEvent{},
		latestTelemetry: map[string]LatestTelemetry{},
		alertStates:     map[string]AlertState{},
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

func (s *Store) ListNodes() []Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		out = append(out, node)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].NodeID < out[j].NodeID
	})
	return out
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

func (s *Store) GetCheckEvent(eventID string) (CheckEvent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	event, ok := s.checkEvents[eventID]
	return event, ok
}

func (s *Store) AddTelemetryEvent(event TelemetryEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.telemetryEvents = append(s.telemetryEvents, event)
	key := event.NodeID + "\x00" + event.WarningCode
	s.latestTelemetry[key] = LatestTelemetry{
		NodeID:             event.NodeID,
		WarningCode:        event.WarningCode,
		TelemetryTimestamp: event.TelemetryTimestamp,
	}
}

func (s *Store) ListTelemetry(nodeID, warningCode string) []TelemetryEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []TelemetryEvent
	for _, event := range s.telemetryEvents {
		if nodeID != "" && event.NodeID != nodeID {
			continue
		}
		if warningCode != "" && event.WarningCode != warningCode {
			continue
		}
		out = append(out, event)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].TelemetryTimestamp > out[j].TelemetryTimestamp
	})
	return out
}

func (s *Store) ListLatestTelemetry(nodeID, warningCode string) []LatestTelemetry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []LatestTelemetry
	for _, event := range s.latestTelemetry {
		if nodeID != "" && event.NodeID != nodeID {
			continue
		}
		if warningCode != "" && event.WarningCode != warningCode {
			continue
		}
		out = append(out, event)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].NodeID == out[j].NodeID {
			return out[i].WarningCode < out[j].WarningCode
		}
		return out[i].NodeID < out[j].NodeID
	})
	return out
}

func (s *Store) GetAlertState(nodeID, alertCode, severity string) (AlertState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.alertStates[alertKey(nodeID, alertCode, severity)]
	return state, ok
}

func (s *Store) SaveAlertState(state AlertState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alertStates[alertKey(state.NodeID, state.AlertCode, state.Severity)] = state
}

func (s *Store) AddNotificationDelivery(delivery NotificationDelivery) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveries = append(s.deliveries, delivery)
}

func (s *Store) ListNotificationDeliveries(nodeID, alertCode string) []NotificationDelivery {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []NotificationDelivery
	for _, delivery := range s.deliveries {
		if nodeID != "" && delivery.NodeID != nodeID {
			continue
		}
		if alertCode != "" && delivery.AlertCode != alertCode {
			continue
		}
		out = append(out, delivery)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].SentAt > out[j].SentAt
	})
	return out
}

func (s *Store) AddOperationalEvent(event OperationalEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operational = append(s.operational, event)
}

func (s *Store) ListOperationalEvents(nodeID, eventCode string) []OperationalEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []OperationalEvent
	for _, event := range s.operational {
		if nodeID != "" && event.NodeID != nodeID {
			continue
		}
		if eventCode != "" && event.EventCode != eventCode {
			continue
		}
		out = append(out, event)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].EventTimestamp > out[j].EventTimestamp
	})
	return out
}

func alertKey(nodeID, alertCode, severity string) string {
	return nodeID + "\x00" + alertCode + "\x00" + severity
}
