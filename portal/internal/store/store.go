package store

import "sync"

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
	rewardAllocations  map[string]RewardAllocationRecord
	operatorGroups     map[string]OperatorGroupEvidence
	votingKeyEvidence  map[string]VotingKeyEvidence
	decentralization   map[string]DecentralizationEvidence
	domainEvidence     map[string]DomainEvidence
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
		rewardAllocations:  map[string]RewardAllocationRecord{},
		operatorGroups:     map[string]OperatorGroupEvidence{},
		votingKeyEvidence:  map[string]VotingKeyEvidence{},
		decentralization:   map[string]DecentralizationEvidence{},
		domainEvidence:     map[string]DomainEvidence{},
		latestTelemetry:    map[string]LatestTelemetry{},
		alertStates:        map[string]AlertState{},
	}
}
