package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/store"
)

func (s *Server) handleVotingKeyEvidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	payload, err := decodeObject(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	nodeID := stringField(payload, "node_id")
	evidenceRef := stringField(payload, "evidence_ref")
	observedAt := stringField(payload, "observed_at")
	source := stringField(payload, "source")
	if nodeID == "" || evidenceRef == "" || observedAt == "" || source == "" {
		writeError(w, http.StatusBadRequest, "missing_required_field", "missing required field")
		return
	}
	if _, ok := s.store.GetNode(nodeID); !ok {
		writeError(w, http.StatusNotFound, "unknown_node_id", "unknown node")
		return
	}
	if err := s.validateTimestamp(observedAt); err != nil {
		writeError(w, http.StatusBadRequest, "stale_timestamp", err.Error())
		return
	}
	currentEpoch, ok := intField(payload, "current_epoch")
	if !ok || currentEpoch < 0 {
		writeError(w, http.StatusBadRequest, "missing_required_field", "missing current_epoch")
		return
	}
	votingKeyPresent, ok := payload["voting_key_present"].(bool)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing_required_field", "missing voting_key_present")
		return
	}
	votingKeyValidForEpoch, ok := payload["voting_key_valid_for_epoch"].(bool)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing_required_field", "missing voting_key_valid_for_epoch")
		return
	}
	if !s.store.SaveVotingKeyEvidence(store.VotingKeyEvidence{
		EvidenceRef:            evidenceRef,
		NodeID:                 nodeID,
		ObservedAt:             observedAt,
		CurrentEpoch:           currentEpoch,
		VotingKeyPresent:       votingKeyPresent,
		VotingKeyValidForEpoch: votingKeyValidForEpoch,
		Source:                 source,
	}) {
		writeError(w, http.StatusConflict, "duplicate_evidence_ref", "duplicate evidence_ref")
		return
	}
	if dateUTC, err := datePart(observedAt); err == nil {
		s.updateQualificationArtifacts(nodeID, dateUTC)
	}
	if err := s.persist(); err != nil {
		writeError(w, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "accepted",
		"evidence_ref": evidenceRef,
		"node_id":      nodeID,
		"received_at":  time.Now().UTC().Format(time.RFC3339),
	})
}

func normalizeEvidenceSource(raw string) string {
	return strings.TrimSpace(raw)
}

func normalizeReviewState(raw string) string {
	reviewState := strings.ToLower(strings.TrimSpace(raw))
	if reviewState == "" {
		return "accepted"
	}
	return reviewState
}

func validEvidenceSource(source string) bool {
	switch source {
	case "manual_review", "probe_derived", "operator_submitted":
		return true
	default:
		return false
	}
}

func validReviewState(reviewState string) bool {
	switch reviewState {
	case "pending", "accepted", "rejected":
		return true
	default:
		return false
	}
}

func (s *Server) handleOperatorGroupEvidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	payload, err := decodeObject(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	nodeID := stringField(payload, "node_id")
	evidenceRef := stringField(payload, "evidence_ref")
	operatorGroupID := stringField(payload, "operator_group_id")
	observedAt := stringField(payload, "observed_at")
	source := normalizeEvidenceSource(stringField(payload, "source"))
	reviewState := normalizeReviewState(stringField(payload, "review_state"))
	if nodeID == "" || evidenceRef == "" || operatorGroupID == "" || observedAt == "" || source == "" {
		writeError(w, http.StatusBadRequest, "missing_required_field", "missing required field")
		return
	}
	if !validEvidenceSource(source) {
		writeError(w, http.StatusBadRequest, "invalid_source", "invalid source")
		return
	}
	if !validReviewState(reviewState) {
		writeError(w, http.StatusBadRequest, "invalid_review_state", "invalid review_state")
		return
	}
	if _, ok := s.store.GetNode(nodeID); !ok {
		writeError(w, http.StatusNotFound, "unknown_node_id", "unknown node")
		return
	}
	if err := s.validateTimestamp(observedAt); err != nil {
		writeError(w, http.StatusBadRequest, "stale_timestamp", err.Error())
		return
	}
	if !s.store.SaveOperatorGroupEvidence(store.OperatorGroupEvidence{
		EvidenceRef:     evidenceRef,
		NodeID:          nodeID,
		OperatorGroupID: operatorGroupID,
		ObservedAt:      observedAt,
		Source:          source,
		ReviewState:     reviewState,
	}) {
		writeError(w, http.StatusConflict, "duplicate_evidence_ref", "duplicate evidence_ref")
		return
	}
	if dateUTC, err := datePart(observedAt); err == nil {
		s.rebuildRewardEligibility(dateUTC, s.store.ListRankingRecordsByDate(dateUTC))
	}
	if err := s.persist(); err != nil {
		writeError(w, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "accepted",
		"evidence_ref": evidenceRef,
		"node_id":      nodeID,
		"received_at":  time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleDecentralizationEvidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	payload, err := decodeObject(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	nodeID := stringField(payload, "node_id")
	evidenceRef := stringField(payload, "evidence_ref")
	observedAt := stringField(payload, "observed_at")
	countryCode := strings.ToUpper(strings.TrimSpace(stringField(payload, "country_code")))
	providerID := strings.TrimSpace(stringField(payload, "provider_id"))
	infrastructureID := strings.TrimSpace(stringField(payload, "infrastructure_id"))
	source := stringField(payload, "source")
	if nodeID == "" || evidenceRef == "" || observedAt == "" || countryCode == "" || providerID == "" || source == "" {
		writeError(w, http.StatusBadRequest, "missing_required_field", "missing required field")
		return
	}
	if _, ok := s.store.GetNode(nodeID); !ok {
		writeError(w, http.StatusNotFound, "unknown_node_id", "unknown node")
		return
	}
	if err := s.validateTimestamp(observedAt); err != nil {
		writeError(w, http.StatusBadRequest, "stale_timestamp", err.Error())
		return
	}
	asn, ok := intField(payload, "asn")
	if !ok || asn <= 0 {
		writeError(w, http.StatusBadRequest, "missing_required_field", "missing asn")
		return
	}
	if !s.store.SaveDecentralizationEvidence(store.DecentralizationEvidence{
		EvidenceRef:      evidenceRef,
		NodeID:           nodeID,
		ObservedAt:       observedAt,
		CountryCode:      countryCode,
		ASN:              asn,
		ProviderID:       providerID,
		InfrastructureID: infrastructureID,
		Source:           source,
	}) {
		writeError(w, http.StatusConflict, "duplicate_evidence_ref", "duplicate evidence_ref")
		return
	}
	if dateUTC, err := datePart(observedAt); err == nil {
		s.rebuildRankings(dateUTC)
	}
	if err := s.persist(); err != nil {
		writeError(w, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "accepted",
		"evidence_ref": evidenceRef,
		"node_id":      nodeID,
		"received_at":  time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleDomainEvidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	payload, err := decodeObject(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	nodeID := stringField(payload, "node_id")
	evidenceRef := stringField(payload, "evidence_ref")
	observedAt := stringField(payload, "observed_at")
	registrableDomain := strings.ToLower(strings.TrimSpace(stringField(payload, "registrable_domain")))
	source := stringField(payload, "source")
	if nodeID == "" || evidenceRef == "" || observedAt == "" || registrableDomain == "" || source == "" {
		writeError(w, http.StatusBadRequest, "missing_required_field", "missing required field")
		return
	}
	if _, ok := s.store.GetNode(nodeID); !ok {
		writeError(w, http.StatusNotFound, "unknown_node_id", "unknown node")
		return
	}
	if err := s.validateTimestamp(observedAt); err != nil {
		writeError(w, http.StatusBadRequest, "stale_timestamp", err.Error())
		return
	}
	if !s.store.SaveDomainEvidence(store.DomainEvidence{
		EvidenceRef:       evidenceRef,
		NodeID:            nodeID,
		ObservedAt:        observedAt,
		RegistrableDomain: registrableDomain,
		Source:            source,
	}) {
		writeError(w, http.StatusConflict, "duplicate_evidence_ref", "duplicate evidence_ref")
		return
	}
	if dateUTC, err := datePart(observedAt); err == nil {
		s.rebuildRewardEligibility(dateUTC, s.store.ListRankingRecordsByDate(dateUTC))
	}
	if err := s.persist(); err != nil {
		writeError(w, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "accepted",
		"evidence_ref": evidenceRef,
		"node_id":      nodeID,
		"received_at":  time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleSharedControlPlaneEvidence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	payload, err := decodeObject(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	nodeID := stringField(payload, "node_id")
	evidenceRef := stringField(payload, "evidence_ref")
	observedAt := stringField(payload, "observed_at")
	controlPlaneID := strings.TrimSpace(stringField(payload, "control_plane_id"))
	classification := strings.TrimSpace(stringField(payload, "classification"))
	source := normalizeEvidenceSource(stringField(payload, "source"))
	reviewState := normalizeReviewState(stringField(payload, "review_state"))
	if nodeID == "" || evidenceRef == "" || observedAt == "" || controlPlaneID == "" || classification == "" || source == "" {
		writeError(w, http.StatusBadRequest, "missing_required_field", "missing required field")
		return
	}
	if !validEvidenceSource(source) {
		writeError(w, http.StatusBadRequest, "invalid_source", "invalid source")
		return
	}
	if !validReviewState(reviewState) {
		writeError(w, http.StatusBadRequest, "invalid_review_state", "invalid review_state")
		return
	}
	switch classification {
	case "managed_provider", "operational_contractor", "shared_certificate_admin", "material_overlap":
	default:
		writeError(w, http.StatusBadRequest, "invalid_classification", "invalid classification")
		return
	}
	if _, ok := s.store.GetNode(nodeID); !ok {
		writeError(w, http.StatusNotFound, "unknown_node_id", "unknown node")
		return
	}
	if err := s.validateTimestamp(observedAt); err != nil {
		writeError(w, http.StatusBadRequest, "stale_timestamp", err.Error())
		return
	}
	if !s.store.SaveSharedControlPlaneEvidence(store.SharedControlPlaneEvidence{
		EvidenceRef:    evidenceRef,
		NodeID:         nodeID,
		ObservedAt:     observedAt,
		ControlPlaneID: controlPlaneID,
		Classification: classification,
		Source:         source,
		ReviewState:    reviewState,
	}) {
		writeError(w, http.StatusConflict, "duplicate_evidence_ref", "duplicate evidence_ref")
		return
	}
	if dateUTC, err := datePart(observedAt); err == nil {
		s.rebuildRewardEligibility(dateUTC, s.store.ListRankingRecordsByDate(dateUTC))
	}
	if err := s.persist(); err != nil {
		writeError(w, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "accepted",
		"evidence_ref": evidenceRef,
		"node_id":      nodeID,
		"received_at":  time.Now().UTC().Format(time.RFC3339),
	})
}
