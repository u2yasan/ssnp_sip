package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
)

type Notification struct {
	NodeID     string   `json:"node_id"`
	AlertCode  string   `json:"alert_code"`
	Severity   Severity `json:"severity"`
	Channel    string   `json:"channel"`
	Recipient  string   `json:"recipient"`
	OccurredAt string   `json:"occurred_at"`
	Message    string   `json:"message"`
}

type Notifier interface {
	Send(ctx context.Context, notification Notification) error
}

type StdoutNotifier struct {
	Writer io.Writer
}

func (n StdoutNotifier) Send(_ context.Context, notification Notification) error {
	writer := n.Writer
	if writer == nil {
		writer = os.Stdout
	}
	body, err := json.Marshal(notification)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, string(body)); err != nil {
		return err
	}
	return nil
}

func Subject(notification Notification) string {
	return fmt.Sprintf("[SSNP][%s] %s %s", strings.ToUpper(string(notification.Severity)), notification.AlertCode, notification.NodeID)
}

func Body(notification Notification) string {
	return fmt.Sprintf(
		"node_id: %s\nalert_code: %s\nseverity: %s\noccurred_at: %s\nmessage: %s\nnote: dedupe/cooldown may suppress repeats\n",
		notification.NodeID,
		notification.AlertCode,
		notification.Severity,
		notification.OccurredAt,
		notification.Message,
	)
}
