package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/notifier"
	"github.com/u2yasan/ssnp_sip/portal/internal/policy"
	"github.com/u2yasan/ssnp_sip/portal/internal/store"
	"github.com/u2yasan/ssnp_sip/portal/internal/verify"
)

const (
	alertHeartbeatStale             = "heartbeat_stale"
	alertHeartbeatFailed            = "heartbeat_failed"
	operationalEventDeliveryFailure = "notification_delivery_failed"
)

type Config struct {
	ListenAddr              string
	PolicyPath              string
	NodesConfigPath         string
	StatePath               string
	AllowedClockSkewSeconds int
	NotificationEmailTo     string
	HeartbeatStaleAfter     time.Duration
	HeartbeatFailedAfter    time.Duration
	AlertScanInterval       time.Duration
	Notifier                notifier.Notifier
}

type Server struct {
	cfg      Config
	policy   policy.Document
	store    *store.Store
	notifier notifier.Notifier
}

func New(cfg Config) (*Server, error) {
	doc, err := policy.Load(cfg.PolicyPath)
	if err != nil {
		return nil, err
	}
	if cfg.ListenAddr == "" {
		return nil, errors.New("missing listen address")
	}
	if cfg.NodesConfigPath == "" {
		return nil, errors.New("missing nodes config path")
	}
	if cfg.StatePath == "" {
		return nil, errors.New("missing state path")
	}
	if cfg.AllowedClockSkewSeconds <= 0 {
		cfg.AllowedClockSkewSeconds = 300
	}
	if cfg.HeartbeatStaleAfter <= 0 {
		cfg.HeartbeatStaleAfter = 15 * time.Minute
	}
	if cfg.HeartbeatFailedAfter <= 0 {
		cfg.HeartbeatFailedAfter = 30 * time.Minute
	}
	if cfg.HeartbeatFailedAfter <= cfg.HeartbeatStaleAfter {
		return nil, errors.New("heartbeat failed threshold must be greater than stale threshold")
	}
	if cfg.AlertScanInterval <= 0 {
		cfg.AlertScanInterval = time.Minute
	}
	if cfg.Notifier == nil {
		if strings.TrimSpace(cfg.NotificationEmailTo) == "" {
			return nil, errors.New("missing notification email")
		}
		cfg.Notifier = notifier.StdoutNotifier{}
	}
	seedNodes, err := store.LoadNodesConfig(cfg.NodesConfigPath)
	if err != nil {
		return nil, err
	}
	st, err := store.Load(seedNodes, cfg.StatePath)
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:      cfg,
		policy:   doc,
		store:    st,
		notifier: cfg.Notifier,
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
	go s.runAlertLoop(ctx)
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

func (s *Server) persist() error {
	return s.store.Save(s.cfg.StatePath)
}

func (s *Server) runAlertLoop(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.AlertScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			s.evaluateHeartbeatAlerts(ctx, now.UTC())
		}
	}
}

func (s *Server) evaluateHeartbeatAlerts(ctx context.Context, now time.Time) {
	for _, node := range s.store.ListNodes() {
		if node.AgentPublicKey == "" || node.LastHeartbeatTimestamp == "" {
			continue
		}
		lastSeen, err := time.Parse(time.RFC3339, node.LastHeartbeatTimestamp)
		if err != nil {
			continue
		}
		age := now.Sub(lastSeen)
		switch {
		case age >= s.cfg.HeartbeatFailedAfter:
			if err := s.maybeNotifyAlert(ctx, node.NodeID, alertHeartbeatFailed, now.Format(time.RFC3339), now); err != nil {
				fmt.Fprintf(os.Stderr, "portal-server: failed heartbeat alert persistence for %s: %v\n", node.NodeID, err)
			}
		case age >= s.cfg.HeartbeatStaleAfter:
			if err := s.maybeNotifyAlert(ctx, node.NodeID, alertHeartbeatStale, now.Format(time.RFC3339), now); err != nil {
				fmt.Fprintf(os.Stderr, "portal-server: stale heartbeat alert persistence for %s: %v\n", node.NodeID, err)
			}
		}
	}
}

func (s *Server) maybeNotifyAlert(ctx context.Context, nodeID, alertCode, occurredAt string, now time.Time) error {
	severity := severityForAlert(alertCode)
	if state, ok := s.store.GetAlertState(nodeID, alertCode, string(severity)); ok {
		lastSentAt, err := time.Parse(time.RFC3339, state.LastSentAt)
		if err == nil && now.Before(lastSentAt.Add(cooldownForSeverity(severity))) {
			return nil
		}
	}
	recipient := s.cfg.NotificationEmailTo
	if node, ok := s.store.GetNode(nodeID); ok && strings.TrimSpace(node.OperatorEmail) != "" {
		recipient = node.OperatorEmail
	}
	notification := notifier.Notification{
		NodeID:     nodeID,
		AlertCode:  alertCode,
		Severity:   severity,
		Channel:    "email",
		Recipient:  recipient,
		OccurredAt: occurredAt,
		Message:    notificationMessage(alertCode),
	}
	if err := s.notifier.Send(ctx, notification); err != nil {
		s.store.AddNotificationDelivery(store.NotificationDelivery{
			NodeID:      nodeID,
			AlertCode:   alertCode,
			Severity:    string(severity),
			Channel:     notification.Channel,
			Recipient:   notification.Recipient,
			OccurredAt:  occurredAt,
			SentAt:      now.Format(time.RFC3339),
			Status:      "failed",
			ErrorDetail: err.Error(),
		})
		s.store.AddOperationalEvent(store.OperationalEvent{
			NodeID:         nodeID,
			EventCode:      operationalEventDeliveryFailure,
			Severity:       "warning",
			EventTimestamp: now.Format(time.RFC3339),
			Detail:         err.Error(),
		})
		if persistErr := s.persist(); persistErr != nil {
			fmt.Fprintf(os.Stderr, "portal-server: failed to persist notification failure for %s: %v\n", nodeID, persistErr)
		}
		return nil
	}
	s.store.SaveAlertState(store.AlertState{
		NodeID:      nodeID,
		AlertCode:   alertCode,
		Severity:    string(severity),
		LastSentAt:  now.Format(time.RFC3339),
		LastChannel: notification.Channel,
		Recipient:   notification.Recipient,
	})
	s.store.AddNotificationDelivery(store.NotificationDelivery{
		NodeID:     nodeID,
		AlertCode:  alertCode,
		Severity:   string(severity),
		Channel:    notification.Channel,
		Recipient:  notification.Recipient,
		OccurredAt: occurredAt,
		SentAt:     now.Format(time.RFC3339),
		Status:     "sent",
	})
	return s.persist()
}

func severityForAlert(alertCode string) notifier.Severity {
	switch alertCode {
	case alertHeartbeatStale, alertHeartbeatFailed:
		return notifier.SeverityCritical
	default:
		return notifier.SeverityWarning
	}
}

func cooldownForSeverity(severity notifier.Severity) time.Duration {
	if severity == notifier.SeverityCritical {
		return 15 * time.Minute
	}
	return 24 * time.Hour
}

func notificationMessage(alertCode string) string {
	switch alertCode {
	case alertHeartbeatStale:
		return "Program Agent heartbeat is stale"
	case alertHeartbeatFailed:
		return "Program Agent heartbeat is failed"
	case "portal_unreachable":
		return "Program Agent cannot reach portal"
	case "voting_key_expiry_risk":
		return "Voting key expiry is approaching"
	case "certificate_expiry_risk":
		return "TLS certificate expiry is approaching"
	case "local_check_execution_failed":
		return "Local hardware check execution failed"
	default:
		return "SSNP portal alert"
	}
}

func isSupportedWarningCode(code string) bool {
	switch code {
	case "portal_unreachable", "local_check_execution_failed", "voting_key_expiry_risk", "certificate_expiry_risk":
		return true
	default:
		return false
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
