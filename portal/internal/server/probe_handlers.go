package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/store"
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
