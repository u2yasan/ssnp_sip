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
	st.SaveProbeEvent(ProbeEvent{
		ProbeID:                  "probe-001",
		NodeID:                   "node-abc",
		RegionID:                 "ap-sg-1",
		ObservedAt:               "2026-04-06T10:02:00Z",
		Endpoint:                 "https://node.example.net:3001",
		AvailabilityUp:           true,
		FinalizedLagBlocks:       intPtr(1),
		ChainLagBlocks:           intPtr(2),
		SourceHeight:             intPtr(100),
		PeerHeight:               intPtr(102),
		MeasurementWindowSeconds: 30,
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
	if _, ok := reloaded.GetProbeEvent("probe-001"); !ok {
		t.Fatal("expected probe event after reload")
	}
	reloaded.SaveDailyQualificationSummary(DailyQualificationSummary{
		NodeID:                           "node-abc",
		DateUTC:                          "2026-04-06",
		PolicyVersion:                    "2026-04",
		FinalizedLagThresholdBlocks:      2,
		ChainLagThresholdBlocks:          5,
		ValidProbeCount:                  1,
		AvailabilityUpCount:              1,
		AvailabilityRatio:                1,
		FinalizedLagMeasurableCount:      1,
		FinalizedLagWithinThresholdCount: 1,
		FinalizedLagRatio:                1,
		ChainLagMeasurableCount:          1,
		ChainLagWithinThresholdCount:     1,
		ChainLagRatio:                    1,
		RegionCount:                      1,
		AvailabilityPassed:               true,
		FinalizedLagPassed:               true,
		ChainLagPassed:                   true,
		MultiRegionEvidencePassed:        false,
	})
	if got, ok := reloaded.GetDailyQualificationSummary("node-abc", "2026-04-06"); !ok || got.NodeID != "node-abc" {
		t.Fatal("expected daily summary in store")
	}
	reloaded.SaveQualifiedDecisionRecord(QualifiedDecisionRecord{
		NodeID:              "node-abc",
		DateUTC:             "2026-04-06",
		PolicyVersion:       "2026-04",
		ProbeEvidencePassed: false,
		Qualified:           false,
		FailureReasons:      []string{"voting_key_evidence_missing"},
		DecidedAt:           "2026-04-06T10:03:00Z",
	})
	if got, ok := reloaded.GetQualifiedDecisionRecord("node-abc", "2026-04-06"); !ok || got.NodeID != "node-abc" {
		t.Fatal("expected qualified decision in store")
	}
	if !reloaded.SaveVotingKeyEvidence(VotingKeyEvidence{
		EvidenceRef:            "vk-001",
		NodeID:                 "node-abc",
		ObservedAt:             "2026-04-06T10:04:00Z",
		CurrentEpoch:           12,
		VotingKeyPresent:       true,
		VotingKeyValidForEpoch: true,
		Source:                 "external_probe",
	}) {
		t.Fatal("expected voting key evidence save")
	}
	if got, ok := reloaded.GetLatestVotingKeyEvidenceForNode("node-abc"); !ok || got.EvidenceRef != "vk-001" {
		t.Fatal("expected voting key evidence in store")
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

func intPtr(v int) *int {
	return &v
}
