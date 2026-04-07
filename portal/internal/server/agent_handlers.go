package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/store"
	"github.com/u2yasan/ssnp_sip/portal/internal/verify"
)

func (s *Server) handlePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	nodeID := strings.TrimSpace(r.URL.Query().Get("node_id"))
	fingerprint := strings.TrimSpace(r.URL.Query().Get("agent_key_fingerprint"))
	if nodeID == "" {
		writeError(w, http.StatusBadRequest, "missing_node_id", "missing node_id")
		return
	}
	if fingerprint == "" {
		writeError(w, http.StatusBadRequest, "missing_agent_key_fingerprint", "missing agent_key_fingerprint")
		return
	}
	if _, ok := s.store.GetNode(nodeID); !ok {
		writeError(w, http.StatusNotFound, "unknown_node", "unknown node")
		return
	}
	writeJSON(w, http.StatusOK, s.policy)
}

func (s *Server) handleEnrollmentChallenge(w http.ResponseWriter, r *http.Request) {
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
	if nodeID == "" {
		writeError(w, http.StatusBadRequest, "missing_node_id", "missing node_id")
		return
	}
	if _, ok := s.store.GetNode(nodeID); !ok {
		writeError(w, http.StatusNotFound, "unknown_node", "unknown node")
		return
	}
	issuedAt := time.Now().UTC()
	challenge := store.EnrollmentChallenge{
		ChallengeID: fmt.Sprintf("%s-%d", nodeID, issuedAt.UnixNano()),
		NodeID:      nodeID,
		IssuedAt:    issuedAt.Format(time.RFC3339),
		ExpiresAt:   issuedAt.Add(s.cfg.EnrollmentChallengeTTL).Format(time.RFC3339),
	}
	s.store.SaveEnrollmentChallenge(challenge)
	if err := s.persist(); err != nil {
		writeError(w, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "ok",
		"challenge_id": challenge.ChallengeID,
		"node_id":      challenge.NodeID,
		"expires_at":   challenge.ExpiresAt,
	})
}

func (s *Server) handleEnroll(w http.ResponseWriter, r *http.Request) {
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
	challengeID := stringField(payload, "enrollment_challenge_id")
	publicKey := stringField(payload, "agent_public_key")
	signature := stringField(payload, "signature")
	if nodeID == "" || challengeID == "" || publicKey == "" || signature == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "missing required field")
		return
	}
	node, ok := s.store.GetNode(nodeID)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown_node", "unknown node")
		return
	}
	challenge, ok := s.store.GetEnrollmentChallenge(challengeID)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_enrollment_challenge", "invalid enrollment challenge")
		return
	}
	if challenge.NodeID != nodeID {
		writeError(w, http.StatusUnauthorized, "invalid_enrollment_challenge", "enrollment challenge does not match node")
		return
	}
	if strings.TrimSpace(challenge.UsedAt) != "" {
		writeError(w, http.StatusConflict, "enrollment_challenge_used", "enrollment challenge already used")
		return
	}
	expiresAt, err := time.Parse(time.RFC3339, challenge.ExpiresAt)
	if err != nil || !time.Now().UTC().Before(expiresAt) {
		writeError(w, http.StatusUnauthorized, "enrollment_challenge_expired", "enrollment challenge expired")
		return
	}
	fingerprint, err := verify.FingerprintFromHexPublicKey(publicKey)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_public_key", err.Error())
		return
	}
	canonical, err := verify.CanonicalJSONWithoutSignature(payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}
	if err := verify.VerifyHexPublicKeySignature(publicKey, signature, canonical); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_signature", err.Error())
		return
	}
	node.ActiveAgentKeyFingerprint = fingerprint
	node.AgentPublicKey = publicKey
	node.EnrollmentGeneration++
	if strings.TrimSpace(node.ValidatedRegistrationAt) == "" {
		node.ValidatedRegistrationAt = time.Now().UTC().Format(time.RFC3339)
	}
	node.LastPolicyVersion = s.policy.PolicyVersion
	node.LastHeartbeatSequence = 0
	s.store.SaveNode(node)
	s.store.ConsumeEnrollmentChallenge(challengeID, time.Now().UTC().Format(time.RFC3339))
	if err := s.persist(); err != nil {
		writeError(w, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                "ok",
		"node_id":               nodeID,
		"agent_key_fingerprint": fingerprint,
		"policy_version":        s.policy.PolicyVersion,
	})
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusBadRequest, "invalid_method", "invalid method")
		return
	}
	var payload struct {
		NodeID                string   `json:"node_id"`
		AgentKeyFingerprint   string   `json:"agent_key_fingerprint"`
		HeartbeatTimestamp    string   `json:"heartbeat_timestamp"`
		SequenceNumber        int      `json:"sequence_number"`
		AgentVersion          string   `json:"agent_version"`
		EnrollmentGeneration  int      `json:"enrollment_generation"`
		LocalObservationFlags []string `json:"local_observation_flags"`
		Signature             string   `json:"signature,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	node, ok := s.store.GetNode(payload.NodeID)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown_node", "unknown node")
		return
	}
	if node.AgentPublicKey == "" || payload.AgentKeyFingerprint != node.ActiveAgentKeyFingerprint {
		writeError(w, http.StatusUnauthorized, "unknown_fingerprint", "unknown fingerprint")
		return
	}
	if payload.EnrollmentGeneration != node.EnrollmentGeneration {
		writeError(w, http.StatusConflict, "enrollment_generation_mismatch", "enrollment generation mismatch")
		return
	}
	if err := s.validateTimestamp(payload.HeartbeatTimestamp); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_timestamp", err.Error())
		return
	}
	copyPayload := payload
	copyPayload.Signature = ""
	canonical, err := json.Marshal(copyPayload)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_payload", err.Error())
		return
	}
	if err := verify.VerifyHexPublicKeySignature(node.AgentPublicKey, payload.Signature, canonical); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_signature", err.Error())
		return
	}
	if payload.SequenceNumber <= node.LastHeartbeatSequence {
		writeError(w, http.StatusConflict, "stale_sequence", "sequence number must be strictly increasing")
		return
	}
	node.LastHeartbeatSequence = payload.SequenceNumber
	node.LastHeartbeatTimestamp = payload.HeartbeatTimestamp
	node.LastPolicyVersion = s.policy.PolicyVersion
	s.store.SaveNode(node)
	s.store.SaveHeartbeatEvent(store.HeartbeatEvent{
		NodeID:             payload.NodeID,
		HeartbeatTimestamp: payload.HeartbeatTimestamp,
		SequenceNumber:     payload.SequenceNumber,
	})
	if dateUTC, err := datePart(payload.HeartbeatTimestamp); err == nil {
		s.updateQualificationArtifacts(payload.NodeID, dateUTC)
	}
	if err := s.persist(); err != nil {
		writeError(w, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, acceptedResponse(payload.NodeID))
}

func (s *Server) handleChecks(w http.ResponseWriter, r *http.Request) {
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
	node, ok := s.store.GetNode(nodeID)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown_node", "unknown node")
		return
	}
	if err := s.authorizeMapPayload(node, payload); err != nil {
		status, code := classifyAuthError(err)
		writeError(w, status, code, err.Error())
		return
	}
	if stringField(payload, "policy_version") != s.policy.PolicyVersion {
		writeError(w, http.StatusConflict, "policy_version_mismatch", "policy version mismatch")
		return
	}
	if stringField(payload, "schema_version") != "1" {
		writeError(w, http.StatusBadRequest, "invalid_schema_version", "invalid schema_version")
		return
	}
	if stringField(payload, "cpu_profile_id") != s.policy.CPUProfile.ID {
		writeError(w, http.StatusConflict, "cpu_profile_mismatch", "cpu profile mismatch")
		return
	}
	if stringField(payload, "disk_profile_id") != s.policy.DiskProfile.ID {
		writeError(w, http.StatusConflict, "disk_profile_mismatch", "disk profile mismatch")
		return
	}
	eventID := stringField(payload, "event_id")
	if eventID == "" {
		writeError(w, http.StatusBadRequest, "missing_field", "missing event_id")
		return
	}
	eventType := stringField(payload, "event_type")
	switch eventType {
	case "registration", "voting_key_renewal", "recheck":
	default:
		writeError(w, http.StatusBadRequest, "invalid_event_type", "invalid event_type")
		return
	}
	if err := s.validateTimestamp(stringField(payload, "checked_at")); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_timestamp", err.Error())
		return
	}
	boolKeys := []string{
		"cpu_check_passed",
		"disk_check_passed",
		"ram_check_passed",
		"storage_size_check_passed",
		"ssd_check_passed",
		"cpu_load_test_passed",
	}
	subChecks := make(map[string]bool, len(boolKeys))
	for _, key := range boolKeys {
		value, ok := payload[key].(bool)
		if !ok {
			writeError(w, http.StatusBadRequest, "missing_field", "missing "+key)
			return
		}
		subChecks[key] = value
	}
	overall, ok := payload["overall_passed"].(bool)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing_field", "missing overall_passed")
		return
	}
	expectedOverall := subChecks["cpu_check_passed"] &&
		subChecks["disk_check_passed"] &&
		subChecks["ram_check_passed"] &&
		subChecks["storage_size_check_passed"] &&
		subChecks["ssd_check_passed"] &&
		subChecks["cpu_load_test_passed"]
	if overall != expectedOverall {
		writeError(w, http.StatusBadRequest, "invalid_overall_passed", "overall_passed does not match sub-checks")
		return
	}
	if !s.store.SaveCheckEvent(store.CheckEvent{
		EventID:       eventID,
		NodeID:        nodeID,
		OverallPassed: overall,
		CheckedAt:     stringField(payload, "checked_at"),
	}) {
		writeError(w, http.StatusConflict, "duplicate_event_id", "duplicate event_id")
		return
	}
	if checkedAt := stringField(payload, "checked_at"); checkedAt != "" {
		if dateUTC, err := datePart(checkedAt); err == nil {
			s.updateQualificationArtifacts(nodeID, dateUTC)
		}
	}
	if err := s.persist(); err != nil {
		writeError(w, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "accepted",
		"node_id":        nodeID,
		"event_id":       eventID,
		"overall_passed": overall,
		"received_at":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.handleTelemetryRead(w, r)
		return
	}
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
	node, ok := s.store.GetNode(nodeID)
	if !ok {
		writeError(w, http.StatusNotFound, "unknown_node", "unknown node")
		return
	}
	if err := s.authorizeMapPayload(node, payload); err != nil {
		status, code := classifyAuthError(err)
		writeError(w, status, code, err.Error())
		return
	}
	if stringField(payload, "policy_version") != s.policy.PolicyVersion {
		writeError(w, http.StatusConflict, "policy_version_mismatch", "policy version mismatch")
		return
	}
	if err := s.validateTimestamp(stringField(payload, "telemetry_timestamp")); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_timestamp", err.Error())
		return
	}
	flags, ok := payload["warning_flags"].([]any)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing_field", "missing warning_flags")
		return
	}
	if len(flags) == 0 {
		writeError(w, http.StatusBadRequest, "missing_field", "missing warning_flags")
		return
	}
	telemetryTimestamp := stringField(payload, "telemetry_timestamp")
	for _, raw := range flags {
		warningCode, _ := raw.(string)
		if !isSupportedWarningCode(warningCode) {
			writeError(w, http.StatusBadRequest, "invalid_warning_code", "invalid warning_code")
			return
		}
		s.store.AddTelemetryEvent(store.TelemetryEvent{
			NodeID:             nodeID,
			TelemetryTimestamp: telemetryTimestamp,
			WarningCode:        warningCode,
		})
		if err := s.maybeNotifyAlert(r.Context(), nodeID, warningCode, telemetryTimestamp, time.Now().UTC()); err != nil {
			writeError(w, http.StatusInternalServerError, "state_persist_failed", err.Error())
			return
		}
	}
	if err := s.persist(); err != nil {
		writeError(w, http.StatusInternalServerError, "state_persist_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, acceptedResponse(nodeID))
}

func (s *Server) handleTelemetryRead(w http.ResponseWriter, r *http.Request) {
	nodeID := strings.TrimSpace(r.URL.Query().Get("node_id"))
	warningCode := strings.TrimSpace(r.URL.Query().Get("warning_code"))
	view := strings.TrimSpace(r.URL.Query().Get("view"))
	if view == "latest" {
		writeJSON(w, http.StatusOK, map[string]any{
			"items": s.store.ListLatestTelemetry(nodeID, warningCode),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": s.store.ListTelemetry(nodeID, warningCode),
	})
}
