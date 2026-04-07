package store

import (
	"sort"
	"strings"
)

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

func (s *Store) SaveEnrollmentChallenge(challenge EnrollmentChallenge) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.challenges[challenge.ChallengeID] = challenge
}

func (s *Store) GetEnrollmentChallenge(challengeID string) (EnrollmentChallenge, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	challenge, ok := s.challenges[challengeID]
	return challenge, ok
}

func (s *Store) ConsumeEnrollmentChallenge(challengeID, usedAt string) (EnrollmentChallenge, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, ok := s.challenges[challengeID]
	if !ok {
		return EnrollmentChallenge{}, false
	}
	challenge.UsedAt = usedAt
	s.challenges[challengeID] = challenge
	return challenge, true
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

func (s *Store) LatestCheckEventForNode(nodeID string) (CheckEvent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest CheckEvent
	found := false
	for _, event := range s.checkEvents {
		if event.NodeID != nodeID {
			continue
		}
		if !found || event.CheckedAt > latest.CheckedAt {
			latest = event
			found = true
		}
	}
	return latest, found
}

func (s *Store) LatestCheckEventForNodeAndDate(nodeID, dateUTC string) (CheckEvent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest CheckEvent
	found := false
	for _, event := range s.checkEvents {
		if event.NodeID != nodeID {
			continue
		}
		if dateUTC != "" && !strings.HasPrefix(event.CheckedAt, dateUTC) {
			continue
		}
		if !found || event.CheckedAt > latest.CheckedAt {
			latest = event
			found = true
		}
	}
	return latest, found
}

func (s *Store) SaveHeartbeatEvent(event HeartbeatEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.heartbeatEvents = append(s.heartbeatEvents, event)
}

func (s *Store) ListHeartbeatEventsByNode(nodeID string) []HeartbeatEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []HeartbeatEvent
	for _, event := range s.heartbeatEvents {
		if nodeID != "" && event.NodeID != nodeID {
			continue
		}
		out = append(out, event)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].HeartbeatTimestamp == out[j].HeartbeatTimestamp {
			return out[i].SequenceNumber > out[j].SequenceNumber
		}
		return out[i].HeartbeatTimestamp > out[j].HeartbeatTimestamp
	})
	return out
}

func (s *Store) LatestHeartbeatEventForNodeAndDate(nodeID, dateUTC string) (HeartbeatEvent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest HeartbeatEvent
	found := false
	for _, event := range s.heartbeatEvents {
		if event.NodeID != nodeID {
			continue
		}
		if dateUTC != "" && !strings.HasPrefix(event.HeartbeatTimestamp, dateUTC) {
			continue
		}
		if !found || event.HeartbeatTimestamp > latest.HeartbeatTimestamp {
			latest = event
			found = true
		}
	}
	return latest, found
}

func (s *Store) SaveProbeEvent(event ProbeEvent) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.probeEvents[event.ProbeID]; exists {
		return false
	}
	s.probeEvents[event.ProbeID] = event
	return true
}

func (s *Store) GetProbeEvent(probeID string) (ProbeEvent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	event, ok := s.probeEvents[probeID]
	return event, ok
}

func (s *Store) ListProbeEventsByNodeAndDate(nodeID, dateUTC string) []ProbeEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ProbeEvent
	for _, event := range s.probeEvents {
		if nodeID != "" && event.NodeID != nodeID {
			continue
		}
		if dateUTC != "" && !strings.HasPrefix(event.ObservedAt, dateUTC) {
			continue
		}
		out = append(out, event)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ObservedAt == out[j].ObservedAt {
			return out[i].ProbeID < out[j].ProbeID
		}
		return out[i].ObservedAt > out[j].ObservedAt
	})
	return out
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
