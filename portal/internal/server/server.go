package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/policy"
	"github.com/u2yasan/ssnp_sip/portal/internal/store"
	"github.com/u2yasan/ssnp_sip/portal/internal/verify"
)

type Config struct {
	ListenAddr              string
	PolicyPath              string
	AllowedClockSkewSeconds int
}

type Server struct {
	cfg    Config
	policy policy.Document
	store  *store.Store
}

func New(cfg Config) (*Server, error) {
	doc, err := policy.Load(cfg.PolicyPath)
	if err != nil {
		return nil, err
	}
	if cfg.ListenAddr == "" {
		return nil, errors.New("missing listen address")
	}
	if cfg.AllowedClockSkewSeconds <= 0 {
		cfg.AllowedClockSkewSeconds = 300
	}
	return &Server{
		cfg:    cfg,
		policy: doc,
		store:  store.New([]store.Node{{NodeID: "node-abc"}}),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/agent/policy", s.handlePolicy)
	mux.HandleFunc("/api/v1/agent/enroll", s.handleEnroll)
	mux.HandleFunc("/api/v1/agent/heartbeat", s.handleHeartbeat)
	mux.HandleFunc("/api/v1/agent/checks", s.handleChecks)
	mux.HandleFunc("/api/v1/agent/telemetry", s.handleTelemetry)
	return mux
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:    s.cfg.ListenAddr,
		Handler: s.Handler(),
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

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
	node.LastPolicyVersion = s.policy.PolicyVersion
	node.LastHeartbeatSequence = 0
	s.store.SaveNode(node)
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
	node.LastPolicyVersion = s.policy.PolicyVersion
	s.store.SaveNode(node)
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
	overall, ok := payload["overall_passed"].(bool)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing_field", "missing overall_passed")
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
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "accepted",
		"node_id":        nodeID,
		"event_id":       eventID,
		"overall_passed": overall,
		"received_at":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
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
	warningCode := ""
	if len(flags) > 0 {
		warningCode, _ = flags[0].(string)
	}
	s.store.AddTelemetryEvent(store.TelemetryEvent{
		NodeID:             nodeID,
		TelemetryTimestamp: stringField(payload, "telemetry_timestamp"),
		WarningCode:        warningCode,
	})
	writeJSON(w, http.StatusOK, acceptedResponse(nodeID))
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

func classifyAuthError(err error) (int, string) {
	switch {
	case strings.Contains(err.Error(), "unknown fingerprint"):
		return http.StatusUnauthorized, "unknown_fingerprint"
	case strings.Contains(err.Error(), "invalid signature"):
		return http.StatusUnauthorized, "invalid_signature"
	default:
		return http.StatusBadRequest, "invalid_payload"
	}
}

func acceptedResponse(nodeID string) map[string]any {
	return map[string]any{
		"status":      "accepted",
		"node_id":     nodeID,
		"received_at": time.Now().UTC().Format(time.RFC3339),
	}
}

func writeError(w http.ResponseWriter, status int, errorCode, message string) {
	writeJSON(w, status, map[string]any{
		"status":     "error",
		"error_code": errorCode,
		"message":    message,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
