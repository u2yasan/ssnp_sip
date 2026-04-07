package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/notifier"
	"github.com/u2yasan/ssnp_sip/portal/internal/store"
)

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
	case alertHeartbeatFailed, alertNodeOutage, alertFinalizedLag:
		return notifier.SeverityCritical
	case alertHeartbeatStale:
		return notifier.SeverityWarning
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
	case alertNodeOutage:
		return "Node availability is below the qualification threshold"
	case alertFinalizedLag:
		return "Finalized lag is above the qualification threshold"
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
