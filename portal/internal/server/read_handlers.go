package server

import (
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleRankingRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	dateUTC := strings.TrimPrefix(r.URL.Path, "/api/v1/rankings/")
	dateUTC = strings.TrimSpace(dateUTC)
	if _, err := time.Parse("2006-01-02", dateUTC); err != nil {
		writeError(w, http.StatusBadRequest, "missing_required_field", "invalid date_utc")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date_utc": dateUTC,
		"items":    s.store.ListRankingRecordsByDate(dateUTC),
	})
}

func (s *Server) handleRewardEligibilityRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	dateUTC := strings.TrimPrefix(r.URL.Path, "/api/v1/reward-eligibility/")
	dateUTC = strings.TrimSpace(dateUTC)
	if _, err := time.Parse("2006-01-02", dateUTC); err != nil {
		writeError(w, http.StatusBadRequest, "missing_required_field", "invalid date_utc")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date_utc": dateUTC,
		"items":    s.store.ListRewardEligibilityRecordsByDate(dateUTC),
	})
}

func (s *Server) handleAntiConcentrationEvidenceRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	dateUTC := strings.TrimPrefix(r.URL.Path, "/api/v1/anti-concentration-evidence/")
	dateUTC = strings.TrimSpace(dateUTC)
	if _, err := time.Parse("2006-01-02", dateUTC); err != nil {
		writeError(w, http.StatusBadRequest, "missing_required_field", "invalid date_utc")
		return
	}
	items := make([]antiConcentrationEvidenceView, 0)
	for _, node := range s.store.ListNodes() {
		view := antiConcentrationEvidenceView{
			NodeID:  node.NodeID,
			DateUTC: dateUTC,
		}
		if evidence, ok := s.store.GetLatestOperatorGroupEvidenceForNodeAndDate(node.NodeID, dateUTC); ok {
			view.OperatorGroupID = evidence.OperatorGroupID
		}
		if evidence, ok := s.store.GetLatestDomainEvidenceForNodeAndDate(node.NodeID, dateUTC); ok {
			view.RegistrableDomain = evidence.RegistrableDomain
		}
		if evidence, ok := s.store.GetLatestSharedControlPlaneEvidenceForNodeAndDate(node.NodeID, dateUTC); ok {
			view.SharedControlPlaneID = evidence.ControlPlaneID
			view.SharedControlClassification = evidence.Classification
		}
		if view.OperatorGroupID == "" && view.RegistrableDomain == "" && view.SharedControlPlaneID == "" {
			continue
		}
		items = append(items, view)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date_utc": dateUTC,
		"items":    items,
	})
}

func (s *Server) handleRewardAllocationRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	dateUTC := strings.TrimPrefix(r.URL.Path, "/api/v1/reward-allocations/")
	dateUTC = strings.TrimSpace(dateUTC)
	if _, err := time.Parse("2006-01-02", dateUTC); err != nil {
		writeError(w, http.StatusBadRequest, "missing_required_field", "invalid date_utc")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date_utc": dateUTC,
		"items":    s.store.ListRewardAllocationRecordsByDate(dateUTC),
	})
}

func (s *Server) handlePublicNodeStatusRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	dateUTC := strings.TrimPrefix(r.URL.Path, "/api/v1/public-node-status/")
	dateUTC = strings.TrimSpace(dateUTC)
	if _, err := time.Parse("2006-01-02", dateUTC); err != nil {
		writeError(w, http.StatusBadRequest, "missing_required_field", "invalid date_utc")
		return
	}
	items := make([]publicNodeStatusView, 0)
	rankingByNode := rankingIndex(s.store.ListRankingRecordsByDate(dateUTC))
	rewardByNode := rewardEligibilityIndex(s.store.ListRewardEligibilityRecordsByDate(dateUTC))
	for _, node := range s.store.ListNodes() {
		decision, ok := s.store.GetQualifiedDecisionRecord(node.NodeID, dateUTC)
		if !ok {
			continue
		}
		items = append(items, buildPublicNodeStatusView(node.NodeID, dateUTC, decision, rankingByNode[node.NodeID], rewardByNode[node.NodeID]))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date_utc": dateUTC,
		"items":    items,
	})
}

func (s *Server) handleOperatorNodeStatusRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/operator-node-status/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		writeError(w, http.StatusBadRequest, "missing_required_field", "missing node_id or date_utc")
		return
	}
	nodeID := strings.TrimSpace(parts[0])
	dateUTC := strings.TrimSpace(parts[1])
	if _, err := time.Parse("2006-01-02", dateUTC); err != nil {
		writeError(w, http.StatusBadRequest, "missing_required_field", "invalid date_utc")
		return
	}
	if _, ok := s.store.GetNode(nodeID); !ok {
		writeError(w, http.StatusNotFound, "unknown_node_id", "unknown node")
		return
	}
	decision, ok := s.store.GetQualifiedDecisionRecord(nodeID, dateUTC)
	if !ok {
		writeError(w, http.StatusNotFound, "missing_qualified_decision", "qualified decision not found")
		return
	}
	rankingByNode := rankingIndex(s.store.ListRankingRecordsByDate(dateUTC))
	rewardByNode := rewardEligibilityIndex(s.store.ListRewardEligibilityRecordsByDate(dateUTC))
	writeJSON(w, http.StatusOK, buildOperatorNodeStatusView(nodeID, dateUTC, decision, rankingByNode[nodeID], rewardByNode[nodeID]))
}
