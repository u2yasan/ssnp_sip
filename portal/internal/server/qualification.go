package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/store"
)

func (s *Server) computeDailyQualificationSummary(nodeID, dateUTC string) store.DailyQualificationSummary {
	events := s.store.ListProbeEventsByNodeAndDate(nodeID, dateUTC)
	regionSet := map[string]struct{}{}
	validProbeCount := len(events)
	availabilityUpCount := 0
	finalizedLagMeasurableCount := 0
	finalizedLagWithinThresholdCount := 0
	chainLagMeasurableCount := 0
	chainLagWithinThresholdCount := 0

	for _, event := range events {
		regionSet[event.RegionID] = struct{}{}
		if event.AvailabilityUp {
			availabilityUpCount++
		}
		if event.AvailabilityUp && event.FinalizedLagBlocks != nil {
			finalizedLagMeasurableCount++
			if *event.FinalizedLagBlocks <= s.policy.ProbeThresholds.FinalizedLagMaxBlocks {
				finalizedLagWithinThresholdCount++
			}
		}
		if event.AvailabilityUp && event.ChainLagBlocks != nil {
			chainLagMeasurableCount++
			if *event.ChainLagBlocks <= s.policy.ProbeThresholds.ChainLagMaxBlocks {
				chainLagWithinThresholdCount++
			}
		}
	}

	availabilityRatio := ratio(availabilityUpCount, validProbeCount)
	finalizedLagRatio := ratio(finalizedLagWithinThresholdCount, finalizedLagMeasurableCount)
	chainLagRatio := ratio(chainLagWithinThresholdCount, chainLagMeasurableCount)
	regionCount := len(regionSet)

	availabilityPassed := validProbeCount > 0 && availabilityRatio >= 0.99
	finalizedLagPassed := finalizedLagMeasurableCount > 0 && finalizedLagRatio >= 0.95
	chainLagPassed := chainLagMeasurableCount > 0 && chainLagRatio >= 0.95
	multiRegionEvidencePassed := regionCount >= 2

	insufficientEvidenceReason := ""
	switch {
	case validProbeCount == 0:
		insufficientEvidenceReason = "no_valid_probes"
	case regionCount < 2:
		insufficientEvidenceReason = "insufficient_probe_regions"
	case finalizedLagMeasurableCount == 0:
		insufficientEvidenceReason = "missing_finalized_lag_evidence"
	case chainLagMeasurableCount == 0:
		insufficientEvidenceReason = "missing_chain_lag_evidence"
	}

	return store.DailyQualificationSummary{
		NodeID:                           nodeID,
		DateUTC:                          dateUTC,
		PolicyVersion:                    s.policy.PolicyVersion,
		FinalizedLagThresholdBlocks:      s.policy.ProbeThresholds.FinalizedLagMaxBlocks,
		ChainLagThresholdBlocks:          s.policy.ProbeThresholds.ChainLagMaxBlocks,
		ValidProbeCount:                  validProbeCount,
		AvailabilityUpCount:              availabilityUpCount,
		AvailabilityRatio:                availabilityRatio,
		FinalizedLagMeasurableCount:      finalizedLagMeasurableCount,
		FinalizedLagWithinThresholdCount: finalizedLagWithinThresholdCount,
		FinalizedLagRatio:                finalizedLagRatio,
		ChainLagMeasurableCount:          chainLagMeasurableCount,
		ChainLagWithinThresholdCount:     chainLagWithinThresholdCount,
		ChainLagRatio:                    chainLagRatio,
		RegionCount:                      regionCount,
		AvailabilityPassed:               availabilityPassed,
		FinalizedLagPassed:               finalizedLagPassed,
		ChainLagPassed:                   chainLagPassed,
		MultiRegionEvidencePassed:        multiRegionEvidencePassed,
		QualifiedProbeEvidencePassed:     insufficientEvidenceReason == "" && availabilityPassed && finalizedLagPassed && chainLagPassed && multiRegionEvidencePassed,
		InsufficientEvidenceReason:       insufficientEvidenceReason,
		GeneratedAt:                      time.Now().UTC().Format(time.RFC3339),
	}
}

func ratio(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func datePart(timestamp string) (string, error) {
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return "", err
	}
	return ts.UTC().Format("2006-01-02"), nil
}

func evaluationAnchor(dateUTC string) time.Time {
	endOfDay, err := time.Parse(time.RFC3339, dateUTC+"T23:59:59Z")
	if err != nil {
		return time.Now().UTC()
	}
	now := time.Now().UTC()
	if now.Before(endOfDay) {
		return now
	}
	return endOfDay
}

func countHeartbeatsWithinWindow(events []store.HeartbeatEvent, anchor time.Time, window time.Duration) int {
	count := 0
	for _, event := range events {
		ts, err := time.Parse(time.RFC3339, event.HeartbeatTimestamp)
		if err != nil || ts.After(anchor) {
			continue
		}
		if anchor.Sub(ts) <= window {
			count++
		}
	}
	return count
}

func (s *Server) updateQualificationArtifacts(nodeID, dateUTC string) {
	summary := s.computeDailyQualificationSummary(nodeID, dateUTC)
	s.store.SaveDailyQualificationSummary(summary)
	decision := s.computeQualifiedDecisionRecord(nodeID, summary)
	s.store.SaveQualifiedDecisionRecord(decision)
	if record, ok := s.computeBasePerformanceRecord(summary, decision); ok {
		s.store.SaveBasePerformanceRecord(record)
	} else {
		s.store.DeleteBasePerformanceRecord(nodeID, dateUTC)
	}
	s.rebuildRankings(dateUTC)
	now := time.Now().UTC()
	if summary.ValidProbeCount > 0 && !summary.AvailabilityPassed {
		_ = s.maybeNotifyAlert(context.Background(), nodeID, alertNodeOutage, now.Format(time.RFC3339), now)
	}
	if summary.FinalizedLagMeasurableCount > 0 && !summary.FinalizedLagPassed {
		_ = s.maybeNotifyAlert(context.Background(), nodeID, alertFinalizedLag, now.Format(time.RFC3339), now)
	}
}

func (s *Server) computeQualifiedDecisionRecord(nodeID string, summary store.DailyQualificationSummary) store.QualifiedDecisionRecord {
	node, _ := s.store.GetNode(nodeID)
	failureReasons := []string{}

	probeEvidencePassed := summary.QualifiedProbeEvidencePassed
	if !probeEvidencePassed {
		if summary.InsufficientEvidenceReason != "" {
			failureReasons = append(failureReasons, "insufficient_probe_evidence")
		} else {
			failureReasons = append(failureReasons, "probe_evidence_failed")
		}
	}

	anchor := evaluationAnchor(summary.DateUTC)
	heartbeatEvents := s.store.ListHeartbeatEventsByNode(nodeID)
	healthyHeartbeatCount := countHeartbeatsWithinWindow(heartbeatEvents, anchor, healthyHeartbeatWindow)
	staleHeartbeatCount := countHeartbeatsWithinWindow(heartbeatEvents, anchor, failedHeartbeatWindow)
	heartbeatPassed := healthyHeartbeatCount >= 2
	if !heartbeatPassed {
		switch {
		case staleHeartbeatCount > 0:
			failureReasons = append(failureReasons, "heartbeat_stale")
		default:
			failureReasons = append(failureReasons, "heartbeat_missing")
		}
	}

	hardwarePassed := false
	if latestCheck, ok := s.store.LatestCheckEventForNodeAndDate(nodeID, summary.DateUTC); !ok {
		failureReasons = append(failureReasons, "hardware_check_missing")
	} else if strings.TrimSpace(latestCheck.CheckedAt) == "" {
		failureReasons = append(failureReasons, "hardware_check_missing")
	} else if !latestCheck.OverallPassed {
		failureReasons = append(failureReasons, "hardware_check_failed")
	} else {
		hardwarePassed = true
	}

	votingKeyPassed := false
	if latestEvidence, ok := s.store.GetLatestVotingKeyEvidenceForNodeAndDate(nodeID, summary.DateUTC); !ok {
		failureReasons = append(failureReasons, "voting_key_evidence_missing")
	} else if !latestEvidence.VotingKeyPresent {
		failureReasons = append(failureReasons, "voting_key_not_present")
	} else if !latestEvidence.VotingKeyValidForEpoch {
		failureReasons = append(failureReasons, "voting_key_invalid")
	} else {
		votingKeyPassed = true
	}

	observationWindowPassed := false
	registrationTime, ok := parseRFC3339(node.ValidatedRegistrationAt)
	if ok && !anchor.Before(registrationTime.Add(observationWindow)) {
		observationWindowPassed = true
	} else {
		failureReasons = append(failureReasons, "observation_window_incomplete")
	}

	return store.QualifiedDecisionRecord{
		NodeID:                     nodeID,
		DateUTC:                    summary.DateUTC,
		PolicyVersion:              summary.PolicyVersion,
		ProbeEvidencePassed:        probeEvidencePassed,
		HeartbeatPassed:            heartbeatPassed,
		HardwarePassed:             hardwarePassed,
		VotingKeyPassed:            votingKeyPassed,
		Qualified:                  probeEvidencePassed && heartbeatPassed && hardwarePassed && votingKeyPassed && observationWindowPassed,
		FailureReasons:             failureReasons,
		InsufficientEvidenceReason: summary.InsufficientEvidenceReason,
		DecidedAt:                  time.Now().UTC().Format(time.RFC3339),
	}
}

func (s *Server) computeBasePerformanceRecord(summary store.DailyQualificationSummary, decision store.QualifiedDecisionRecord) (store.BasePerformanceRecord, bool) {
	if !decision.Qualified {
		return store.BasePerformanceRecord{}, false
	}
	availabilityScore := clampScore(30 * summary.AvailabilityRatio)
	finalizationScore := clampScore(20 * summary.FinalizedLagRatio)
	chainSyncScore := clampScore(10 * summary.ChainLagRatio)
	votingKeyScore := 0.0
	if decision.VotingKeyPassed {
		votingKeyScore = 10
	}
	basePerformanceScore := availabilityScore + finalizationScore + chainSyncScore + votingKeyScore
	return store.BasePerformanceRecord{
		NodeID:               summary.NodeID,
		DateUTC:              summary.DateUTC,
		PolicyVersion:        summary.PolicyVersion,
		AvailabilityScore:    availabilityScore,
		FinalizationScore:    finalizationScore,
		ChainSyncScore:       chainSyncScore,
		VotingKeyScore:       votingKeyScore,
		BasePerformanceScore: basePerformanceScore,
		QualifiedDecisionRef: summary.NodeID + ":" + summary.DateUTC,
		DailySummaryRef:      summary.NodeID + ":" + summary.DateUTC,
		ComputedAt:           time.Now().UTC().Format(time.RFC3339),
	}, true
}

func (s *Server) rebuildRankings(dateUTC string) {
	records := s.store.ListBasePerformanceRecordsByDate(dateUTC)
	decentralizationScores := s.computeDecentralizationScores(dateUTC, records)
	sort.Slice(records, func(i, j int) bool {
		leftTotal := totalScore(records[i].BasePerformanceScore, decentralizationScores[records[i].NodeID])
		rightTotal := totalScore(records[j].BasePerformanceScore, decentralizationScores[records[j].NodeID])
		if leftTotal == rightTotal {
			if records[i].BasePerformanceScore == records[j].BasePerformanceScore {
				if records[i].FinalizationScore == records[j].FinalizationScore {
					if records[i].AvailabilityScore == records[j].AvailabilityScore {
						iNode, _ := s.store.GetNode(records[i].NodeID)
						jNode, _ := s.store.GetNode(records[j].NodeID)
						if registrationTimeLess(iNode.ValidatedRegistrationAt, jNode.ValidatedRegistrationAt) {
							return true
						}
						if registrationTimeLess(jNode.ValidatedRegistrationAt, iNode.ValidatedRegistrationAt) {
							return false
						}
						return records[i].NodeID < records[j].NodeID
					}
					return records[i].AvailabilityScore > records[j].AvailabilityScore
				}
				return records[i].FinalizationScore > records[j].FinalizationScore
			}
			return records[i].BasePerformanceScore > records[j].BasePerformanceScore
		}
		return leftTotal > rightTotal
	})
	rankings := make([]store.RankingRecord, 0, len(records))
	computedAt := time.Now().UTC().Format(time.RFC3339)
	for i, record := range records {
		operatorGroupID := record.NodeID
		if evidence, ok := s.store.GetLatestOperatorGroupEvidenceForNodeAndDate(record.NodeID, dateUTC); ok && strings.TrimSpace(evidence.OperatorGroupID) != "" {
			operatorGroupID = evidence.OperatorGroupID
		}
		dScore := decentralizationScores[record.NodeID]
		rankings = append(rankings, store.RankingRecord{
			NodeID:                record.NodeID,
			DateUTC:               record.DateUTC,
			PolicyVersion:         record.PolicyVersion,
			RankPosition:          i + 1,
			AvailabilityScore:     record.AvailabilityScore,
			FinalizationScore:     record.FinalizationScore,
			ChainSyncScore:        record.ChainSyncScore,
			VotingKeyScore:        record.VotingKeyScore,
			BasePerformanceScore:  record.BasePerformanceScore,
			DecentralizationScore: dScore,
			TotalScore:            totalScore(record.BasePerformanceScore, dScore),
			OperatorGroupID:       operatorGroupID,
			RewardEligible:        true,
			ComputedAt:            computedAt,
		})
	}
	s.store.ReplaceRankingRecordsForDate(dateUTC, rankings)
	s.rebuildRewardEligibility(dateUTC, rankings)
}

func (s *Server) computeDecentralizationScores(dateUTC string, records []store.BasePerformanceRecord) map[string]float64 {
	scores := make(map[string]float64, len(records))
	countryCounts := map[string]int{}
	infraCounts := map[string]int{}
	for _, record := range records {
		evidence, ok := s.store.GetLatestDecentralizationEvidenceForNodeAndDate(record.NodeID, dateUTC)
		if !ok {
			continue
		}
		if evidence.CountryCode != "" {
			countryCounts[evidence.CountryCode]++
		}
		infraKey := fmt.Sprintf("%d|%s|%s", evidence.ASN, evidence.ProviderID, evidence.InfrastructureID)
		infraCounts[infraKey]++
	}
	for _, record := range records {
		evidence, ok := s.store.GetLatestDecentralizationEvidenceForNodeAndDate(record.NodeID, dateUTC)
		if !ok {
			scores[record.NodeID] = 0
			continue
		}
		score := 0.0
		if evidence.CountryCode != "" && countryCounts[evidence.CountryCode] == 1 {
			score += 15
			score += 5
		}
		infraKey := fmt.Sprintf("%d|%s|%s", evidence.ASN, evidence.ProviderID, evidence.InfrastructureID)
		if evidence.ASN > 0 && strings.TrimSpace(evidence.ProviderID) != "" && infraCounts[infraKey] == 1 {
			score += 10
		}
		scores[record.NodeID] = score
	}
	return scores
}

func totalScore(basePerformanceScore, decentralizationScore float64) float64 {
	return clampScore((0.7 * basePerformanceScore) + (0.3 * decentralizationScore))
}

func registrationTimeLess(left, right string) bool {
	leftTime, leftOK := parseRFC3339(left)
	rightTime, rightOK := parseRFC3339(right)
	switch {
	case leftOK && rightOK:
		return leftTime.Before(rightTime)
	case leftOK:
		return true
	case rightOK:
		return false
	default:
		return false
	}
}

func parseRFC3339(raw string) (time.Time, bool) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func (s *Server) rebuildRewardEligibility(dateUTC string, rankings []store.RankingRecord) {
	records := make([]store.RewardEligibilityRecord, 0, len(rankings))
	decidedAt := time.Now().UTC().Format(time.RFC3339)
	seenGroups := map[string]struct{}{}
	seenDomains := map[string]struct{}{}
	seenControlPlanes := map[string]struct{}{}
	for _, ranking := range rankings {
		operatorGroupID := ranking.NodeID
		if evidence, ok := s.store.GetLatestOperatorGroupEvidenceForNodeAndDate(ranking.NodeID, dateUTC); ok && strings.TrimSpace(evidence.OperatorGroupID) != "" {
			operatorGroupID = evidence.OperatorGroupID
		} else if ranking.OperatorGroupID != "" {
			operatorGroupID = ranking.OperatorGroupID
		}
		rewardEligible := true
		exclusionReason := ""
		excludedOperatorGroupID := ""
		excludedRegistrableDomain := ""
		excludedControlPlaneID := ""
		excludedClassification := ""
		if _, exists := seenGroups[operatorGroupID]; exists {
			rewardEligible = false
			exclusionReason = "same_operator_group_lower_ranked"
			excludedOperatorGroupID = operatorGroupID
		} else {
			seenGroups[operatorGroupID] = struct{}{}
		}
		if evidence, ok := s.store.GetLatestDomainEvidenceForNodeAndDate(ranking.NodeID, dateUTC); ok && strings.TrimSpace(evidence.RegistrableDomain) != "" {
			domain := strings.ToLower(strings.TrimSpace(evidence.RegistrableDomain))
			if _, exists := seenDomains[domain]; exists && exclusionReason == "" {
				rewardEligible = false
				exclusionReason = "same_registrable_domain_lower_ranked"
				excludedRegistrableDomain = domain
			} else {
				seenDomains[domain] = struct{}{}
			}
		}
		if evidence, ok := s.store.GetLatestSharedControlPlaneEvidenceForNodeAndDate(ranking.NodeID, dateUTC); ok && strings.TrimSpace(evidence.ControlPlaneID) != "" {
			controlPlaneID := strings.TrimSpace(evidence.ControlPlaneID)
			if _, exists := seenControlPlanes[controlPlaneID]; exists && exclusionReason == "" {
				rewardEligible = false
				exclusionReason = "same_shared_control_plane_lower_ranked"
				excludedControlPlaneID = controlPlaneID
				excludedClassification = evidence.Classification
			} else {
				seenControlPlanes[controlPlaneID] = struct{}{}
			}
		}
		records = append(records, store.RewardEligibilityRecord{
			NodeID:                    ranking.NodeID,
			DateUTC:                   dateUTC,
			PolicyVersion:             ranking.PolicyVersion,
			RankPosition:              ranking.RankPosition,
			Qualified:                 true,
			OperatorGroupID:           operatorGroupID,
			RewardEligible:            rewardEligible,
			ExclusionReason:           exclusionReason,
			ExcludedOperatorGroupID:   excludedOperatorGroupID,
			ExcludedRegistrableDomain: excludedRegistrableDomain,
			ExcludedControlPlaneID:    excludedControlPlaneID,
			ExcludedClassification:    excludedClassification,
			DecidedAt:                 decidedAt,
		})
	}
	s.store.ReplaceRewardEligibilityRecordsForDate(dateUTC, records)
	s.rebuildRewardAllocations(dateUTC, rankings, records)
}

func (s *Server) rebuildRewardAllocations(dateUTC string, rankings []store.RankingRecord, eligibility []store.RewardEligibilityRecord) {
	if len(rankings) == 0 {
		s.store.ReplaceRewardAllocationRecordsForDate(dateUTC, nil)
		return
	}
	eligibleByNode := rewardEligibilityIndex(eligibility)
	qualifiedNodeCount := len(rankings)
	participationRate := rewardParticipationRate(qualifiedNodeCount)
	distributedPool := s.cfg.NominalDailyPool * participationRate
	reservePool := s.cfg.NominalDailyPool - distributedPool
	computedAt := time.Now().UTC().Format(time.RFC3339)
	allocations := make([]store.RewardAllocationRecord, 0, len(rankings))

	type rewardBand struct {
		label string
		start int
		end   int
		share float64
	}
	bands := []rewardBand{
		{label: "1-5", start: 1, end: 5, share: 0.25},
		{label: "6-20", start: 6, end: 20, share: 0.35},
		{label: "21-50", start: 21, end: 50, share: 0.25},
		{label: "51+", start: 51, end: 1 << 30, share: 0.15},
	}
	for _, band := range bands {
		bandPoolAmount := distributedPool * band.share
		bandRankings := make([]store.RankingRecord, 0)
		for _, ranking := range rankings {
			if ranking.RankPosition >= band.start && ranking.RankPosition <= band.end {
				bandRankings = append(bandRankings, ranking)
			}
		}
		if len(bandRankings) == 0 {
			continue
		}
		bandEligibleCount := 0
		for _, ranking := range bandRankings {
			if eligibleByNode[ranking.NodeID].RewardEligible {
				bandEligibleCount++
			}
		}
		rewardAmount := 0.0
		if bandEligibleCount > 0 {
			rewardAmount = bandPoolAmount / float64(bandEligibleCount)
		}
		for _, ranking := range bandRankings {
			eligibilityRecord := eligibleByNode[ranking.NodeID]
			nodeRewardAmount := 0.0
			if eligibilityRecord.RewardEligible {
				nodeRewardAmount = rewardAmount
			}
			allocations = append(allocations, store.RewardAllocationRecord{
				NodeID:             ranking.NodeID,
				DateUTC:            dateUTC,
				PolicyVersion:      ranking.PolicyVersion,
				RankPosition:       ranking.RankPosition,
				QualifiedNodeCount: qualifiedNodeCount,
				NominalDailyPool:   s.cfg.NominalDailyPool,
				ParticipationRate:  participationRate,
				DistributedPool:    distributedPool,
				ReservePool:        reservePool,
				BandLabel:          band.label,
				BandShare:          band.share,
				BandPoolAmount:     bandPoolAmount,
				BandEligibleCount:  bandEligibleCount,
				RewardAmount:       nodeRewardAmount,
				RewardEligible:     eligibilityRecord.RewardEligible,
				ExclusionReason:    eligibilityRecord.ExclusionReason,
				ComputedAt:         computedAt,
			})
		}
	}
	s.store.ReplaceRewardAllocationRecordsForDate(dateUTC, allocations)
}

func rewardParticipationRate(qualifiedNodeCount int) float64 {
	switch {
	case qualifiedNodeCount <= 0:
		return 0
	case qualifiedNodeCount < 25:
		return 0.30
	case qualifiedNodeCount < 50:
		return 0.50
	case qualifiedNodeCount < 100:
		return 0.75
	default:
		return 1.0
	}
}

func clampScore(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 100:
		return 100
	default:
		return value
	}
}
