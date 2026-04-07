package store

import (
	"sort"
	"strings"
)

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

func rewardAllocationKey(nodeID, dateUTC string) string {
	return nodeID + "\x00" + dateUTC
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

func (s *Store) ReplaceRewardAllocationRecordsForDate(dateUTC string, records []RewardAllocationRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, record := range s.rewardAllocations {
		if record.DateUTC == dateUTC {
			delete(s.rewardAllocations, key)
		}
	}
	for _, record := range records {
		s.rewardAllocations[rewardAllocationKey(record.NodeID, record.DateUTC)] = record
	}
}

func (s *Store) ListRewardAllocationRecordsByDate(dateUTC string) []RewardAllocationRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []RewardAllocationRecord
	for _, record := range s.rewardAllocations {
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
	if evidence.ReviewState == "" {
		evidence.ReviewState = "accepted"
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
	if found && latest.ReviewState == "" {
		latest.ReviewState = "accepted"
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
	if found && latest.ReviewState == "" {
		latest.ReviewState = "accepted"
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

func (s *Store) SaveDecentralizationEvidence(evidence DecentralizationEvidence) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.decentralization[evidence.EvidenceRef]; exists {
		return false
	}
	s.decentralization[evidence.EvidenceRef] = evidence
	return true
}

func (s *Store) GetLatestDecentralizationEvidenceForNodeAndDate(nodeID, dateUTC string) (DecentralizationEvidence, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest DecentralizationEvidence
	found := false
	for _, evidence := range s.decentralization {
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

func (s *Store) SaveDomainEvidence(evidence DomainEvidence) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.domainEvidence[evidence.EvidenceRef]; exists {
		return false
	}
	s.domainEvidence[evidence.EvidenceRef] = evidence
	return true
}

func (s *Store) GetLatestDomainEvidenceForNodeAndDate(nodeID, dateUTC string) (DomainEvidence, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest DomainEvidence
	found := false
	for _, evidence := range s.domainEvidence {
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

func (s *Store) SaveSharedControlPlaneEvidence(evidence SharedControlPlaneEvidence) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.controlPlane[evidence.EvidenceRef]; exists {
		return false
	}
	if evidence.ReviewState == "" {
		evidence.ReviewState = "accepted"
	}
	s.controlPlane[evidence.EvidenceRef] = evidence
	return true
}

func (s *Store) GetLatestSharedControlPlaneEvidenceForNodeAndDate(nodeID, dateUTC string) (SharedControlPlaneEvidence, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest SharedControlPlaneEvidence
	found := false
	for _, evidence := range s.controlPlane {
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
	if found && latest.ReviewState == "" {
		latest.ReviewState = "accepted"
	}
	return latest, found
}
