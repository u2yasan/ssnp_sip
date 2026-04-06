package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

type Node struct {
	NodeID                    string `yaml:"node_id" json:"node_id"`
	DisplayName               string `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	OperatorEmail             string `yaml:"operator_email,omitempty" json:"operator_email,omitempty"`
	Enabled                   bool   `yaml:"enabled" json:"enabled"`
	ActiveAgentKeyFingerprint string `json:"active_agent_key_fingerprint,omitempty"`
	AgentPublicKey            string `json:"agent_public_key,omitempty"`
	EnrollmentGeneration      int    `json:"enrollment_generation,omitempty"`
	LastHeartbeatSequence     int    `json:"last_heartbeat_sequence,omitempty"`
	LastHeartbeatTimestamp    string `json:"last_heartbeat_timestamp,omitempty"`
	LastPolicyVersion         string `json:"last_policy_version,omitempty"`
}

type CheckEvent struct {
	EventID       string `json:"event_id"`
	NodeID        string `json:"node_id"`
	OverallPassed bool   `json:"overall_passed"`
	CheckedAt     string `json:"checked_at"`
}

type TelemetryEvent struct {
	NodeID             string `json:"node_id"`
	TelemetryTimestamp string `json:"telemetry_timestamp"`
	WarningCode        string `json:"warning_code"`
}

type LatestTelemetry struct {
	NodeID             string `json:"node_id"`
	WarningCode        string `json:"warning_code"`
	TelemetryTimestamp string `json:"telemetry_timestamp"`
}

type AlertState struct {
	NodeID      string `json:"node_id"`
	AlertCode   string `json:"alert_code"`
	Severity    string `json:"severity"`
	LastSentAt  string `json:"last_sent_at"`
	LastChannel string `json:"last_channel,omitempty"`
	Recipient   string `json:"recipient,omitempty"`
}

type NotificationDelivery struct {
	NodeID      string `json:"node_id"`
	AlertCode   string `json:"alert_code"`
	Severity    string `json:"severity"`
	Channel     string `json:"channel"`
	Recipient   string `json:"recipient"`
	OccurredAt  string `json:"occurred_at"`
	SentAt      string `json:"sent_at"`
	Status      string `json:"status"`
	ErrorDetail string `json:"error_detail,omitempty"`
}

type OperationalEvent struct {
	NodeID         string `json:"node_id"`
	EventCode      string `json:"event_code"`
	Severity       string `json:"severity"`
	EventTimestamp string `json:"event_timestamp"`
	Detail         string `json:"detail,omitempty"`
}

type NodesConfig struct {
	Nodes []Node `yaml:"nodes"`
}

type snapshot struct {
	Nodes                  []Node                 `json:"nodes"`
	CheckEvents            []CheckEvent           `json:"check_events"`
	TelemetryEvents        []TelemetryEvent       `json:"telemetry_events"`
	LatestTelemetry        []LatestTelemetry      `json:"latest_telemetry"`
	AlertStates            []AlertState           `json:"alert_states"`
	NotificationDeliveries []NotificationDelivery `json:"notification_deliveries"`
	OperationalEvents      []OperationalEvent     `json:"operational_events"`
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
		if !node.Enabled {
			node.Enabled = true
		}
		nodes[node.NodeID] = node
	}
	return &Store{
		nodes:           nodes,
		checkEvents:     map[string]CheckEvent{},
		latestTelemetry: map[string]LatestTelemetry{},
		alertStates:     map[string]AlertState{},
	}
}

func LoadNodesConfig(path string) ([]Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg NodesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Nodes) == 0 {
		return nil, errors.New("missing nodes")
	}
	seen := map[string]struct{}{}
	out := make([]Node, 0, len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		if node.NodeID == "" {
			return nil, errors.New("missing node_id")
		}
		if _, exists := seen[node.NodeID]; exists {
			return nil, errors.New("duplicate node_id")
		}
		seen[node.NodeID] = struct{}{}
		if !node.Enabled {
			node.Enabled = true
		}
		node.ActiveAgentKeyFingerprint = ""
		node.AgentPublicKey = ""
		node.EnrollmentGeneration = 0
		node.LastHeartbeatSequence = 0
		node.LastHeartbeatTimestamp = ""
		node.LastPolicyVersion = ""
		out = append(out, node)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].NodeID < out[j].NodeID
	})
	return out, nil
}

func Load(seedNodes []Node, snapshotPath string) (*Store, error) {
	st := New(seedNodes)
	if snapshotPath == "" {
		return st, nil
	}
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return st, nil
		}
		return nil, err
	}
	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	if err := st.applySnapshot(snap); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Store) Save(path string) error {
	if path == "" {
		return errors.New("missing state path")
	}
	snap := s.snapshot()
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".portal-state-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func (s *Store) applySnapshot(snap snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	seedNodes := make(map[string]Node, len(s.nodes))
	for nodeID, node := range s.nodes {
		seedNodes[nodeID] = node
	}

	for _, node := range snap.Nodes {
		seedNode, ok := seedNodes[node.NodeID]
		if !ok {
			return errors.New("snapshot contains unknown node_id")
		}
		seedNode.ActiveAgentKeyFingerprint = node.ActiveAgentKeyFingerprint
		seedNode.AgentPublicKey = node.AgentPublicKey
		seedNode.EnrollmentGeneration = node.EnrollmentGeneration
		seedNode.LastHeartbeatSequence = node.LastHeartbeatSequence
		seedNode.LastHeartbeatTimestamp = node.LastHeartbeatTimestamp
		seedNode.LastPolicyVersion = node.LastPolicyVersion
		seedNodes[node.NodeID] = seedNode
	}
	s.nodes = seedNodes

	s.checkEvents = map[string]CheckEvent{}
	for _, event := range snap.CheckEvents {
		if _, ok := seedNodes[event.NodeID]; !ok {
			return errors.New("snapshot check event contains unknown node_id")
		}
		s.checkEvents[event.EventID] = event
	}

	s.telemetryEvents = nil
	for _, event := range snap.TelemetryEvents {
		if _, ok := seedNodes[event.NodeID]; !ok {
			return errors.New("snapshot telemetry contains unknown node_id")
		}
		s.telemetryEvents = append(s.telemetryEvents, event)
	}

	s.latestTelemetry = map[string]LatestTelemetry{}
	for _, event := range snap.LatestTelemetry {
		if _, ok := seedNodes[event.NodeID]; !ok {
			return errors.New("snapshot latest telemetry contains unknown node_id")
		}
		s.latestTelemetry[event.NodeID+"\x00"+event.WarningCode] = event
	}

	s.alertStates = map[string]AlertState{}
	for _, state := range snap.AlertStates {
		if _, ok := seedNodes[state.NodeID]; !ok {
			return errors.New("snapshot alert state contains unknown node_id")
		}
		s.alertStates[alertKey(state.NodeID, state.AlertCode, state.Severity)] = state
	}

	s.deliveries = nil
	for _, delivery := range snap.NotificationDeliveries {
		if _, ok := seedNodes[delivery.NodeID]; !ok {
			return errors.New("snapshot delivery contains unknown node_id")
		}
		s.deliveries = append(s.deliveries, delivery)
	}

	s.operational = nil
	for _, event := range snap.OperationalEvents {
		if _, ok := seedNodes[event.NodeID]; !ok {
			return errors.New("snapshot operational event contains unknown node_id")
		}
		s.operational = append(s.operational, event)
	}
	return nil
}

func (s *Store) snapshot() snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].NodeID < nodes[j].NodeID
	})

	checks := make([]CheckEvent, 0, len(s.checkEvents))
	for _, event := range s.checkEvents {
		checks = append(checks, event)
	}
	sort.Slice(checks, func(i, j int) bool {
		return checks[i].EventID < checks[j].EventID
	})

	latest := make([]LatestTelemetry, 0, len(s.latestTelemetry))
	for _, event := range s.latestTelemetry {
		latest = append(latest, event)
	}
	sort.Slice(latest, func(i, j int) bool {
		if latest[i].NodeID == latest[j].NodeID {
			return latest[i].WarningCode < latest[j].WarningCode
		}
		return latest[i].NodeID < latest[j].NodeID
	})

	alerts := make([]AlertState, 0, len(s.alertStates))
	for _, state := range s.alertStates {
		alerts = append(alerts, state)
	}
	sort.Slice(alerts, func(i, j int) bool {
		if alerts[i].NodeID == alerts[j].NodeID {
			if alerts[i].AlertCode == alerts[j].AlertCode {
				return alerts[i].Severity < alerts[j].Severity
			}
			return alerts[i].AlertCode < alerts[j].AlertCode
		}
		return alerts[i].NodeID < alerts[j].NodeID
	})

	deliveries := append([]NotificationDelivery(nil), s.deliveries...)
	operational := append([]OperationalEvent(nil), s.operational...)
	telemetry := append([]TelemetryEvent(nil), s.telemetryEvents...)

	return snapshot{
		Nodes:                  nodes,
		CheckEvents:            checks,
		TelemetryEvents:        telemetry,
		LatestTelemetry:        latest,
		AlertStates:            alerts,
		NotificationDeliveries: deliveries,
		OperationalEvents:      operational,
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
