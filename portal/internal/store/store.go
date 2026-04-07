package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	ValidatedRegistrationAt   string `json:"validated_registration_at,omitempty"`
	LastHeartbeatSequence     int    `json:"last_heartbeat_sequence,omitempty"`
	LastHeartbeatTimestamp    string `json:"last_heartbeat_timestamp,omitempty"`
	LastPolicyVersion         string `json:"last_policy_version,omitempty"`
}

type EnrollmentChallenge struct {
	ChallengeID string `json:"challenge_id"`
	NodeID      string `json:"node_id"`
	IssuedAt    string `json:"issued_at"`
	ExpiresAt   string `json:"expires_at"`
	UsedAt      string `json:"used_at,omitempty"`
}

type CheckEvent struct {
	EventID       string `json:"event_id"`
	NodeID        string `json:"node_id"`
	OverallPassed bool   `json:"overall_passed"`
	CheckedAt     string `json:"checked_at"`
}

type HeartbeatEvent struct {
	NodeID             string `json:"node_id"`
	HeartbeatTimestamp string `json:"heartbeat_timestamp"`
	SequenceNumber     int    `json:"sequence_number"`
}

type TelemetryEvent struct {
	NodeID             string `json:"node_id"`
	TelemetryTimestamp string `json:"telemetry_timestamp"`
	WarningCode        string `json:"warning_code"`
}

type ProbeEvent struct {
	ProbeID                  string `json:"probe_id"`
	NodeID                   string `json:"node_id"`
	RegionID                 string `json:"region_id"`
	ObservedAt               string `json:"observed_at"`
	Endpoint                 string `json:"endpoint"`
	AvailabilityUp           bool   `json:"availability_up"`
	FinalizedLagBlocks       *int   `json:"finalized_lag_blocks,omitempty"`
	ChainLagBlocks           *int   `json:"chain_lag_blocks,omitempty"`
	SourceHeight             *int   `json:"source_height,omitempty"`
	PeerHeight               *int   `json:"peer_height,omitempty"`
	MeasurementWindowSeconds int    `json:"measurement_window_seconds"`
	HTTPStatus               *int   `json:"http_status,omitempty"`
	ErrorCode                string `json:"error_code,omitempty"`
	ResolverIP               string `json:"resolver_ip,omitempty"`
	Notes                    string `json:"notes,omitempty"`
}

type DailyQualificationSummary struct {
	NodeID                           string  `json:"node_id"`
	DateUTC                          string  `json:"date_utc"`
	PolicyVersion                    string  `json:"policy_version"`
	FinalizedLagThresholdBlocks      int     `json:"finalized_lag_threshold_blocks"`
	ChainLagThresholdBlocks          int     `json:"chain_lag_threshold_blocks"`
	ValidProbeCount                  int     `json:"valid_probe_count"`
	AvailabilityUpCount              int     `json:"availability_up_count"`
	AvailabilityRatio                float64 `json:"availability_ratio"`
	FinalizedLagMeasurableCount      int     `json:"finalized_lag_measurable_count"`
	FinalizedLagWithinThresholdCount int     `json:"finalized_lag_within_threshold_count"`
	FinalizedLagRatio                float64 `json:"finalized_lag_ratio"`
	ChainLagMeasurableCount          int     `json:"chain_lag_measurable_count"`
	ChainLagWithinThresholdCount     int     `json:"chain_lag_within_threshold_count"`
	ChainLagRatio                    float64 `json:"chain_lag_ratio"`
	RegionCount                      int     `json:"region_count"`
	AvailabilityPassed               bool    `json:"availability_passed"`
	FinalizedLagPassed               bool    `json:"finalized_lag_passed"`
	ChainLagPassed                   bool    `json:"chain_lag_passed"`
	MultiRegionEvidencePassed        bool    `json:"multi_region_evidence_passed"`
	QualifiedProbeEvidencePassed     bool    `json:"qualified_probe_evidence_passed"`
	InsufficientEvidenceReason       string  `json:"insufficient_evidence_reason,omitempty"`
	GeneratedAt                      string  `json:"generated_at,omitempty"`
}

type QualifiedDecisionRecord struct {
	NodeID                     string   `json:"node_id"`
	DateUTC                    string   `json:"date_utc"`
	PolicyVersion              string   `json:"policy_version"`
	ProbeEvidencePassed        bool     `json:"probe_evidence_passed"`
	HeartbeatPassed            bool     `json:"heartbeat_passed"`
	HardwarePassed             bool     `json:"hardware_passed"`
	VotingKeyPassed            bool     `json:"voting_key_passed"`
	Qualified                  bool     `json:"qualified"`
	FailureReasons             []string `json:"failure_reasons,omitempty"`
	InsufficientEvidenceReason string   `json:"insufficient_evidence_reason,omitempty"`
	DecidedAt                  string   `json:"decided_at"`
}

type BasePerformanceRecord struct {
	NodeID               string  `json:"node_id"`
	DateUTC              string  `json:"date_utc"`
	PolicyVersion        string  `json:"policy_version"`
	AvailabilityScore    float64 `json:"availability_score"`
	FinalizationScore    float64 `json:"finalization_score"`
	ChainSyncScore       float64 `json:"chain_sync_score"`
	VotingKeyScore       float64 `json:"voting_key_score"`
	BasePerformanceScore float64 `json:"base_performance_score"`
	QualifiedDecisionRef string  `json:"qualified_decision_ref"`
	DailySummaryRef      string  `json:"daily_summary_ref"`
	ComputedAt           string  `json:"computed_at"`
}

type RankingRecord struct {
	NodeID                string  `json:"node_id"`
	DateUTC               string  `json:"date_utc"`
	PolicyVersion         string  `json:"policy_version"`
	RankPosition          int     `json:"rank_position"`
	AvailabilityScore     float64 `json:"availability_score"`
	FinalizationScore     float64 `json:"finalization_score"`
	ChainSyncScore        float64 `json:"chain_sync_score"`
	VotingKeyScore        float64 `json:"voting_key_score"`
	BasePerformanceScore  float64 `json:"base_performance_score"`
	DecentralizationScore float64 `json:"decentralization_score"`
	TotalScore            float64 `json:"total_score"`
	OperatorGroupID       string  `json:"operator_group_id"`
	RewardEligible        bool    `json:"reward_eligible"`
	ExclusionReason       string  `json:"exclusion_reason,omitempty"`
	ComputedAt            string  `json:"computed_at"`
}

type RewardEligibilityRecord struct {
	NodeID          string `json:"node_id"`
	DateUTC         string `json:"date_utc"`
	PolicyVersion   string `json:"policy_version"`
	RankPosition    int    `json:"rank_position"`
	Qualified       bool   `json:"qualified"`
	OperatorGroupID string `json:"operator_group_id"`
	RewardEligible  bool   `json:"reward_eligible"`
	ExclusionReason string `json:"exclusion_reason,omitempty"`
	DecidedAt       string `json:"decided_at"`
}

type OperatorGroupEvidence struct {
	EvidenceRef     string `json:"evidence_ref"`
	NodeID          string `json:"node_id"`
	OperatorGroupID string `json:"operator_group_id"`
	ObservedAt      string `json:"observed_at"`
	Source          string `json:"source"`
}

type VotingKeyEvidence struct {
	EvidenceRef            string `json:"evidence_ref"`
	NodeID                 string `json:"node_id"`
	ObservedAt             string `json:"observed_at"`
	CurrentEpoch           int    `json:"current_epoch"`
	VotingKeyPresent       bool   `json:"voting_key_present"`
	VotingKeyValidForEpoch bool   `json:"voting_key_valid_for_epoch"`
	Source                 string `json:"source"`
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
	Nodes                  []Node                      `json:"nodes"`
	EnrollmentChallenges   []EnrollmentChallenge       `json:"enrollment_challenges"`
	CheckEvents            []CheckEvent                `json:"check_events"`
	HeartbeatEvents        []HeartbeatEvent            `json:"heartbeat_events"`
	TelemetryEvents        []TelemetryEvent            `json:"telemetry_events"`
	ProbeEvents            []ProbeEvent                `json:"probe_events"`
	DailySummaries         []DailyQualificationSummary `json:"daily_summaries"`
	QualifiedDecisions     []QualifiedDecisionRecord   `json:"qualified_decisions"`
	BasePerformanceRecords []BasePerformanceRecord     `json:"base_performance_records"`
	RankingRecords         []RankingRecord             `json:"ranking_records"`
	RewardEligibility      []RewardEligibilityRecord   `json:"reward_eligibility"`
	OperatorGroupEvidence  []OperatorGroupEvidence     `json:"operator_group_evidence"`
	VotingKeyEvidence      []VotingKeyEvidence         `json:"voting_key_evidence"`
	LatestTelemetry        []LatestTelemetry           `json:"latest_telemetry"`
	AlertStates            []AlertState                `json:"alert_states"`
	NotificationDeliveries []NotificationDelivery      `json:"notification_deliveries"`
	OperationalEvents      []OperationalEvent          `json:"operational_events"`
}

type Store struct {
	mu                 sync.RWMutex
	nodes              map[string]Node
	challenges         map[string]EnrollmentChallenge
	checkEvents        map[string]CheckEvent
	heartbeatEvents    []HeartbeatEvent
	telemetryEvents    []TelemetryEvent
	probeEvents        map[string]ProbeEvent
	dailySummaries     map[string]DailyQualificationSummary
	qualifiedDecisions map[string]QualifiedDecisionRecord
	basePerformance    map[string]BasePerformanceRecord
	rankingRecords     map[string]RankingRecord
	rewardEligibility  map[string]RewardEligibilityRecord
	operatorGroups     map[string]OperatorGroupEvidence
	votingKeyEvidence  map[string]VotingKeyEvidence
	latestTelemetry    map[string]LatestTelemetry
	alertStates        map[string]AlertState
	deliveries         []NotificationDelivery
	operational        []OperationalEvent
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
		nodes:              nodes,
		challenges:         map[string]EnrollmentChallenge{},
		checkEvents:        map[string]CheckEvent{},
		heartbeatEvents:    nil,
		probeEvents:        map[string]ProbeEvent{},
		dailySummaries:     map[string]DailyQualificationSummary{},
		qualifiedDecisions: map[string]QualifiedDecisionRecord{},
		basePerformance:    map[string]BasePerformanceRecord{},
		rankingRecords:     map[string]RankingRecord{},
		rewardEligibility:  map[string]RewardEligibilityRecord{},
		operatorGroups:     map[string]OperatorGroupEvidence{},
		votingKeyEvidence:  map[string]VotingKeyEvidence{},
		latestTelemetry:    map[string]LatestTelemetry{},
		alertStates:        map[string]AlertState{},
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
		EnrollmentChallenges:   challenges,
		CheckEvents:            checks,
		HeartbeatEvents:        heartbeats,
		TelemetryEvents:        telemetry,
		ProbeEvents:            probes,
		DailySummaries:         dailySummaries,
		QualifiedDecisions:     qualifiedDecisions,
		BasePerformanceRecords: basePerformance,
		RankingRecords:         rankingRecords,
		RewardEligibility:      rewardEligibility,
		OperatorGroupEvidence:  operatorGroupEvidence,
		VotingKeyEvidence:      votingKeyEvidence,
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

func (s *Store) SaveDailyQualificationSummary(summary DailyQualificationSummary) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dailySummaries[dailySummaryKey(summary.NodeID, summary.DateUTC)] = summary
}

func (s *Store) GetDailyQualificationSummary(nodeID, dateUTC string) (DailyQualificationSummary, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	summary, ok := s.dailySummaries[dailySummaryKey(nodeID, dateUTC)]
	return summary, ok
}

func (s *Store) SaveQualifiedDecisionRecord(decision QualifiedDecisionRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.qualifiedDecisions[qualifiedDecisionKey(decision.NodeID, decision.DateUTC)] = decision
}

func (s *Store) GetQualifiedDecisionRecord(nodeID, dateUTC string) (QualifiedDecisionRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	decision, ok := s.qualifiedDecisions[qualifiedDecisionKey(nodeID, dateUTC)]
	return decision, ok
}

func (s *Store) SaveBasePerformanceRecord(record BasePerformanceRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.basePerformance[basePerformanceKey(record.NodeID, record.DateUTC)] = record
}

func (s *Store) DeleteBasePerformanceRecord(nodeID, dateUTC string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.basePerformance, basePerformanceKey(nodeID, dateUTC))
}

func (s *Store) GetBasePerformanceRecord(nodeID, dateUTC string) (BasePerformanceRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	record, ok := s.basePerformance[basePerformanceKey(nodeID, dateUTC)]
	return record, ok
}

func (s *Store) ListBasePerformanceRecordsByDate(dateUTC string) []BasePerformanceRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []BasePerformanceRecord
	for _, record := range s.basePerformance {
		if dateUTC != "" && record.DateUTC != dateUTC {
			continue
		}
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].NodeID < out[j].NodeID
	})
	return out
}

func (s *Store) ReplaceRankingRecordsForDate(dateUTC string, records []RankingRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, record := range s.rankingRecords {
		if record.DateUTC == dateUTC {
			delete(s.rankingRecords, key)
		}
	}
	for _, record := range records {
		s.rankingRecords[rankingRecordKey(record.NodeID, record.DateUTC)] = record
	}
}

func (s *Store) ListRankingRecordsByDate(dateUTC string) []RankingRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []RankingRecord
	for _, record := range s.rankingRecords {
		if dateUTC != "" && record.DateUTC != dateUTC {
			continue
		}
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RankPosition == out[j].RankPosition {
			return out[i].NodeID < out[j].NodeID
		}
		return out[i].RankPosition < out[j].RankPosition
	})
	return out
}

func (s *Store) ReplaceRewardEligibilityRecordsForDate(dateUTC string, records []RewardEligibilityRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, record := range s.rewardEligibility {
		if record.DateUTC == dateUTC {
			delete(s.rewardEligibility, key)
		}
	}
	for _, record := range records {
		s.rewardEligibility[rewardEligibilityKey(record.NodeID, record.DateUTC)] = record
	}
}

func (s *Store) ListRewardEligibilityRecordsByDate(dateUTC string) []RewardEligibilityRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []RewardEligibilityRecord
	for _, record := range s.rewardEligibility {
		if dateUTC != "" && record.DateUTC != dateUTC {
			continue
		}
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RankPosition == out[j].RankPosition {
			return out[i].NodeID < out[j].NodeID
		}
		return out[i].RankPosition < out[j].RankPosition
	})
	return out
}

func (s *Store) SaveOperatorGroupEvidence(evidence OperatorGroupEvidence) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.operatorGroups[evidence.EvidenceRef]; exists {
		return false
	}
	s.operatorGroups[evidence.EvidenceRef] = evidence
	return true
}

func (s *Store) GetLatestOperatorGroupEvidenceForNode(nodeID string) (OperatorGroupEvidence, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest OperatorGroupEvidence
	found := false
	for _, evidence := range s.operatorGroups {
		if evidence.NodeID != nodeID {
			continue
		}
		if !found || evidence.ObservedAt > latest.ObservedAt {
			latest = evidence
			found = true
		}
	}
	return latest, found
}

func (s *Store) GetLatestOperatorGroupEvidenceForNodeAndDate(nodeID, dateUTC string) (OperatorGroupEvidence, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest OperatorGroupEvidence
	found := false
	for _, evidence := range s.operatorGroups {
		if evidence.NodeID != nodeID {
			continue
		}
		if dateUTC != "" && !strings.HasPrefix(evidence.ObservedAt, dateUTC) {
			continue
		}
		if !found || evidence.ObservedAt > latest.ObservedAt {
			latest = evidence
			found = true
		}
	}
	return latest, found
}

func dailySummaryKey(nodeID, dateUTC string) string {
	return nodeID + "\x00" + dateUTC
}

func qualifiedDecisionKey(nodeID, dateUTC string) string {
	return nodeID + "\x00" + dateUTC
}

func basePerformanceKey(nodeID, dateUTC string) string {
	return nodeID + "\x00" + dateUTC
}

func rankingRecordKey(nodeID, dateUTC string) string {
	return nodeID + "\x00" + dateUTC
}

func rewardEligibilityKey(nodeID, dateUTC string) string {
	return nodeID + "\x00" + dateUTC
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

func (s *Store) SaveVotingKeyEvidence(evidence VotingKeyEvidence) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.votingKeyEvidence[evidence.EvidenceRef]; exists {
		return false
	}
	s.votingKeyEvidence[evidence.EvidenceRef] = evidence
	return true
}

func (s *Store) GetLatestVotingKeyEvidenceForNode(nodeID string) (VotingKeyEvidence, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest VotingKeyEvidence
	found := false
	for _, evidence := range s.votingKeyEvidence {
		if evidence.NodeID != nodeID {
			continue
		}
		if !found || evidence.ObservedAt > latest.ObservedAt {
			latest = evidence
			found = true
		}
	}
	return latest, found
}

func (s *Store) GetLatestVotingKeyEvidenceForNodeAndDate(nodeID, dateUTC string) (VotingKeyEvidence, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest VotingKeyEvidence
	found := false
	for _, evidence := range s.votingKeyEvidence {
		if evidence.NodeID != nodeID {
			continue
		}
		if dateUTC != "" && !strings.HasPrefix(evidence.ObservedAt, dateUTC) {
			continue
		}
		if !found || evidence.ObservedAt > latest.ObservedAt {
			latest = evidence
			found = true
		}
	}
	return latest, found
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
