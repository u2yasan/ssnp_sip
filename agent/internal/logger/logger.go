package logger

import (
	"encoding/json"
	"os"
	"time"
)

type Entry struct {
	Timestamp string         `json:"ts"`
	Level     string         `json:"level"`
	Component string         `json:"component"`
	Event     string         `json:"event"`
	NodeID    string         `json:"node_id,omitempty"`
	Fields    map[string]any `json:"fields,omitempty"`
}

func Log(level, component, event, nodeID string, fields map[string]any) {
	entry := Entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Component: component,
		Event:     event,
		NodeID:    nodeID,
		Fields:    fields,
	}
	enc := json.NewEncoder(os.Stdout)
	_ = enc.Encode(entry)
}
