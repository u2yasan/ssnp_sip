package server

import "github.com/u2yasan/ssnp_sip/portal/internal/store"

func rankingIndex(records []store.RankingRecord) map[string]store.RankingRecord {
	out := make(map[string]store.RankingRecord, len(records))
	for _, record := range records {
		out[record.NodeID] = record
	}
	return out
}

func rewardEligibilityIndex(records []store.RewardEligibilityRecord) map[string]store.RewardEligibilityRecord {
	out := make(map[string]store.RewardEligibilityRecord, len(records))
	for _, record := range records {
		out[record.NodeID] = record
	}
	return out
}

func buildPublicNodeStatusView(nodeID, dateUTC string, decision store.QualifiedDecisionRecord, ranking store.RankingRecord, reward store.RewardEligibilityRecord) publicNodeStatusView {
	view := publicNodeStatusView{
		NodeID:         nodeID,
		DateUTC:        dateUTC,
		Qualified:      decision.Qualified,
		RewardEligible: reward.RewardEligible,
		StatusReason:   summarizeStatusReason(decision, reward),
	}
	if ranking.NodeID != "" {
		view.RankPosition = intPtr(ranking.RankPosition)
	}
	return view
}

func buildOperatorNodeStatusView(nodeID, dateUTC string, decision store.QualifiedDecisionRecord, ranking store.RankingRecord, reward store.RewardEligibilityRecord) operatorNodeStatusView {
	view := operatorNodeStatusView{
		NodeID:          nodeID,
		DateUTC:         dateUTC,
		Qualified:       decision.Qualified,
		RewardEligible:  reward.RewardEligible,
		StatusReason:    summarizeStatusReason(decision, reward),
		FailureReasons:  append([]string(nil), decision.FailureReasons...),
		HeartbeatPassed: decision.HeartbeatPassed,
		HardwarePassed:  decision.HardwarePassed,
		VotingKeyPassed: decision.VotingKeyPassed,
	}
	if ranking.NodeID != "" {
		view.RankPosition = intPtr(ranking.RankPosition)
	}
	if reward.NodeID != "" {
		view.OperatorGroupID = reward.OperatorGroupID
		view.ExclusionReason = reward.ExclusionReason
		view.RewardEligible = reward.RewardEligible
	}
	return view
}

func summarizeStatusReason(decision store.QualifiedDecisionRecord, reward store.RewardEligibilityRecord) string {
	if reward.ExclusionReason != "" {
		return reward.ExclusionReason
	}
	if !decision.Qualified {
		if decision.InsufficientEvidenceReason != "" {
			return decision.InsufficientEvidenceReason
		}
		if len(decision.FailureReasons) > 0 {
			return decision.FailureReasons[0]
		}
		return "not_qualified"
	}
	if reward.NodeID == "" {
		return "qualified"
	}
	if reward.RewardEligible {
		return "reward_eligible"
	}
	return "qualified"
}

func intPtr(v int) *int {
	return &v
}
