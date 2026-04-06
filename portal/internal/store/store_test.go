package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNodesConfigAndSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	nodesPath := filepath.Join(dir, "nodes.yaml")
	if err := os.WriteFile(nodesPath, []byte(`nodes:
  - node_id: "node-abc"
    display_name: "Node ABC"
    operator_email: "ops@example.invalid"
    enabled: true
`), 0o600); err != nil {
		t.Fatalf("WriteFile(nodes) error = %v", err)
	}
	seedNodes, err := LoadNodesConfig(nodesPath)
	if err != nil {
		t.Fatalf("LoadNodesConfig() error = %v", err)
	}
	if len(seedNodes) != 1 || seedNodes[0].NodeID != "node-abc" {
		t.Fatalf("seedNodes = %#v, want node-abc", seedNodes)
	}

	statePath := filepath.Join(dir, "portal-state.json")
	st, err := Load(seedNodes, statePath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	node, _ := st.GetNode("node-abc")
	node.ActiveAgentKeyFingerprint = "fp"
	node.LastHeartbeatSequence = 2
	node.LastHeartbeatTimestamp = "2026-04-06T10:00:00Z"
	st.SaveNode(node)
	st.AddTelemetryEvent(TelemetryEvent{
		NodeID:             "node-abc",
		TelemetryTimestamp: "2026-04-06T10:01:00Z",
		WarningCode:        "portal_unreachable",
	})
	st.SaveAlertState(AlertState{
		NodeID:     "node-abc",
		AlertCode:  "portal_unreachable",
		Severity:   "warning",
		LastSentAt: "2026-04-06T10:01:00Z",
	})
	if err := st.Save(statePath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := Load(seedNodes, statePath)
	if err != nil {
		t.Fatalf("Load(reloaded) error = %v", err)
	}
	got, _ := reloaded.GetNode("node-abc")
	if got.ActiveAgentKeyFingerprint != "fp" || got.LastHeartbeatSequence != 2 {
		t.Fatalf("reloaded node = %#v", got)
	}
	if len(reloaded.ListTelemetry("node-abc", "")) != 1 {
		t.Fatal("expected telemetry event after reload")
	}
	if _, ok := reloaded.GetAlertState("node-abc", "portal_unreachable", "warning"); !ok {
		t.Fatal("expected alert state after reload")
	}
}

func TestLoadRejectsSnapshotUnknownNode(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "portal-state.json")
	if err := os.WriteFile(statePath, []byte(`{
  "nodes": [{"node_id":"node-other"}]
}`), 0o600); err != nil {
		t.Fatalf("WriteFile(state) error = %v", err)
	}
	_, err := Load([]Node{{NodeID: "node-abc", Enabled: true}}, statePath)
	if err == nil {
		t.Fatal("Load() error = nil, want unknown node failure")
	}
}

func TestLoadNodesConfigRejectsBrokenInput(t *testing.T) {
	dir := t.TempDir()
	nodesPath := filepath.Join(dir, "nodes.yaml")
	if err := os.WriteFile(nodesPath, []byte(`nodes:
  - display_name: "broken"
`), 0o600); err != nil {
		t.Fatalf("WriteFile(nodes) error = %v", err)
	}
	if _, err := LoadNodesConfig(nodesPath); err == nil {
		t.Fatal("LoadNodesConfig() error = nil, want missing node_id failure")
	}
}
