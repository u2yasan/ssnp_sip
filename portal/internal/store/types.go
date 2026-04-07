package store

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
	NodeID                    string `json:"node_id"`
	DateUTC                   string `json:"date_utc"`
	PolicyVersion             string `json:"policy_version"`
	RankPosition              int    `json:"rank_position"`
	Qualified                 bool   `json:"qualified"`
	OperatorGroupID           string `json:"operator_group_id"`
	RewardEligible            bool   `json:"reward_eligible"`
	ExclusionReason           string `json:"exclusion_reason,omitempty"`
	ExcludedOperatorGroupID   string `json:"excluded_operator_group_id,omitempty"`
	ExcludedRegistrableDomain string `json:"excluded_registrable_domain,omitempty"`
	ExcludedControlPlaneID    string `json:"excluded_control_plane_id,omitempty"`
	ExcludedClassification    string `json:"excluded_classification,omitempty"`
	DecidedAt                 string `json:"decided_at"`
}

type RewardAllocationRecord struct {
	NodeID             string  `json:"node_id"`
	DateUTC            string  `json:"date_utc"`
	PolicyVersion      string  `json:"policy_version"`
	RankPosition       int     `json:"rank_position"`
	QualifiedNodeCount int     `json:"qualified_node_count"`
	NominalDailyPool   float64 `json:"nominal_daily_pool"`
	ParticipationRate  float64 `json:"participation_rate"`
	DistributedPool    float64 `json:"distributed_pool"`
	ReservePool        float64 `json:"reserve_pool"`
	BandLabel          string  `json:"band_label"`
	BandShare          float64 `json:"band_share"`
	BandPoolAmount     float64 `json:"band_pool_amount"`
	BandEligibleCount  int     `json:"band_eligible_count"`
	RewardAmount       float64 `json:"reward_amount"`
	RewardEligible     bool    `json:"reward_eligible"`
	ExclusionReason    string  `json:"exclusion_reason,omitempty"`
	ComputedAt         string  `json:"computed_at"`
}

type OperatorGroupEvidence struct {
	EvidenceRef     string `json:"evidence_ref"`
	NodeID          string `json:"node_id"`
	OperatorGroupID string `json:"operator_group_id"`
	ObservedAt      string `json:"observed_at"`
	Source          string `json:"source"`
	ReviewState     string `json:"review_state"`
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

type DecentralizationEvidence struct {
	EvidenceRef      string `json:"evidence_ref"`
	NodeID           string `json:"node_id"`
	ObservedAt       string `json:"observed_at"`
	CountryCode      string `json:"country_code"`
	ASN              int    `json:"asn"`
	ProviderID       string `json:"provider_id"`
	InfrastructureID string `json:"infrastructure_id,omitempty"`
	Source           string `json:"source"`
}

type DomainEvidence struct {
	EvidenceRef       string `json:"evidence_ref"`
	NodeID            string `json:"node_id"`
	ObservedAt        string `json:"observed_at"`
	RegistrableDomain string `json:"registrable_domain"`
	Source            string `json:"source"`
}

type SharedControlPlaneEvidence struct {
	EvidenceRef    string `json:"evidence_ref"`
	NodeID         string `json:"node_id"`
	ObservedAt     string `json:"observed_at"`
	ControlPlaneID string `json:"control_plane_id"`
	Classification string `json:"classification"`
	Source         string `json:"source"`
	ReviewState    string `json:"review_state"`
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
