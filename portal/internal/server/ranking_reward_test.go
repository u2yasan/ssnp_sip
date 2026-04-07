package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/notifier"
	"github.com/u2yasan/ssnp_sip/portal/internal/store"
)

func TestQualifiedNodeGeneratesBasePerformanceAndRanking(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()
	now := time.Now().UTC()
	dateUTC := now.Format("2006-01-02")

	node, _ := srv.store.GetNode("node-abc")
	node.LastHeartbeatTimestamp = now.Format(time.RFC3339)
	srv.store.SaveNode(node)
	srv.store.SaveCheckEvent(store.CheckEvent{
		EventID:       "check-ranking-001",
		NodeID:        "node-abc",
		OverallPassed: true,
		CheckedAt:     now.Format(time.RFC3339),
	})
	for _, payload := range []map[string]any{
		{
			"schema_version":             "1",
			"probe_id":                   "probe-ranking-1",
			"node_id":                    "node-abc",
			"region_id":                  "ap-sg-1",
			"observed_at":                now.Format(time.RFC3339),
			"endpoint":                   "https://node.example.net:3001",
			"availability_up":            true,
			"finalized_lag_blocks":       1,
			"chain_lag_blocks":           2,
			"source_height":              100,
			"peer_height":                102,
			"measurement_window_seconds": 30,
		},
		{
			"schema_version":             "1",
			"probe_id":                   "probe-ranking-2",
			"node_id":                    "node-abc",
			"region_id":                  "us-va-1",
			"observed_at":                now.Add(5 * time.Second).Format(time.RFC3339),
			"endpoint":                   "https://node.example.net:3001",
			"availability_up":            true,
			"finalized_lag_blocks":       2,
			"chain_lag_blocks":           5,
			"source_height":              101,
			"peer_height":                103,
			"measurement_window_seconds": 30,
		},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", payload); rec.Code != http.StatusOK {
			t.Fatalf("probe status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/voting-key-evidence", map[string]any{
		"node_id":                    "node-abc",
		"evidence_ref":               "vk-ranking-001",
		"observed_at":                now.Add(10 * time.Second).Format(time.RFC3339),
		"current_epoch":              12,
		"voting_key_present":         true,
		"voting_key_valid_for_epoch": true,
		"source":                     "external_probe",
	}); rec.Code != http.StatusOK {
		t.Fatalf("voting key evidence status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	record, ok := srv.store.GetBasePerformanceRecord("node-abc", dateUTC)
	if !ok {
		t.Fatal("expected base performance record")
	}
	if record.BasePerformanceScore != 70 {
		t.Fatalf("base performance = %#v, want score 70", record)
	}
	rankings := srv.store.ListRankingRecordsByDate(dateUTC)
	if len(rankings) != 1 {
		t.Fatalf("ranking count = %d, want 1", len(rankings))
	}
	if rankings[0].NodeID != "node-abc" || rankings[0].RankPosition != 1 || rankings[0].TotalScore != 49 {
		t.Fatalf("ranking = %#v, want node-abc rank 1 score 49", rankings[0])
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/rankings/"+dateUTC, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ranking read status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		DateUTC string                `json:"date_utc"`
		Items   []store.RankingRecord `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload.DateUTC != dateUTC || len(payload.Items) != 1 || payload.Items[0].NodeID != "node-abc" {
		t.Fatalf("payload = %#v, want ranking payload for node-abc", payload)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/reward-eligibility/"+dateUTC, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reward eligibility read status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var rewardPayload struct {
		DateUTC string                          `json:"date_utc"`
		Items   []store.RewardEligibilityRecord `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rewardPayload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if rewardPayload.DateUTC != dateUTC || len(rewardPayload.Items) != 1 || !rewardPayload.Items[0].RewardEligible {
		t.Fatalf("reward payload = %#v, want eligible node-abc", rewardPayload)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/anti-concentration-evidence/"+dateUTC, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("anti-concentration evidence read status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/public-node-status/"+dateUTC, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("public node status read status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var publicPayload struct {
		DateUTC string           `json:"date_utc"`
		Items   []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &publicPayload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if publicPayload.DateUTC != dateUTC || len(publicPayload.Items) != 1 {
		t.Fatalf("public payload = %#v, want one item", publicPayload)
	}
	if _, ok := publicPayload.Items[0]["failure_reasons"]; ok {
		t.Fatalf("public payload leaked failure_reasons: %#v", publicPayload.Items[0])
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/operator-node-status/node-abc/"+dateUTC, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("operator node status read status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var operatorPayload struct {
		NodeID         string   `json:"node_id"`
		DateUTC        string   `json:"date_utc"`
		Qualified      bool     `json:"qualified"`
		RewardEligible bool     `json:"reward_eligible"`
		FailureReasons []string `json:"failure_reasons"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &operatorPayload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if operatorPayload.NodeID != "node-abc" || operatorPayload.DateUTC != dateUTC || !operatorPayload.Qualified || !operatorPayload.RewardEligible {
		t.Fatalf("operator payload = %#v, want qualified eligible node-abc", operatorPayload)
	}
}

func TestRankingOrdersQualifiedNodesDeterministically(t *testing.T) {
	dir := t.TempDir()
	nodesPath := filepath.Join(dir, "nodes.yaml")
	if err := os.WriteFile(nodesPath, []byte(`nodes:
  - node_id: "node-abc"
    display_name: "Node ABC"
    operator_email: "ops@example.invalid"
    enabled: true
  - node_id: "node-def"
    display_name: "Node DEF"
    operator_email: "ops@example.invalid"
    enabled: true
`), 0o600); err != nil {
		t.Fatalf("WriteFile(nodes) error = %v", err)
	}
	srv, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              testPolicyPath(),
		NodesConfigPath:         nodesPath,
		StatePath:               filepath.Join(dir, "portal-state.json"),
		AllowedClockSkewSeconds: 300,
		NotificationEmailTo:     "ops@example.invalid",
		Notifier:                &notifier.Recorder{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := srv.Handler()
	now := time.Now().UTC()
	dateUTC := now.Format("2006-01-02")
	for _, nodeID := range []string{"node-abc", "node-def"} {
		node, _ := srv.store.GetNode(nodeID)
		node.ValidatedRegistrationAt = now.Add(-(observationWindow + time.Hour)).Format(time.RFC3339)
		node.LastHeartbeatTimestamp = now.Format(time.RFC3339)
		srv.store.SaveNode(node)
		srv.store.SaveHeartbeatEvent(store.HeartbeatEvent{
			NodeID:             nodeID,
			HeartbeatTimestamp: now.Add(-10 * time.Minute).Format(time.RFC3339),
			SequenceNumber:     1,
		})
		srv.store.SaveHeartbeatEvent(store.HeartbeatEvent{
			NodeID:             nodeID,
			HeartbeatTimestamp: now.Add(-5 * time.Minute).Format(time.RFC3339),
			SequenceNumber:     2,
		})
		srv.store.SaveCheckEvent(store.CheckEvent{
			EventID:       "check-" + nodeID,
			NodeID:        nodeID,
			OverallPassed: true,
			CheckedAt:     now.Format(time.RFC3339),
		})
	}
	probes := []map[string]any{
		{
			"schema_version":             "1",
			"probe_id":                   "probe-abc-1",
			"node_id":                    "node-abc",
			"region_id":                  "ap-sg-1",
			"observed_at":                now.Format(time.RFC3339),
			"endpoint":                   "https://node-abc.example.net:3001",
			"availability_up":            true,
			"finalized_lag_blocks":       1,
			"chain_lag_blocks":           2,
			"source_height":              100,
			"peer_height":                102,
			"measurement_window_seconds": 30,
		},
		{
			"schema_version":             "1",
			"probe_id":                   "probe-abc-2",
			"node_id":                    "node-abc",
			"region_id":                  "us-va-1",
			"observed_at":                now.Add(5 * time.Second).Format(time.RFC3339),
			"endpoint":                   "https://node-abc.example.net:3001",
			"availability_up":            true,
			"finalized_lag_blocks":       2,
			"chain_lag_blocks":           5,
			"source_height":              101,
			"peer_height":                103,
			"measurement_window_seconds": 30,
		},
		{
			"schema_version":             "1",
			"probe_id":                   "probe-def-1",
			"node_id":                    "node-def",
			"region_id":                  "ap-sg-1",
			"observed_at":                now.Format(time.RFC3339),
			"endpoint":                   "https://node-def.example.net:3001",
			"availability_up":            true,
			"finalized_lag_blocks":       2,
			"chain_lag_blocks":           5,
			"source_height":              100,
			"peer_height":                102,
			"measurement_window_seconds": 30,
		},
		{
			"schema_version":             "1",
			"probe_id":                   "probe-def-2",
			"node_id":                    "node-def",
			"region_id":                  "us-va-1",
			"observed_at":                now.Add(5 * time.Second).Format(time.RFC3339),
			"endpoint":                   "https://node-def.example.net:3001",
			"availability_up":            true,
			"finalized_lag_blocks":       2,
			"chain_lag_blocks":           5,
			"source_height":              101,
			"peer_height":                103,
			"measurement_window_seconds": 30,
		},
	}
	for _, probe := range probes {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", probe); rec.Code != http.StatusOK {
			t.Fatalf("probe status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	for _, evidence := range []map[string]any{
		{
			"node_id":                    "node-abc",
			"evidence_ref":               "vk-abc",
			"observed_at":                now.Add(10 * time.Second).Format(time.RFC3339),
			"current_epoch":              12,
			"voting_key_present":         true,
			"voting_key_valid_for_epoch": true,
			"source":                     "external_probe",
		},
		{
			"node_id":                    "node-def",
			"evidence_ref":               "vk-def",
			"observed_at":                now.Add(10 * time.Second).Format(time.RFC3339),
			"current_epoch":              12,
			"voting_key_present":         true,
			"voting_key_valid_for_epoch": true,
			"source":                     "external_probe",
		},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/voting-key-evidence", evidence); rec.Code != http.StatusOK {
			t.Fatalf("voting key evidence status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}

	rankings := srv.store.ListRankingRecordsByDate(dateUTC)
	if len(rankings) != 2 {
		t.Fatalf("ranking count = %d, want 2", len(rankings))
	}
	if rankings[0].NodeID != "node-abc" || rankings[0].RankPosition != 1 {
		t.Fatalf("rankings[0] = %#v, want node-abc rank 1", rankings[0])
	}
	if rankings[1].NodeID != "node-def" || rankings[1].RankPosition != 2 {
		t.Fatalf("rankings[1] = %#v, want node-def rank 2", rankings[1])
	}
	if rankings[0].TotalScore != rankings[1].TotalScore {
		t.Fatalf("scores = %#v, want tied scores for deterministic fallback", rankings)
	}
	for _, evidence := range []map[string]any{
		{
			"node_id":           "node-abc",
			"evidence_ref":      "group-abc",
			"operator_group_id": "operator-1",
			"observed_at":       now.Add(15 * time.Second).Format(time.RFC3339),
			"source":            "manual_review",
		},
		{
			"node_id":           "node-def",
			"evidence_ref":      "group-def",
			"operator_group_id": "operator-1",
			"observed_at":       now.Add(15 * time.Second).Format(time.RFC3339),
			"source":            "manual_review",
		},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/operator-group-evidence", evidence); rec.Code != http.StatusOK {
			t.Fatalf("operator group evidence status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	reward := srv.store.ListRewardEligibilityRecordsByDate(dateUTC)
	if len(reward) != 2 {
		t.Fatalf("reward eligibility = %#v, want two records", reward)
	}
	if !reward[0].RewardEligible || reward[0].OperatorGroupID != "operator-1" {
		t.Fatalf("reward[0] = %#v, want top ranked node eligible in operator-1", reward[0])
	}
	if reward[1].RewardEligible || reward[1].ExclusionReason != "same_operator_group_lower_ranked" {
		t.Fatalf("reward[1] = %#v, want lower ranked node excluded", reward[1])
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public-node-status/"+dateUTC, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("public node status read status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var publicPayload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &publicPayload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(publicPayload.Items) != 2 {
		t.Fatalf("public payload = %#v, want two items", publicPayload)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/operator-node-status/node-def/"+dateUTC, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("operator node status read status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var operatorPayload struct {
		NodeID          string   `json:"node_id"`
		RewardEligible  bool     `json:"reward_eligible"`
		ExclusionReason string   `json:"exclusion_reason"`
		FailureReasons  []string `json:"failure_reasons"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &operatorPayload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if operatorPayload.NodeID != "node-def" || operatorPayload.RewardEligible || operatorPayload.ExclusionReason != "same_operator_group_lower_ranked" {
		t.Fatalf("operator payload = %#v, want excluded node-def", operatorPayload)
	}
}

func TestRankingTieBreakUsesValidatedRegistrationTime(t *testing.T) {
	dir := t.TempDir()
	nodesPath := filepath.Join(dir, "nodes.yaml")
	if err := os.WriteFile(nodesPath, []byte(`nodes:
  - node_id: "node-abc"
    display_name: "Node ABC"
    operator_email: "ops@example.invalid"
    enabled: true
  - node_id: "node-def"
    display_name: "Node DEF"
    operator_email: "ops@example.invalid"
    enabled: true
`), 0o600); err != nil {
		t.Fatalf("WriteFile(nodes) error = %v", err)
	}
	srv, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              testPolicyPath(),
		NodesConfigPath:         nodesPath,
		StatePath:               filepath.Join(dir, "portal-state.json"),
		AllowedClockSkewSeconds: 300,
		NotificationEmailTo:     "ops@example.invalid",
		Notifier:                &notifier.Recorder{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := srv.Handler()
	now := time.Now().UTC()
	dateUTC := now.Format("2006-01-02")
	nodeABC, _ := srv.store.GetNode("node-abc")
	nodeABC.ValidatedRegistrationAt = now.Add(-(observationWindow + 2*time.Hour)).Format(time.RFC3339)
	nodeABC.LastHeartbeatTimestamp = now.Format(time.RFC3339)
	srv.store.SaveNode(nodeABC)
	nodeDEF, _ := srv.store.GetNode("node-def")
	nodeDEF.ValidatedRegistrationAt = now.Add(-(observationWindow + 3*time.Hour)).Format(time.RFC3339)
	nodeDEF.LastHeartbeatTimestamp = now.Format(time.RFC3339)
	srv.store.SaveNode(nodeDEF)
	for _, heartbeat := range []store.HeartbeatEvent{
		{NodeID: "node-abc", HeartbeatTimestamp: now.Add(-10 * time.Minute).Format(time.RFC3339), SequenceNumber: 10},
		{NodeID: "node-abc", HeartbeatTimestamp: now.Add(-5 * time.Minute).Format(time.RFC3339), SequenceNumber: 11},
		{NodeID: "node-def", HeartbeatTimestamp: now.Add(-10 * time.Minute).Format(time.RFC3339), SequenceNumber: 10},
		{NodeID: "node-def", HeartbeatTimestamp: now.Add(-5 * time.Minute).Format(time.RFC3339), SequenceNumber: 11},
	} {
		srv.store.SaveHeartbeatEvent(heartbeat)
	}
	for _, nodeID := range []string{"node-abc", "node-def"} {
		srv.store.SaveCheckEvent(store.CheckEvent{
			EventID:       "check-reg-" + nodeID,
			NodeID:        nodeID,
			OverallPassed: true,
			CheckedAt:     now.Format(time.RFC3339),
		})
	}
	for _, probe := range []map[string]any{
		{
			"schema_version":             "1",
			"probe_id":                   "probe-reg-abc-1",
			"node_id":                    "node-abc",
			"region_id":                  "ap-sg-1",
			"observed_at":                now.Format(time.RFC3339),
			"endpoint":                   "https://node-abc.example.net:3001",
			"availability_up":            true,
			"finalized_lag_blocks":       1,
			"chain_lag_blocks":           2,
			"source_height":              100,
			"peer_height":                102,
			"measurement_window_seconds": 30,
		},
		{
			"schema_version":             "1",
			"probe_id":                   "probe-reg-abc-2",
			"node_id":                    "node-abc",
			"region_id":                  "us-va-1",
			"observed_at":                now.Add(5 * time.Second).Format(time.RFC3339),
			"endpoint":                   "https://node-abc.example.net:3001",
			"availability_up":            true,
			"finalized_lag_blocks":       2,
			"chain_lag_blocks":           5,
			"source_height":              101,
			"peer_height":                103,
			"measurement_window_seconds": 30,
		},
		{
			"schema_version":             "1",
			"probe_id":                   "probe-reg-def-1",
			"node_id":                    "node-def",
			"region_id":                  "ap-sg-1",
			"observed_at":                now.Format(time.RFC3339),
			"endpoint":                   "https://node-def.example.net:3001",
			"availability_up":            true,
			"finalized_lag_blocks":       1,
			"chain_lag_blocks":           2,
			"source_height":              100,
			"peer_height":                102,
			"measurement_window_seconds": 30,
		},
		{
			"schema_version":             "1",
			"probe_id":                   "probe-reg-def-2",
			"node_id":                    "node-def",
			"region_id":                  "us-va-1",
			"observed_at":                now.Add(5 * time.Second).Format(time.RFC3339),
			"endpoint":                   "https://node-def.example.net:3001",
			"availability_up":            true,
			"finalized_lag_blocks":       2,
			"chain_lag_blocks":           5,
			"source_height":              101,
			"peer_height":                103,
			"measurement_window_seconds": 30,
		},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", probe); rec.Code != http.StatusOK {
			t.Fatalf("probe status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	for _, evidence := range []map[string]any{
		{
			"node_id":                    "node-abc",
			"evidence_ref":               "vk-reg-abc",
			"observed_at":                now.Add(10 * time.Second).Format(time.RFC3339),
			"current_epoch":              12,
			"voting_key_present":         true,
			"voting_key_valid_for_epoch": true,
			"source":                     "external_probe",
		},
		{
			"node_id":                    "node-def",
			"evidence_ref":               "vk-reg-def",
			"observed_at":                now.Add(10 * time.Second).Format(time.RFC3339),
			"current_epoch":              12,
			"voting_key_present":         true,
			"voting_key_valid_for_epoch": true,
			"source":                     "external_probe",
		},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/voting-key-evidence", evidence); rec.Code != http.StatusOK {
			t.Fatalf("voting key evidence status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	rankings := srv.store.ListRankingRecordsByDate(dateUTC)
	if len(rankings) != 2 {
		t.Fatalf("ranking count = %d, want 2", len(rankings))
	}
	if rankings[0].NodeID != "node-def" || rankings[1].NodeID != "node-abc" {
		t.Fatalf("rankings = %#v, want earlier validated registration first", rankings)
	}
}

func TestDecentralizationEvidenceAffectsRanking(t *testing.T) {
	dir := t.TempDir()
	nodesPath := filepath.Join(dir, "nodes.yaml")
	if err := os.WriteFile(nodesPath, []byte(`nodes:
  - node_id: "node-abc"
    display_name: "Node ABC"
    operator_email: "ops@example.invalid"
    enabled: true
  - node_id: "node-def"
    display_name: "Node DEF"
    operator_email: "ops@example.invalid"
    enabled: true
`), 0o600); err != nil {
		t.Fatalf("WriteFile(nodes) error = %v", err)
	}
	srv, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              testPolicyPath(),
		NodesConfigPath:         nodesPath,
		StatePath:               filepath.Join(dir, "portal-state.json"),
		AllowedClockSkewSeconds: 300,
		NotificationEmailTo:     "ops@example.invalid",
		Notifier:                &notifier.Recorder{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := srv.Handler()
	now := time.Now().UTC()
	dateUTC := now.Format("2006-01-02")
	for _, nodeID := range []string{"node-abc", "node-def"} {
		node, _ := srv.store.GetNode(nodeID)
		node.ValidatedRegistrationAt = now.Add(-(observationWindow + time.Hour)).Format(time.RFC3339)
		node.LastHeartbeatTimestamp = now.Format(time.RFC3339)
		srv.store.SaveNode(node)
		srv.store.SaveHeartbeatEvent(store.HeartbeatEvent{NodeID: nodeID, HeartbeatTimestamp: now.Add(-10 * time.Minute).Format(time.RFC3339), SequenceNumber: 1})
		srv.store.SaveHeartbeatEvent(store.HeartbeatEvent{NodeID: nodeID, HeartbeatTimestamp: now.Add(-5 * time.Minute).Format(time.RFC3339), SequenceNumber: 2})
		srv.store.SaveCheckEvent(store.CheckEvent{EventID: "check-d-" + nodeID, NodeID: nodeID, OverallPassed: true, CheckedAt: now.Format(time.RFC3339)})
	}
	for _, probe := range []map[string]any{
		{"schema_version": "1", "probe_id": "probe-d-abc-1", "node_id": "node-abc", "region_id": "ap-sg-1", "observed_at": now.Format(time.RFC3339), "endpoint": "https://node-abc.example.net:3001", "availability_up": true, "finalized_lag_blocks": 1, "chain_lag_blocks": 2, "source_height": 100, "peer_height": 102, "measurement_window_seconds": 30},
		{"schema_version": "1", "probe_id": "probe-d-abc-2", "node_id": "node-abc", "region_id": "us-va-1", "observed_at": now.Add(5 * time.Second).Format(time.RFC3339), "endpoint": "https://node-abc.example.net:3001", "availability_up": true, "finalized_lag_blocks": 2, "chain_lag_blocks": 5, "source_height": 101, "peer_height": 103, "measurement_window_seconds": 30},
		{"schema_version": "1", "probe_id": "probe-d-def-1", "node_id": "node-def", "region_id": "ap-sg-1", "observed_at": now.Format(time.RFC3339), "endpoint": "https://node-def.example.net:3001", "availability_up": true, "finalized_lag_blocks": 1, "chain_lag_blocks": 2, "source_height": 100, "peer_height": 102, "measurement_window_seconds": 30},
		{"schema_version": "1", "probe_id": "probe-d-def-2", "node_id": "node-def", "region_id": "us-va-1", "observed_at": now.Add(5 * time.Second).Format(time.RFC3339), "endpoint": "https://node-def.example.net:3001", "availability_up": true, "finalized_lag_blocks": 2, "chain_lag_blocks": 5, "source_height": 101, "peer_height": 103, "measurement_window_seconds": 30},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", probe); rec.Code != http.StatusOK {
			t.Fatalf("probe status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	for _, evidence := range []map[string]any{
		{"node_id": "node-abc", "evidence_ref": "vk-d-abc", "observed_at": now.Add(10 * time.Second).Format(time.RFC3339), "current_epoch": 12, "voting_key_present": true, "voting_key_valid_for_epoch": true, "source": "external_probe"},
		{"node_id": "node-def", "evidence_ref": "vk-d-def", "observed_at": now.Add(10 * time.Second).Format(time.RFC3339), "current_epoch": 12, "voting_key_present": true, "voting_key_valid_for_epoch": true, "source": "external_probe"},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/voting-key-evidence", evidence); rec.Code != http.StatusOK {
			t.Fatalf("voting key evidence status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	rankings := srv.store.ListRankingRecordsByDate(dateUTC)
	if len(rankings) != 2 {
		t.Fatalf("ranking count = %d, want 2", len(rankings))
	}
	if rankings[0].TotalScore != 49 || rankings[1].TotalScore != 49 {
		t.Fatalf("rankings = %#v, want both nodes at base-only score 49", rankings)
	}

	rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/decentralization-evidence", map[string]any{
		"node_id":      "node-abc",
		"evidence_ref": "d-abc-1",
		"observed_at":  now.Add(15 * time.Second).Format(time.RFC3339),
		"country_code": "SG",
		"asn":          64501,
		"provider_id":  "provider-b",
		"source":       "manual_review",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("decentralization evidence status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	rankings = srv.store.ListRankingRecordsByDate(dateUTC)
	if rankings[0].NodeID != "node-abc" {
		t.Fatalf("rankings = %#v, want node-abc promoted by D score", rankings)
	}
	if rankings[0].DecentralizationScore != 30 {
		t.Fatalf("rankings[0] = %#v, want full decentralization score", rankings[0])
	}
	if rankings[0].TotalScore <= rankings[1].TotalScore {
		t.Fatalf("rankings = %#v, want node-abc total score above node-def", rankings)
	}
}

func TestDomainEvidenceExcludesLowerRankedNodeFromRewardEligibility(t *testing.T) {
	dir := t.TempDir()
	nodesPath := filepath.Join(dir, "nodes.yaml")
	if err := os.WriteFile(nodesPath, []byte(`nodes:
  - node_id: "node-abc"
    display_name: "Node ABC"
    operator_email: "ops@example.invalid"
    enabled: true
  - node_id: "node-def"
    display_name: "Node DEF"
    operator_email: "ops@example.invalid"
    enabled: true
`), 0o600); err != nil {
		t.Fatalf("WriteFile(nodes) error = %v", err)
	}
	srv, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              testPolicyPath(),
		NodesConfigPath:         nodesPath,
		StatePath:               filepath.Join(dir, "portal-state.json"),
		AllowedClockSkewSeconds: 300,
		NotificationEmailTo:     "ops@example.invalid",
		Notifier:                &notifier.Recorder{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := srv.Handler()
	now := time.Now().UTC()
	dateUTC := now.Format("2006-01-02")
	for _, nodeID := range []string{"node-abc", "node-def"} {
		node, _ := srv.store.GetNode(nodeID)
		node.ValidatedRegistrationAt = now.Add(-(observationWindow + time.Hour)).Format(time.RFC3339)
		node.LastHeartbeatTimestamp = now.Format(time.RFC3339)
		srv.store.SaveNode(node)
		srv.store.SaveHeartbeatEvent(store.HeartbeatEvent{NodeID: nodeID, HeartbeatTimestamp: now.Add(-10 * time.Minute).Format(time.RFC3339), SequenceNumber: 1})
		srv.store.SaveHeartbeatEvent(store.HeartbeatEvent{NodeID: nodeID, HeartbeatTimestamp: now.Add(-5 * time.Minute).Format(time.RFC3339), SequenceNumber: 2})
		srv.store.SaveCheckEvent(store.CheckEvent{EventID: "check-domain-" + nodeID, NodeID: nodeID, OverallPassed: true, CheckedAt: now.Format(time.RFC3339)})
	}
	for _, probe := range []map[string]any{
		{"schema_version": "1", "probe_id": "probe-domain-abc-1", "node_id": "node-abc", "region_id": "ap-sg-1", "observed_at": now.Format(time.RFC3339), "endpoint": "https://node-abc.example.net:3001", "availability_up": true, "finalized_lag_blocks": 1, "chain_lag_blocks": 2, "source_height": 100, "peer_height": 102, "measurement_window_seconds": 30},
		{"schema_version": "1", "probe_id": "probe-domain-abc-2", "node_id": "node-abc", "region_id": "us-va-1", "observed_at": now.Add(5 * time.Second).Format(time.RFC3339), "endpoint": "https://node-abc.example.net:3001", "availability_up": true, "finalized_lag_blocks": 2, "chain_lag_blocks": 5, "source_height": 101, "peer_height": 103, "measurement_window_seconds": 30},
		{"schema_version": "1", "probe_id": "probe-domain-def-1", "node_id": "node-def", "region_id": "ap-sg-1", "observed_at": now.Format(time.RFC3339), "endpoint": "https://node-def.example.net:3001", "availability_up": true, "finalized_lag_blocks": 2, "chain_lag_blocks": 5, "source_height": 100, "peer_height": 102, "measurement_window_seconds": 30},
		{"schema_version": "1", "probe_id": "probe-domain-def-2", "node_id": "node-def", "region_id": "us-va-1", "observed_at": now.Add(5 * time.Second).Format(time.RFC3339), "endpoint": "https://node-def.example.net:3001", "availability_up": true, "finalized_lag_blocks": 2, "chain_lag_blocks": 5, "source_height": 101, "peer_height": 103, "measurement_window_seconds": 30},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", probe); rec.Code != http.StatusOK {
			t.Fatalf("probe status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	for _, evidence := range []map[string]any{
		{"node_id": "node-abc", "evidence_ref": "vk-domain-abc", "observed_at": now.Add(10 * time.Second).Format(time.RFC3339), "current_epoch": 12, "voting_key_present": true, "voting_key_valid_for_epoch": true, "source": "external_probe"},
		{"node_id": "node-def", "evidence_ref": "vk-domain-def", "observed_at": now.Add(10 * time.Second).Format(time.RFC3339), "current_epoch": 12, "voting_key_present": true, "voting_key_valid_for_epoch": true, "source": "external_probe"},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/voting-key-evidence", evidence); rec.Code != http.StatusOK {
			t.Fatalf("voting key evidence status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	for _, evidence := range []map[string]any{
		{"node_id": "node-abc", "evidence_ref": "domain-abc", "observed_at": now.Add(15 * time.Second).Format(time.RFC3339), "registrable_domain": "example.net", "source": "manual_review"},
		{"node_id": "node-def", "evidence_ref": "domain-def", "observed_at": now.Add(15 * time.Second).Format(time.RFC3339), "registrable_domain": "example.net", "source": "manual_review"},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/domain-evidence", evidence); rec.Code != http.StatusOK {
			t.Fatalf("domain evidence status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}

	reward := srv.store.ListRewardEligibilityRecordsByDate(dateUTC)
	if len(reward) != 2 {
		t.Fatalf("reward eligibility = %#v, want two records", reward)
	}
	if !reward[0].RewardEligible {
		t.Fatalf("reward[0] = %#v, want top ranked node still eligible", reward[0])
	}
	if reward[1].RewardEligible || reward[1].ExclusionReason != "same_registrable_domain_lower_ranked" {
		t.Fatalf("reward[1] = %#v, want lower ranked node excluded by domain", reward[1])
	}
	if reward[1].ExcludedRegistrableDomain != "example.net" {
		t.Fatalf("reward[1] = %#v, want excluded registrable domain provenance", reward[1])
	}
}

func TestSharedControlPlaneEvidenceExcludesLowerRankedNodeFromRewardEligibility(t *testing.T) {
	dir := t.TempDir()
	nodesPath := filepath.Join(dir, "nodes.yaml")
	if err := os.WriteFile(nodesPath, []byte(`nodes:
  - node_id: "node-abc"
    display_name: "Node ABC"
    operator_email: "ops@example.invalid"
    enabled: true
  - node_id: "node-def"
    display_name: "Node DEF"
    operator_email: "ops@example.invalid"
    enabled: true
`), 0o600); err != nil {
		t.Fatalf("WriteFile(nodes) error = %v", err)
	}
	srv, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              testPolicyPath(),
		NodesConfigPath:         nodesPath,
		StatePath:               filepath.Join(dir, "portal-state.json"),
		AllowedClockSkewSeconds: 300,
		NotificationEmailTo:     "ops@example.invalid",
		Notifier:                &notifier.Recorder{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := srv.Handler()
	now := time.Now().UTC()
	dateUTC := now.Format("2006-01-02")
	for _, nodeID := range []string{"node-abc", "node-def"} {
		node, _ := srv.store.GetNode(nodeID)
		node.ValidatedRegistrationAt = now.Add(-(observationWindow + time.Hour)).Format(time.RFC3339)
		node.LastHeartbeatTimestamp = now.Format(time.RFC3339)
		srv.store.SaveNode(node)
		srv.store.SaveHeartbeatEvent(store.HeartbeatEvent{NodeID: nodeID, HeartbeatTimestamp: now.Add(-10 * time.Minute).Format(time.RFC3339), SequenceNumber: 1})
		srv.store.SaveHeartbeatEvent(store.HeartbeatEvent{NodeID: nodeID, HeartbeatTimestamp: now.Add(-5 * time.Minute).Format(time.RFC3339), SequenceNumber: 2})
		srv.store.SaveCheckEvent(store.CheckEvent{EventID: "check-cp-" + nodeID, NodeID: nodeID, OverallPassed: true, CheckedAt: now.Format(time.RFC3339)})
	}
	for _, probe := range []map[string]any{
		{"schema_version": "1", "probe_id": "probe-cp-abc-1", "node_id": "node-abc", "region_id": "ap-sg-1", "observed_at": now.Format(time.RFC3339), "endpoint": "https://node-abc.example.net:3001", "availability_up": true, "finalized_lag_blocks": 1, "chain_lag_blocks": 2, "source_height": 100, "peer_height": 102, "measurement_window_seconds": 30},
		{"schema_version": "1", "probe_id": "probe-cp-abc-2", "node_id": "node-abc", "region_id": "us-va-1", "observed_at": now.Add(5 * time.Second).Format(time.RFC3339), "endpoint": "https://node-abc.example.net:3001", "availability_up": true, "finalized_lag_blocks": 2, "chain_lag_blocks": 5, "source_height": 101, "peer_height": 103, "measurement_window_seconds": 30},
		{"schema_version": "1", "probe_id": "probe-cp-def-1", "node_id": "node-def", "region_id": "ap-sg-1", "observed_at": now.Format(time.RFC3339), "endpoint": "https://node-def.example.net:3001", "availability_up": true, "finalized_lag_blocks": 2, "chain_lag_blocks": 5, "source_height": 100, "peer_height": 102, "measurement_window_seconds": 30},
		{"schema_version": "1", "probe_id": "probe-cp-def-2", "node_id": "node-def", "region_id": "us-va-1", "observed_at": now.Add(5 * time.Second).Format(time.RFC3339), "endpoint": "https://node-def.example.net:3001", "availability_up": true, "finalized_lag_blocks": 2, "chain_lag_blocks": 5, "source_height": 101, "peer_height": 103, "measurement_window_seconds": 30},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", probe); rec.Code != http.StatusOK {
			t.Fatalf("probe status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	for _, evidence := range []map[string]any{
		{"node_id": "node-abc", "evidence_ref": "vk-cp-abc", "observed_at": now.Add(10 * time.Second).Format(time.RFC3339), "current_epoch": 12, "voting_key_present": true, "voting_key_valid_for_epoch": true, "source": "external_probe"},
		{"node_id": "node-def", "evidence_ref": "vk-cp-def", "observed_at": now.Add(10 * time.Second).Format(time.RFC3339), "current_epoch": 12, "voting_key_present": true, "voting_key_valid_for_epoch": true, "source": "external_probe"},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/voting-key-evidence", evidence); rec.Code != http.StatusOK {
			t.Fatalf("voting key evidence status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	for _, evidence := range []map[string]any{
		{"node_id": "node-abc", "evidence_ref": "cp-abc", "observed_at": now.Add(15 * time.Second).Format(time.RFC3339), "control_plane_id": "provider-x-ops", "classification": "managed_provider", "source": "manual_review"},
		{"node_id": "node-def", "evidence_ref": "cp-def", "observed_at": now.Add(15 * time.Second).Format(time.RFC3339), "control_plane_id": "provider-x-ops", "classification": "shared_certificate_admin", "source": "manual_review"},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/shared-control-plane-evidence", evidence); rec.Code != http.StatusOK {
			t.Fatalf("shared control plane evidence status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}

	reward := srv.store.ListRewardEligibilityRecordsByDate(dateUTC)
	if len(reward) != 2 {
		t.Fatalf("reward eligibility = %#v, want two records", reward)
	}
	if !reward[0].RewardEligible {
		t.Fatalf("reward[0] = %#v, want top ranked node still eligible", reward[0])
	}
	if reward[1].RewardEligible || reward[1].ExclusionReason != "same_shared_control_plane_lower_ranked" {
		t.Fatalf("reward[1] = %#v, want lower ranked node excluded by shared control plane", reward[1])
	}
	if reward[1].ExcludedControlPlaneID != "provider-x-ops" || reward[1].ExcludedClassification != "shared_certificate_admin" {
		t.Fatalf("reward[1] = %#v, want shared control plane provenance", reward[1])
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anti-concentration-evidence/"+dateUTC, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("anti-concentration evidence read status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var evidencePayload struct {
		DateUTC string                          `json:"date_utc"`
		Items   []antiConcentrationEvidenceView `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &evidencePayload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if evidencePayload.DateUTC != dateUTC || len(evidencePayload.Items) != 2 {
		t.Fatalf("evidence payload = %#v, want two anti-concentration evidence items", evidencePayload)
	}
}

func TestRewardAllocationsUseParticipationRateAndBandSplit(t *testing.T) {
	dir := t.TempDir()
	nodesPath := filepath.Join(dir, "nodes.yaml")
	if err := os.WriteFile(nodesPath, []byte(`nodes:
  - node_id: "node-abc"
    display_name: "Node ABC"
    operator_email: "ops@example.invalid"
    enabled: true
  - node_id: "node-def"
    display_name: "Node DEF"
    operator_email: "ops@example.invalid"
    enabled: true
`), 0o600); err != nil {
		t.Fatalf("WriteFile(nodes) error = %v", err)
	}
	srv, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              testPolicyPath(),
		NodesConfigPath:         nodesPath,
		StatePath:               filepath.Join(dir, "portal-state.json"),
		AllowedClockSkewSeconds: 300,
		NotificationEmailTo:     "ops@example.invalid",
		NominalDailyPool:        1000,
		Notifier:                &notifier.Recorder{},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	handler := srv.Handler()
	now := time.Now().UTC()
	dateUTC := now.Format("2006-01-02")
	for _, nodeID := range []string{"node-abc", "node-def"} {
		node, _ := srv.store.GetNode(nodeID)
		node.ValidatedRegistrationAt = now.Add(-(observationWindow + time.Hour)).Format(time.RFC3339)
		node.LastHeartbeatTimestamp = now.Format(time.RFC3339)
		srv.store.SaveNode(node)
		srv.store.SaveHeartbeatEvent(store.HeartbeatEvent{NodeID: nodeID, HeartbeatTimestamp: now.Add(-10 * time.Minute).Format(time.RFC3339), SequenceNumber: 1})
		srv.store.SaveHeartbeatEvent(store.HeartbeatEvent{NodeID: nodeID, HeartbeatTimestamp: now.Add(-5 * time.Minute).Format(time.RFC3339), SequenceNumber: 2})
		srv.store.SaveCheckEvent(store.CheckEvent{EventID: "check-reward-" + nodeID, NodeID: nodeID, OverallPassed: true, CheckedAt: now.Format(time.RFC3339)})
	}
	for _, probe := range []map[string]any{
		{"schema_version": "1", "probe_id": "probe-reward-abc-1", "node_id": "node-abc", "region_id": "ap-sg-1", "observed_at": now.Format(time.RFC3339), "endpoint": "https://node-abc.example.net:3001", "availability_up": true, "finalized_lag_blocks": 1, "chain_lag_blocks": 2, "source_height": 100, "peer_height": 102, "measurement_window_seconds": 30},
		{"schema_version": "1", "probe_id": "probe-reward-abc-2", "node_id": "node-abc", "region_id": "us-va-1", "observed_at": now.Add(5 * time.Second).Format(time.RFC3339), "endpoint": "https://node-abc.example.net:3001", "availability_up": true, "finalized_lag_blocks": 2, "chain_lag_blocks": 5, "source_height": 101, "peer_height": 103, "measurement_window_seconds": 30},
		{"schema_version": "1", "probe_id": "probe-reward-def-1", "node_id": "node-def", "region_id": "ap-sg-1", "observed_at": now.Format(time.RFC3339), "endpoint": "https://node-def.example.net:3001", "availability_up": true, "finalized_lag_blocks": 2, "chain_lag_blocks": 5, "source_height": 100, "peer_height": 102, "measurement_window_seconds": 30},
		{"schema_version": "1", "probe_id": "probe-reward-def-2", "node_id": "node-def", "region_id": "us-va-1", "observed_at": now.Add(5 * time.Second).Format(time.RFC3339), "endpoint": "https://node-def.example.net:3001", "availability_up": true, "finalized_lag_blocks": 2, "chain_lag_blocks": 5, "source_height": 101, "peer_height": 103, "measurement_window_seconds": 30},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", probe); rec.Code != http.StatusOK {
			t.Fatalf("probe status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	for _, evidence := range []map[string]any{
		{"node_id": "node-abc", "evidence_ref": "vk-reward-abc", "observed_at": now.Add(10 * time.Second).Format(time.RFC3339), "current_epoch": 12, "voting_key_present": true, "voting_key_valid_for_epoch": true, "source": "external_probe"},
		{"node_id": "node-def", "evidence_ref": "vk-reward-def", "observed_at": now.Add(10 * time.Second).Format(time.RFC3339), "current_epoch": 12, "voting_key_present": true, "voting_key_valid_for_epoch": true, "source": "external_probe"},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/voting-key-evidence", evidence); rec.Code != http.StatusOK {
			t.Fatalf("voting key evidence status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}

	allocations := srv.store.ListRewardAllocationRecordsByDate(dateUTC)
	if len(allocations) != 2 {
		t.Fatalf("allocations = %#v, want two records", allocations)
	}
	for _, allocation := range allocations {
		if allocation.QualifiedNodeCount != 2 || allocation.ParticipationRate != 0.30 {
			t.Fatalf("allocation = %#v, want N=2 and participation rate 0.30", allocation)
		}
		if allocation.DistributedPool != 300 || allocation.ReservePool != 700 {
			t.Fatalf("allocation = %#v, want distributed 300 reserve 700", allocation)
		}
		if allocation.BandLabel != "1-5" || allocation.BandEligibleCount != 2 || allocation.BandPoolAmount != 75 {
			t.Fatalf("allocation = %#v, want rank band 1-5 with pool 75 split across 2", allocation)
		}
		if allocation.RewardAmount != 37.5 {
			t.Fatalf("allocation = %#v, want reward amount 37.5", allocation)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reward-allocations/"+dateUTC, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reward allocation read status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		DateUTC string                         `json:"date_utc"`
		Items   []store.RewardAllocationRecord `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload.DateUTC != dateUTC || len(payload.Items) != 2 {
		t.Fatalf("payload = %#v, want two reward allocation items", payload)
	}
}

func TestHistoricalQualificationRecomputeUsesSameDayEvidence(t *testing.T) {
	srv := newTestServer(t)
	dayOne := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	dayTwo := dayOne.Add(24 * time.Hour)

	node, _ := srv.store.GetNode("node-abc")
	node.ValidatedRegistrationAt = dayOne.Add(-observationWindow - time.Hour).Format(time.RFC3339)
	node.LastHeartbeatTimestamp = dayTwo.Format(time.RFC3339)
	srv.store.SaveNode(node)
	srv.store.SaveHeartbeatEvent(store.HeartbeatEvent{
		NodeID:             "node-abc",
		HeartbeatTimestamp: dayOne.Add(13*time.Hour + 50*time.Minute).Format(time.RFC3339),
		SequenceNumber:     1,
	})
	srv.store.SaveHeartbeatEvent(store.HeartbeatEvent{
		NodeID:             "node-abc",
		HeartbeatTimestamp: dayOne.Add(13*time.Hour + 55*time.Minute).Format(time.RFC3339),
		SequenceNumber:     2,
	})
	srv.store.SaveHeartbeatEvent(store.HeartbeatEvent{
		NodeID:             "node-abc",
		HeartbeatTimestamp: dayTwo.Format(time.RFC3339),
		SequenceNumber:     3,
	})
	srv.store.SaveCheckEvent(store.CheckEvent{
		EventID:       "check-day-one",
		NodeID:        "node-abc",
		OverallPassed: true,
		CheckedAt:     dayOne.Format(time.RFC3339),
	})
	srv.store.SaveCheckEvent(store.CheckEvent{
		EventID:       "check-day-two",
		NodeID:        "node-abc",
		OverallPassed: false,
		CheckedAt:     dayTwo.Format(time.RFC3339),
	})
	if !srv.store.SaveVotingKeyEvidence(store.VotingKeyEvidence{
		EvidenceRef:            "vk-day-one",
		NodeID:                 "node-abc",
		ObservedAt:             dayOne.Add(5 * time.Minute).Format(time.RFC3339),
		CurrentEpoch:           10,
		VotingKeyPresent:       true,
		VotingKeyValidForEpoch: true,
		Source:                 "external_probe",
	}) {
		t.Fatal("expected day-one voting key evidence save")
	}
	if !srv.store.SaveVotingKeyEvidence(store.VotingKeyEvidence{
		EvidenceRef:            "vk-day-two",
		NodeID:                 "node-abc",
		ObservedAt:             dayTwo.Add(5 * time.Minute).Format(time.RFC3339),
		CurrentEpoch:           11,
		VotingKeyPresent:       true,
		VotingKeyValidForEpoch: false,
		Source:                 "external_probe",
	}) {
		t.Fatal("expected day-two voting key evidence save")
	}
	for _, probe := range []store.ProbeEvent{
		{
			ProbeID:                  "probe-day-one-1",
			NodeID:                   "node-abc",
			RegionID:                 "ap-sg-1",
			ObservedAt:               dayOne.Format(time.RFC3339),
			Endpoint:                 "https://node.example.net:3001",
			AvailabilityUp:           true,
			FinalizedLagBlocks:       intPtr(1),
			ChainLagBlocks:           intPtr(2),
			SourceHeight:             intPtr(100),
			PeerHeight:               intPtr(102),
			MeasurementWindowSeconds: 30,
		},
		{
			ProbeID:                  "probe-day-one-2",
			NodeID:                   "node-abc",
			RegionID:                 "us-va-1",
			ObservedAt:               dayOne.Add(5 * time.Minute).Format(time.RFC3339),
			Endpoint:                 "https://node.example.net:3001",
			AvailabilityUp:           true,
			FinalizedLagBlocks:       intPtr(2),
			ChainLagBlocks:           intPtr(5),
			SourceHeight:             intPtr(101),
			PeerHeight:               intPtr(103),
			MeasurementWindowSeconds: 30,
		},
	} {
		if !srv.store.SaveProbeEvent(probe) {
			t.Fatalf("duplicate probe save: %#v", probe)
		}
	}

	srv.updateQualificationArtifacts("node-abc", "2026-04-06")
	decision, ok := srv.store.GetQualifiedDecisionRecord("node-abc", "2026-04-06")
	if !ok {
		t.Fatal("expected day-one qualified decision")
	}
	if !decision.Qualified {
		t.Fatalf("decision = %#v, want day-one qualified", decision)
	}
	if containsReason(decision.FailureReasons, "hardware_check_missing") || containsReason(decision.FailureReasons, "voting_key_invalid") {
		t.Fatalf("decision = %#v, want same-day evidence only", decision)
	}
}

func TestReadEndpointsReturnEmptyCollectionsForUnknownDate(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()
	dateUTC := "2026-04-08"

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "rankings", path: "/api/v1/rankings/" + dateUTC},
		{name: "reward eligibility", path: "/api/v1/reward-eligibility/" + dateUTC},
		{name: "anti concentration evidence", path: "/api/v1/anti-concentration-evidence/" + dateUTC},
		{name: "reward allocations", path: "/api/v1/reward-allocations/" + dateUTC},
		{name: "public node status", path: "/api/v1/public-node-status/" + dateUTC},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
			}

			var payload struct {
				DateUTC string            `json:"date_utc"`
				Items   []json.RawMessage `json:"items"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if payload.DateUTC != dateUTC {
				t.Fatalf("date_utc = %q, want %q", payload.DateUTC, dateUTC)
			}
			if len(payload.Items) != 0 {
				t.Fatalf("len(items) = %d, want 0", len(payload.Items))
			}
		})
	}
}

func TestReadEndpointsRejectInvalidMethodAndDate(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	for _, tc := range []struct {
		name      string
		method    string
		path      string
		wantCode  int
		wantError string
	}{
		{name: "rankings invalid method", method: http.MethodPost, path: "/api/v1/rankings/2026-04-07", wantCode: http.StatusBadRequest, wantError: "invalid_method"},
		{name: "reward eligibility invalid method", method: http.MethodPost, path: "/api/v1/reward-eligibility/2026-04-07", wantCode: http.StatusBadRequest, wantError: "invalid_method"},
		{name: "anti concentration evidence invalid method", method: http.MethodPost, path: "/api/v1/anti-concentration-evidence/2026-04-07", wantCode: http.StatusBadRequest, wantError: "invalid_method"},
		{name: "reward allocations invalid method", method: http.MethodPost, path: "/api/v1/reward-allocations/2026-04-07", wantCode: http.StatusBadRequest, wantError: "invalid_method"},
		{name: "public node status invalid method", method: http.MethodPost, path: "/api/v1/public-node-status/2026-04-07", wantCode: http.StatusBadRequest, wantError: "invalid_method"},
		{name: "operator node status invalid method", method: http.MethodPost, path: "/api/v1/operator-node-status/node-abc/2026-04-07", wantCode: http.StatusBadRequest, wantError: "invalid_method"},
		{name: "rankings invalid date", method: http.MethodGet, path: "/api/v1/rankings/not-a-date", wantCode: http.StatusBadRequest, wantError: "missing_required_field"},
		{name: "reward eligibility invalid date", method: http.MethodGet, path: "/api/v1/reward-eligibility/not-a-date", wantCode: http.StatusBadRequest, wantError: "missing_required_field"},
		{name: "anti concentration evidence invalid date", method: http.MethodGet, path: "/api/v1/anti-concentration-evidence/not-a-date", wantCode: http.StatusBadRequest, wantError: "missing_required_field"},
		{name: "reward allocations invalid date", method: http.MethodGet, path: "/api/v1/reward-allocations/not-a-date", wantCode: http.StatusBadRequest, wantError: "missing_required_field"},
		{name: "public node status invalid date", method: http.MethodGet, path: "/api/v1/public-node-status/not-a-date", wantCode: http.StatusBadRequest, wantError: "missing_required_field"},
		{name: "operator node status invalid date", method: http.MethodGet, path: "/api/v1/operator-node-status/node-abc/not-a-date", wantCode: http.StatusBadRequest, wantError: "missing_required_field"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assertErrorResponse(t, rec, tc.wantCode, tc.wantError)
		})
	}
}

func TestOperatorNodeStatusReadErrors(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()
	dateUTC := "2026-04-07"

	req := httptest.NewRequest(http.MethodGet, "/api/v1/operator-node-status/unknown-node/"+dateUTC, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusNotFound, "unknown_node_id")

	req = httptest.NewRequest(http.MethodGet, "/api/v1/operator-node-status/node-abc/"+dateUTC, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assertErrorResponse(t, rec, http.StatusNotFound, "missing_qualified_decision")
}
