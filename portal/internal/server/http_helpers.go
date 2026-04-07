package server

import (
	"net/http"
	"strings"
	"time"
)

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
