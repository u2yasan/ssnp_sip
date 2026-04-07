package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
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
	SMTPHost                string
	SMTPPort                int
	SMTPUsername            string
	SMTPPassword            string
	SMTPFrom                string
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

type publicNodeStatusView struct {
	NodeID         string `json:"node_id"`
	DateUTC        string `json:"date_utc"`
	Qualified      bool   `json:"qualified"`
	RankPosition   *int   `json:"rank_position,omitempty"`
	RewardEligible bool   `json:"reward_eligible"`
	StatusReason   string `json:"status_reason,omitempty"`
}

type operatorNodeStatusView struct {
	NodeID          string   `json:"node_id"`
	DateUTC         string   `json:"date_utc"`
	Qualified       bool     `json:"qualified"`
	RankPosition    *int     `json:"rank_position,omitempty"`
	RewardEligible  bool     `json:"reward_eligible"`
	StatusReason    string   `json:"status_reason,omitempty"`
	FailureReasons  []string `json:"failure_reasons,omitempty"`
	HeartbeatPassed bool     `json:"heartbeat_passed"`
	HardwarePassed  bool     `json:"hardware_passed"`
	VotingKeyPassed bool     `json:"voting_key_passed"`
	OperatorGroupID string   `json:"operator_group_id,omitempty"`
	ExclusionReason string   `json:"exclusion_reason,omitempty"`
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
			return nil, errors.New("missing fallback notification email")
		}
		if strings.TrimSpace(cfg.SMTPHost) == "" || strings.TrimSpace(cfg.SMTPUsername) == "" || strings.TrimSpace(cfg.SMTPFrom) == "" {
			return nil, errors.New("missing smtp configuration")
		}
		if cfg.SMTPPort <= 0 {
			cfg.SMTPPort = 587
		}
		if cfg.SMTPPassword == "" {
			return nil, errors.New("missing smtp password")
		}
		cfg.Notifier = notifier.SMTPNotifier{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
			Timeout:  10 * time.Second,
		}
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
	mux.HandleFunc("/api/v1/probes/events", s.handleProbeEvents)
	mux.HandleFunc("/api/v1/probes/daily-summaries/", s.handleDailyProbeSummary)
	mux.HandleFunc("/api/v1/voting-key-evidence", s.handleVotingKeyEvidence)
	mux.HandleFunc("/api/v1/operator-group-evidence", s.handleOperatorGroupEvidence)
	mux.HandleFunc("/api/v1/rankings/", s.handleRankingRead)
	mux.HandleFunc("/api/v1/reward-eligibility/", s.handleRewardEligibilityRead)
	mux.HandleFunc("/api/v1/public-node-status/", s.handlePublicNodeStatusRead)
	mux.HandleFunc("/api/v1/operator-node-status/", s.handleOperatorNodeStatusRead)
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

	heartbeatPassed := false
	if strings.TrimSpace(node.LastHeartbeatTimestamp) == "" {
		failureReasons = append(failureReasons, "heartbeat_missing")
	} else {
		lastSeen, err := time.Parse(time.RFC3339, node.LastHeartbeatTimestamp)
		if err != nil || lastSeen.UTC().Format("2006-01-02") != summary.DateUTC {
			failureReasons = append(failureReasons, "heartbeat_missing")
		} else if time.Since(lastSeen.UTC()) >= s.cfg.HeartbeatStaleAfter {
			failureReasons = append(failureReasons, "heartbeat_stale")
		} else {
			heartbeatPassed = true
		}
	}

	hardwarePassed := false
	if latestCheck, ok := s.store.LatestCheckEventForNode(nodeID); !ok {
		failureReasons = append(failureReasons, "hardware_check_missing")
	} else if strings.TrimSpace(latestCheck.CheckedAt) == "" {
		failureReasons = append(failureReasons, "hardware_check_missing")
	} else if checkedDateUTC, err := datePart(latestCheck.CheckedAt); err != nil || checkedDateUTC != summary.DateUTC {
		failureReasons = append(failureReasons, "hardware_check_missing")
	} else if !latestCheck.OverallPassed {
		failureReasons = append(failureReasons, "hardware_check_failed")
	} else {
		hardwarePassed = true
	}

	votingKeyPassed := false
	if latestEvidence, ok := s.store.GetLatestVotingKeyEvidenceForNode(nodeID); !ok {
		failureReasons = append(failureReasons, "voting_key_evidence_missing")
	} else if observedDateUTC, err := datePart(latestEvidence.ObservedAt); err != nil || observedDateUTC != summary.DateUTC {
		failureReasons = append(failureReasons, "voting_key_evidence_missing")
	} else if !latestEvidence.VotingKeyPresent {
		failureReasons = append(failureReasons, "voting_key_not_present")
	} else if !latestEvidence.VotingKeyValidForEpoch {
		failureReasons = append(failureReasons, "voting_key_invalid")
	} else {
		votingKeyPassed = true
	}

	return store.QualifiedDecisionRecord{
		NodeID:                     nodeID,
		DateUTC:                    summary.DateUTC,
		PolicyVersion:              summary.PolicyVersion,
		ProbeEvidencePassed:        probeEvidencePassed,
		HeartbeatPassed:            heartbeatPassed,
		HardwarePassed:             hardwarePassed,
		VotingKeyPassed:            votingKeyPassed,
		Qualified:                  probeEvidencePassed && heartbeatPassed && hardwarePassed && votingKeyPassed,
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
	sort.Slice(records, func(i, j int) bool {
		if records[i].BasePerformanceScore == records[j].BasePerformanceScore {
			if records[i].FinalizationScore == records[j].FinalizationScore {
				if records[i].AvailabilityScore == records[j].AvailabilityScore {
					return records[i].NodeID < records[j].NodeID
				}
				return records[i].AvailabilityScore > records[j].AvailabilityScore
			}
			return records[i].FinalizationScore > records[j].FinalizationScore
		}
		return records[i].BasePerformanceScore > records[j].BasePerformanceScore
	})
	rankings := make([]store.RankingRecord, 0, len(records))
	computedAt := time.Now().UTC().Format(time.RFC3339)
	for i, record := range records {
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
			DecentralizationScore: 0,
			TotalScore:            record.BasePerformanceScore,
			OperatorGroupID:       record.NodeID,
			RewardEligible:        true,
			ComputedAt:            computedAt,
		})
	}
	s.store.ReplaceRankingRecordsForDate(dateUTC, rankings)
	s.rebuildRewardEligibility(dateUTC, rankings)
}

func (s *Server) rebuildRewardEligibility(dateUTC string, rankings []store.RankingRecord) {
	records := make([]store.RewardEligibilityRecord, 0, len(rankings))
	decidedAt := time.Now().UTC().Format(time.RFC3339)
	seenGroups := map[string]struct{}{}
	for _, ranking := range rankings {
		operatorGroupID := ranking.NodeID
		if evidence, ok := s.store.GetLatestOperatorGroupEvidenceForNode(ranking.NodeID); ok {
			if observedDateUTC, err := datePart(evidence.ObservedAt); err == nil && observedDateUTC == dateUTC && strings.TrimSpace(evidence.OperatorGroupID) != "" {
				operatorGroupID = evidence.OperatorGroupID
			}
		}
		rewardEligible := true
		exclusionReason := ""
		if _, exists := seenGroups[operatorGroupID]; exists {
			rewardEligible = false
			exclusionReason = "same_operator_group_lower_ranked"
		} else {
			seenGroups[operatorGroupID] = struct{}{}
		}
		records = append(records, store.RewardEligibilityRecord{
			NodeID:          ranking.NodeID,
			DateUTC:         dateUTC,
			PolicyVersion:   ranking.PolicyVersion,
			RankPosition:    ranking.RankPosition,
			Qualified:       true,
			OperatorGroupID: operatorGroupID,
			RewardEligible:  rewardEligible,
			ExclusionReason: exclusionReason,
			DecidedAt:       decidedAt,
		})
	}
	s.store.ReplaceRewardEligibilityRecordsForDate(dateUTC, records)
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
	recipient := strings.TrimSpace(s.cfg.NotificationEmailTo)
	if node, ok := s.store.GetNode(nodeID); ok && strings.TrimSpace(node.OperatorEmail) != "" {
		recipient = strings.TrimSpace(node.OperatorEmail)
	}
	if recipient == "" {
		s.recordDeliveryFailure(nodeID, alertCode, severity, occurredAt, now, "email", "", "missing notification recipient")
		return nil
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
		s.recordDeliveryFailure(nodeID, alertCode, severity, occurredAt, now, notification.Channel, notification.Recipient, err.Error())
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

func (s *Server) recordDeliveryFailure(nodeID, alertCode string, severity notifier.Severity, occurredAt string, now time.Time, channel, recipient, detail string) {
	s.store.AddNotificationDelivery(store.NotificationDelivery{
		NodeID:      nodeID,
		AlertCode:   alertCode,
		Severity:    string(severity),
		Channel:     channel,
		Recipient:   recipient,
		OccurredAt:  occurredAt,
		SentAt:      now.Format(time.RFC3339),
		Status:      "failed",
		ErrorDetail: detail,
	})
	s.store.AddOperationalEvent(store.OperationalEvent{
		NodeID:         nodeID,
		EventCode:      operationalEventDeliveryFailure,
		Severity:       "warning",
		EventTimestamp: now.Format(time.RFC3339),
		Detail:         detail,
	})
	if persistErr := s.persist(); persistErr != nil {
		fmt.Fprintf(os.Stderr, "portal-server: failed to persist notification failure for %s: %v\n", nodeID, persistErr)
	}
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
