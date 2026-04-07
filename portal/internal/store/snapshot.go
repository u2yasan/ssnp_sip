package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

type snapshot struct {
	Nodes                      []Node                       `json:"nodes"`
	EnrollmentChallenges       []EnrollmentChallenge        `json:"enrollment_challenges"`
	CheckEvents                []CheckEvent                 `json:"check_events"`
	HeartbeatEvents            []HeartbeatEvent             `json:"heartbeat_events"`
	TelemetryEvents            []TelemetryEvent             `json:"telemetry_events"`
	ProbeEvents                []ProbeEvent                 `json:"probe_events"`
	DailySummaries             []DailyQualificationSummary  `json:"daily_summaries"`
	QualifiedDecisions         []QualifiedDecisionRecord    `json:"qualified_decisions"`
	BasePerformanceRecords     []BasePerformanceRecord      `json:"base_performance_records"`
	RankingRecords             []RankingRecord              `json:"ranking_records"`
	RewardEligibility          []RewardEligibilityRecord    `json:"reward_eligibility"`
	RewardAllocations          []RewardAllocationRecord     `json:"reward_allocations"`
	OperatorGroupEvidence      []OperatorGroupEvidence      `json:"operator_group_evidence"`
	VotingKeyEvidence          []VotingKeyEvidence          `json:"voting_key_evidence"`
	DecentralizationEvidence   []DecentralizationEvidence   `json:"decentralization_evidence"`
	DomainEvidence             []DomainEvidence             `json:"domain_evidence"`
	SharedControlPlaneEvidence []SharedControlPlaneEvidence `json:"shared_control_plane_evidence"`
	LatestTelemetry            []LatestTelemetry            `json:"latest_telemetry"`
	AlertStates                []AlertState                 `json:"alert_states"`
	NotificationDeliveries     []NotificationDelivery       `json:"notification_deliveries"`
	OperationalEvents          []OperationalEvent           `json:"operational_events"`
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
		node.ValidatedRegistrationAt = ""
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
		seedNode.ValidatedRegistrationAt = node.ValidatedRegistrationAt
		seedNode.LastHeartbeatSequence = node.LastHeartbeatSequence
		seedNode.LastHeartbeatTimestamp = node.LastHeartbeatTimestamp
		seedNode.LastPolicyVersion = node.LastPolicyVersion
		seedNodes[node.NodeID] = seedNode
	}
	s.nodes = seedNodes

	s.checkEvents = map[string]CheckEvent{}
	s.challenges = map[string]EnrollmentChallenge{}
	for _, challenge := range snap.EnrollmentChallenges {
		if _, ok := seedNodes[challenge.NodeID]; !ok {
			return errors.New("snapshot enrollment challenge contains unknown node_id")
		}
		s.challenges[challenge.ChallengeID] = challenge
	}
	for _, event := range snap.CheckEvents {
		if _, ok := seedNodes[event.NodeID]; !ok {
			return errors.New("snapshot check event contains unknown node_id")
		}
		s.checkEvents[event.EventID] = event
	}

	s.heartbeatEvents = nil
	for _, event := range snap.HeartbeatEvents {
		if _, ok := seedNodes[event.NodeID]; !ok {
			return errors.New("snapshot heartbeat event contains unknown node_id")
		}
		s.heartbeatEvents = append(s.heartbeatEvents, event)
	}

	s.telemetryEvents = nil
	for _, event := range snap.TelemetryEvents {
		if _, ok := seedNodes[event.NodeID]; !ok {
			return errors.New("snapshot telemetry contains unknown node_id")
		}
		s.telemetryEvents = append(s.telemetryEvents, event)
	}

	s.probeEvents = map[string]ProbeEvent{}
	for _, event := range snap.ProbeEvents {
		if _, ok := seedNodes[event.NodeID]; !ok {
			return errors.New("snapshot probe event contains unknown node_id")
		}
		s.probeEvents[event.ProbeID] = event
	}

	s.dailySummaries = map[string]DailyQualificationSummary{}
	for _, summary := range snap.DailySummaries {
		if _, ok := seedNodes[summary.NodeID]; !ok {
			return errors.New("snapshot daily summary contains unknown node_id")
		}
		s.dailySummaries[dailySummaryKey(summary.NodeID, summary.DateUTC)] = summary
	}

	s.qualifiedDecisions = map[string]QualifiedDecisionRecord{}
	for _, decision := range snap.QualifiedDecisions {
		if _, ok := seedNodes[decision.NodeID]; !ok {
			return errors.New("snapshot qualified decision contains unknown node_id")
		}
		s.qualifiedDecisions[qualifiedDecisionKey(decision.NodeID, decision.DateUTC)] = decision
	}

	s.basePerformance = map[string]BasePerformanceRecord{}
	for _, record := range snap.BasePerformanceRecords {
		if _, ok := seedNodes[record.NodeID]; !ok {
			return errors.New("snapshot base performance record contains unknown node_id")
		}
		s.basePerformance[basePerformanceKey(record.NodeID, record.DateUTC)] = record
	}

	s.rankingRecords = map[string]RankingRecord{}
	for _, record := range snap.RankingRecords {
		if _, ok := seedNodes[record.NodeID]; !ok {
			return errors.New("snapshot ranking record contains unknown node_id")
		}
		s.rankingRecords[rankingRecordKey(record.NodeID, record.DateUTC)] = record
	}

	s.rewardEligibility = map[string]RewardEligibilityRecord{}
	for _, record := range snap.RewardEligibility {
		if _, ok := seedNodes[record.NodeID]; !ok {
			return errors.New("snapshot reward eligibility contains unknown node_id")
		}
		s.rewardEligibility[rewardEligibilityKey(record.NodeID, record.DateUTC)] = record
	}
	s.rewardAllocations = map[string]RewardAllocationRecord{}
	for _, record := range snap.RewardAllocations {
		if _, ok := seedNodes[record.NodeID]; !ok {
			return errors.New("snapshot reward allocation contains unknown node_id")
		}
		s.rewardAllocations[rewardAllocationKey(record.NodeID, record.DateUTC)] = record
	}

	s.operatorGroups = map[string]OperatorGroupEvidence{}
	for _, evidence := range snap.OperatorGroupEvidence {
		if _, ok := seedNodes[evidence.NodeID]; !ok {
			return errors.New("snapshot operator group evidence contains unknown node_id")
		}
		s.operatorGroups[evidence.EvidenceRef] = evidence
	}

	s.votingKeyEvidence = map[string]VotingKeyEvidence{}
	for _, evidence := range snap.VotingKeyEvidence {
		if _, ok := seedNodes[evidence.NodeID]; !ok {
			return errors.New("snapshot voting key evidence contains unknown node_id")
		}
		s.votingKeyEvidence[evidence.EvidenceRef] = evidence
	}
	s.decentralization = map[string]DecentralizationEvidence{}
	for _, evidence := range snap.DecentralizationEvidence {
		if _, ok := seedNodes[evidence.NodeID]; !ok {
			return errors.New("snapshot decentralization evidence contains unknown node_id")
		}
		s.decentralization[evidence.EvidenceRef] = evidence
	}
	s.domainEvidence = map[string]DomainEvidence{}
	for _, evidence := range snap.DomainEvidence {
		if _, ok := seedNodes[evidence.NodeID]; !ok {
			return errors.New("snapshot domain evidence contains unknown node_id")
		}
		s.domainEvidence[evidence.EvidenceRef] = evidence
	}
	s.controlPlane = map[string]SharedControlPlaneEvidence{}
	for _, evidence := range snap.SharedControlPlaneEvidence {
		if _, ok := seedNodes[evidence.NodeID]; !ok {
			return errors.New("snapshot shared control plane evidence contains unknown node_id")
		}
		s.controlPlane[evidence.EvidenceRef] = evidence
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

	challenges := make([]EnrollmentChallenge, 0, len(s.challenges))
	for _, challenge := range s.challenges {
		challenges = append(challenges, challenge)
	}
	sort.Slice(challenges, func(i, j int) bool {
		if challenges[i].NodeID == challenges[j].NodeID {
			return challenges[i].ChallengeID < challenges[j].ChallengeID
		}
		return challenges[i].NodeID < challenges[j].NodeID
	})

	heartbeats := append([]HeartbeatEvent(nil), s.heartbeatEvents...)
	sort.Slice(heartbeats, func(i, j int) bool {
		if heartbeats[i].HeartbeatTimestamp == heartbeats[j].HeartbeatTimestamp {
			if heartbeats[i].NodeID == heartbeats[j].NodeID {
				return heartbeats[i].SequenceNumber < heartbeats[j].SequenceNumber
			}
			return heartbeats[i].NodeID < heartbeats[j].NodeID
		}
		return heartbeats[i].HeartbeatTimestamp < heartbeats[j].HeartbeatTimestamp
	})

	probes := make([]ProbeEvent, 0, len(s.probeEvents))
	for _, event := range s.probeEvents {
		probes = append(probes, event)
	}
	sort.Slice(probes, func(i, j int) bool {
		return probes[i].ProbeID < probes[j].ProbeID
	})

	dailySummaries := make([]DailyQualificationSummary, 0, len(s.dailySummaries))
	for _, summary := range s.dailySummaries {
		dailySummaries = append(dailySummaries, summary)
	}
	sort.Slice(dailySummaries, func(i, j int) bool {
		if dailySummaries[i].DateUTC == dailySummaries[j].DateUTC {
			return dailySummaries[i].NodeID < dailySummaries[j].NodeID
		}
		return dailySummaries[i].DateUTC < dailySummaries[j].DateUTC
	})

	qualifiedDecisions := make([]QualifiedDecisionRecord, 0, len(s.qualifiedDecisions))
	for _, decision := range s.qualifiedDecisions {
		qualifiedDecisions = append(qualifiedDecisions, decision)
	}
	sort.Slice(qualifiedDecisions, func(i, j int) bool {
		if qualifiedDecisions[i].DateUTC == qualifiedDecisions[j].DateUTC {
			return qualifiedDecisions[i].NodeID < qualifiedDecisions[j].NodeID
		}
		return qualifiedDecisions[i].DateUTC < qualifiedDecisions[j].DateUTC
	})

	basePerformance := make([]BasePerformanceRecord, 0, len(s.basePerformance))
	for _, record := range s.basePerformance {
		basePerformance = append(basePerformance, record)
	}
	sort.Slice(basePerformance, func(i, j int) bool {
		if basePerformance[i].DateUTC == basePerformance[j].DateUTC {
			return basePerformance[i].NodeID < basePerformance[j].NodeID
		}
		return basePerformance[i].DateUTC < basePerformance[j].DateUTC
	})

	rankingRecords := make([]RankingRecord, 0, len(s.rankingRecords))
	for _, record := range s.rankingRecords {
		rankingRecords = append(rankingRecords, record)
	}
	sort.Slice(rankingRecords, func(i, j int) bool {
		if rankingRecords[i].DateUTC == rankingRecords[j].DateUTC {
			if rankingRecords[i].RankPosition == rankingRecords[j].RankPosition {
				return rankingRecords[i].NodeID < rankingRecords[j].NodeID
			}
			return rankingRecords[i].RankPosition < rankingRecords[j].RankPosition
		}
		return rankingRecords[i].DateUTC < rankingRecords[j].DateUTC
	})

	rewardEligibility := make([]RewardEligibilityRecord, 0, len(s.rewardEligibility))
	for _, record := range s.rewardEligibility {
		rewardEligibility = append(rewardEligibility, record)
	}
	sort.Slice(rewardEligibility, func(i, j int) bool {
		if rewardEligibility[i].DateUTC == rewardEligibility[j].DateUTC {
			if rewardEligibility[i].RankPosition == rewardEligibility[j].RankPosition {
				return rewardEligibility[i].NodeID < rewardEligibility[j].NodeID
			}
			return rewardEligibility[i].RankPosition < rewardEligibility[j].RankPosition
		}
		return rewardEligibility[i].DateUTC < rewardEligibility[j].DateUTC
	})

	rewardAllocations := make([]RewardAllocationRecord, 0, len(s.rewardAllocations))
	for _, record := range s.rewardAllocations {
		rewardAllocations = append(rewardAllocations, record)
	}
	sort.Slice(rewardAllocations, func(i, j int) bool {
		if rewardAllocations[i].DateUTC == rewardAllocations[j].DateUTC {
			if rewardAllocations[i].RankPosition == rewardAllocations[j].RankPosition {
				return rewardAllocations[i].NodeID < rewardAllocations[j].NodeID
			}
			return rewardAllocations[i].RankPosition < rewardAllocations[j].RankPosition
		}
		return rewardAllocations[i].DateUTC < rewardAllocations[j].DateUTC
	})

	operatorGroupEvidence := make([]OperatorGroupEvidence, 0, len(s.operatorGroups))
	for _, evidence := range s.operatorGroups {
		operatorGroupEvidence = append(operatorGroupEvidence, evidence)
	}
	sort.Slice(operatorGroupEvidence, func(i, j int) bool {
		return operatorGroupEvidence[i].EvidenceRef < operatorGroupEvidence[j].EvidenceRef
	})

	votingKeyEvidence := make([]VotingKeyEvidence, 0, len(s.votingKeyEvidence))
	for _, evidence := range s.votingKeyEvidence {
		votingKeyEvidence = append(votingKeyEvidence, evidence)
	}
	sort.Slice(votingKeyEvidence, func(i, j int) bool {
		return votingKeyEvidence[i].EvidenceRef < votingKeyEvidence[j].EvidenceRef
	})

	decentralizationEvidence := make([]DecentralizationEvidence, 0, len(s.decentralization))
	for _, evidence := range s.decentralization {
		decentralizationEvidence = append(decentralizationEvidence, evidence)
	}
	sort.Slice(decentralizationEvidence, func(i, j int) bool {
		return decentralizationEvidence[i].EvidenceRef < decentralizationEvidence[j].EvidenceRef
	})

	domainEvidence := make([]DomainEvidence, 0, len(s.domainEvidence))
	for _, evidence := range s.domainEvidence {
		domainEvidence = append(domainEvidence, evidence)
	}
	sort.Slice(domainEvidence, func(i, j int) bool {
		return domainEvidence[i].EvidenceRef < domainEvidence[j].EvidenceRef
	})

	sharedControlPlaneEvidence := make([]SharedControlPlaneEvidence, 0, len(s.controlPlane))
	for _, evidence := range s.controlPlane {
		sharedControlPlaneEvidence = append(sharedControlPlaneEvidence, evidence)
	}
	sort.Slice(sharedControlPlaneEvidence, func(i, j int) bool {
		return sharedControlPlaneEvidence[i].EvidenceRef < sharedControlPlaneEvidence[j].EvidenceRef
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
		Nodes:                      nodes,
		EnrollmentChallenges:       challenges,
		CheckEvents:                checks,
		HeartbeatEvents:            heartbeats,
		TelemetryEvents:            telemetry,
		ProbeEvents:                probes,
		DailySummaries:             dailySummaries,
		QualifiedDecisions:         qualifiedDecisions,
		BasePerformanceRecords:     basePerformance,
		RankingRecords:             rankingRecords,
		RewardEligibility:          rewardEligibility,
		RewardAllocations:          rewardAllocations,
		OperatorGroupEvidence:      operatorGroupEvidence,
		VotingKeyEvidence:          votingKeyEvidence,
		DecentralizationEvidence:   decentralizationEvidence,
		DomainEvidence:             domainEvidence,
		SharedControlPlaneEvidence: sharedControlPlaneEvidence,
		LatestTelemetry:            latest,
		AlertStates:                alerts,
		NotificationDeliveries:     deliveries,
		OperationalEvents:          operational,
	}
}
