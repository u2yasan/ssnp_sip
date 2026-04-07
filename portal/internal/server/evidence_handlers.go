package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/store"
	"github.com/u2yasan/ssnp_sip/portal/internal/verify"
)

func (s *Server) handleProbeEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	payload, err := decodeObject(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	probeID := stringField(payload, "probe_id")
	nodeID := stringField(payload, "node_id")
	regionID := stringField(payload, "region_id")
	observedAt := stringField(payload, "observed_at")
	endpoint := stringField(payload, "endpoint")
	if stringField(payload, "schema_version") != "1" {
		writeError(w, http.StatusBadRequest, "invalid_schema_version", "invalid schema_version")
		return
	}
	if probeID == "" || nodeID == "" || regionID == "" || observedAt == "" || endpoint == "" {
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

	availabilityUp, ok := payload["availability_up"].(bool)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing_required_field", "missing availability_up")
		return
	}
	measurementWindowSeconds, ok := intField(payload, "measurement_window_seconds")
	if !ok || measurementWindowSeconds <= 0 {
		writeError(w, http.StatusBadRequest, "missing_required_field", "missing measurement_window_seconds")
		return
	}

	finalizedLag, finalizedPresent, err := optionalNonNegativeIntField(payload, "finalized_lag_blocks")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_lag_value", err.Error())
		return
	}
	chainLag, chainPresent, err := optionalNonNegativeIntField(payload, "chain_lag_blocks")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_lag_value", err.Error())
		return
	}
	sourceHeight, _, err := optionalNonNegativeIntField(payload, "source_height")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_lag_value", err.Error())
		return
	}
	peerHeight, _, err := optionalNonNegativeIntField(payload, "peer_height")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_lag_value", err.Error())
		return
	}
	httpStatus, _, err := optionalNonNegativeIntField(payload, "http_status")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}
	if availabilityUp && (!finalizedPresent || !chainPresent || sourceHeight == nil || peerHeight == nil) {
		writeError(w, http.StatusBadRequest, "missing_required_field", "missing measurable fields for available probe")
		return
	}

	if !s.store.SaveProbeEvent(store.ProbeEvent{
		ProbeID:                  probeID,
		NodeID:                   nodeID,
		RegionID:                 regionID,
		ObservedAt:               observedAt,
		Endpoint:                 endpoint,
		AvailabilityUp:           availabilityUp,
		FinalizedLagBlocks:       finalizedLag,
		ChainLagBlocks:           chainLag,
		SourceHeight:             sourceHeight,
		PeerHeight:               peerHeight,
		MeasurementWindowSeconds: measurementWindowSeconds,
		HTTPStatus:               httpStatus,
		ErrorCode:                stringField(payload, "error_code"),
		ResolverIP:               stringField(payload, "resolver_ip"),
		Notes:                    stringField(payload, "notes"),
	}) {
		writeError(w, http.StatusConflict, "duplicate_probe_id", "duplicate probe_id")
		return
	}
	dateUTC, err := datePart(observedAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "stale_timestamp", err.Error())
		return
	}
	s.updateQualificationArtifacts(nodeID, dateUTC)
	if err := s.persist(); err != nil {
		writeError(w, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "accepted",
		"probe_id":    probeID,
		"node_id":     nodeID,
		"received_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleDailyProbeSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/probes/daily-summaries/")
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
	summary, ok := s.store.GetDailyQualificationSummary(nodeID, dateUTC)
	if !ok {
		writeError(w, http.StatusNotFound, "missing_daily_summary", "daily summary not found")
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

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
	source := stringField(payload, "source")
	if nodeID == "" || evidenceRef == "" || operatorGroupID == "" || observedAt == "" || source == "" {
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
	if !s.store.SaveOperatorGroupEvidence(store.OperatorGroupEvidence{
		EvidenceRef:     evidenceRef,
		NodeID:          nodeID,
		OperatorGroupID: operatorGroupID,
		ObservedAt:      observedAt,
		Source:          source,
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

func (s *Server) authorizeMapPayload(node store.Node, payload map[string]any) error {
	if node.AgentPublicKey == "" {
		return errors.New("unknown fingerprint")
	}
	if stringField(payload, "agent_key_fingerprint") != node.ActiveAgentKeyFingerprint {
		return errors.New("unknown fingerprint")
	}
	canonical, err := verify.CanonicalJSONWithoutSignature(payload)
	if err != nil {
		return err
	}
	return verify.VerifyHexPublicKeySignature(node.AgentPublicKey, stringField(payload, "signature"), canonical)
}

func (s *Server) validateTimestamp(raw string) error {
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return err
	}
	maxSkew := time.Duration(s.cfg.AllowedClockSkewSeconds) * time.Second
	now := time.Now().UTC()
	if ts.Before(now.Add(-maxSkew)) || ts.After(now.Add(maxSkew)) {
		return fmt.Errorf("timestamp outside allowed clock skew")
	}
	return nil
}

func decodeObject(r *http.Request) (map[string]any, error) {
	defer r.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func stringField(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return value
}

func intField(payload map[string]any, key string) (int, bool) {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch value := raw.(type) {
	case float64:
		if value != float64(int(value)) {
			return 0, false
		}
		return int(value), true
	default:
		return 0, false
	}
}

func optionalNonNegativeIntField(payload map[string]any, key string) (*int, bool, error) {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return nil, false, nil
	}
	value, ok := raw.(float64)
	if !ok || value != float64(int(value)) {
		return nil, false, fmt.Errorf("%s must be an integer", key)
	}
	intValue := int(value)
	if intValue < 0 {
		return nil, false, fmt.Errorf("%s must be >= 0", key)
	}
	return &intValue, true, nil
}
