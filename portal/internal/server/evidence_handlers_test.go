package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/store"
)

func TestProbeEventsFlow(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	payload := map[string]any{
		"schema_version":             "1",
		"probe_id":                   "probe-001",
		"node_id":                    "node-abc",
		"region_id":                  "ap-sg-1",
		"observed_at":                time.Now().UTC().Format(time.RFC3339),
		"endpoint":                   "https://node.example.net:3001",
		"availability_up":            true,
		"finalized_lag_blocks":       1,
		"chain_lag_blocks":           2,
		"source_height":              100,
		"peer_height":                102,
		"measurement_window_seconds": 30,
	}
	rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("probe status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := srv.store.GetProbeEvent("probe-001"); !ok {
		t.Fatal("expected probe event in store")
	}
	dateUTC := time.Now().UTC().Format("2006-01-02")
	if _, ok := srv.store.GetDailyQualificationSummary("node-abc", dateUTC); !ok {
		t.Fatal("expected daily summary in store")
	}

	dupRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", payload)
	if dupRec.Code != http.StatusConflict {
		t.Fatalf("duplicate probe status = %d, want 409", dupRec.Code)
	}
}

func TestProbeEventsRejectInvalidPayloads(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	missingFieldPayload := map[string]any{
		"schema_version":             "1",
		"probe_id":                   "probe-missing",
		"node_id":                    "node-abc",
		"region_id":                  "ap-sg-1",
		"observed_at":                time.Now().UTC().Format(time.RFC3339),
		"availability_up":            true,
		"finalized_lag_blocks":       1,
		"chain_lag_blocks":           2,
		"source_height":              100,
		"peer_height":                102,
		"measurement_window_seconds": 30,
	}
	missingRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", missingFieldPayload)
	if missingRec.Code != http.StatusBadRequest {
		t.Fatalf("missing field status = %d, want 400", missingRec.Code)
	}

	negativeLagPayload := map[string]any{
		"schema_version":             "1",
		"probe_id":                   "probe-negative",
		"node_id":                    "node-abc",
		"region_id":                  "ap-sg-1",
		"observed_at":                time.Now().UTC().Format(time.RFC3339),
		"endpoint":                   "https://node.example.net:3001",
		"availability_up":            true,
		"finalized_lag_blocks":       -1,
		"chain_lag_blocks":           2,
		"source_height":              100,
		"peer_height":                102,
		"measurement_window_seconds": 30,
	}
	negativeRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", negativeLagPayload)
	if negativeRec.Code != http.StatusBadRequest {
		t.Fatalf("negative lag status = %d, want 400", negativeRec.Code)
	}
}

func TestDailyProbeSummaryRead(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()
	now := time.Now().UTC()
	dateUTC := now.Format("2006-01-02")

	for _, payload := range []map[string]any{
		{
			"schema_version":             "1",
			"probe_id":                   "probe-summary-1",
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
			"probe_id":                   "probe-summary-2",
			"node_id":                    "node-abc",
			"region_id":                  "us-va-1",
			"observed_at":                now.Add(10 * time.Second).Format(time.RFC3339),
			"endpoint":                   "https://node.example.net:3001",
			"availability_up":            true,
			"finalized_lag_blocks":       2,
			"chain_lag_blocks":           5,
			"source_height":              101,
			"peer_height":                103,
			"measurement_window_seconds": 30,
		},
	} {
		rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", payload)
		if rec.Code != http.StatusOK {
			t.Fatalf("probe status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/probes/daily-summaries/node-abc/"+dateUTC, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("daily summary status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var got struct {
		NodeID                       string  `json:"node_id"`
		DateUTC                      string  `json:"date_utc"`
		ValidProbeCount              int     `json:"valid_probe_count"`
		RegionCount                  int     `json:"region_count"`
		AvailabilityPassed           bool    `json:"availability_passed"`
		FinalizedLagPassed           bool    `json:"finalized_lag_passed"`
		ChainLagPassed               bool    `json:"chain_lag_passed"`
		QualifiedProbeEvidencePassed bool    `json:"qualified_probe_evidence_passed"`
		AvailabilityRatio            float64 `json:"availability_ratio"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.NodeID != "node-abc" || got.DateUTC != dateUTC {
		t.Fatalf("daily summary = %#v", got)
	}
	if got.ValidProbeCount != 2 || got.RegionCount != 2 {
		t.Fatalf("daily summary counts = %#v", got)
	}
	if !got.AvailabilityPassed || !got.FinalizedLagPassed || !got.ChainLagPassed || !got.QualifiedProbeEvidencePassed {
		t.Fatalf("daily summary flags = %#v", got)
	}
	if got.AvailabilityRatio != 1 {
		t.Fatalf("availability ratio = %v, want 1", got.AvailabilityRatio)
	}
}

func TestQualifiedDecisionRecordSavedWithReasons(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()
	now := time.Now().UTC()
	dateUTC := now.Format("2006-01-02")

	node, _ := srv.store.GetNode("node-abc")
	node.LastHeartbeatTimestamp = now.Format(time.RFC3339)
	srv.store.SaveNode(node)
	srv.store.SaveCheckEvent(store.CheckEvent{
		EventID:       "check-qualified-001",
		NodeID:        "node-abc",
		OverallPassed: true,
		CheckedAt:     now.Format(time.RFC3339),
	})

	for _, payload := range []map[string]any{
		{
			"schema_version":             "1",
			"probe_id":                   "probe-qualified-1",
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
			"probe_id":                   "probe-qualified-2",
			"node_id":                    "node-abc",
			"region_id":                  "us-va-1",
			"observed_at":                now.Add(10 * time.Second).Format(time.RFC3339),
			"endpoint":                   "https://node.example.net:3001",
			"availability_up":            true,
			"finalized_lag_blocks":       2,
			"chain_lag_blocks":           5,
			"source_height":              101,
			"peer_height":                103,
			"measurement_window_seconds": 30,
		},
	} {
		rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", payload)
		if rec.Code != http.StatusOK {
			t.Fatalf("probe status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}

	decision, ok := srv.store.GetQualifiedDecisionRecord("node-abc", dateUTC)
	if !ok {
		t.Fatal("expected qualified decision record")
	}
	if decision.ProbeEvidencePassed != true {
		t.Fatalf("decision = %#v, want probe_evidence_passed", decision)
	}
	if decision.HeartbeatPassed != true {
		t.Fatalf("decision = %#v, want heartbeat_passed", decision)
	}
	if decision.HardwarePassed != true {
		t.Fatalf("decision = %#v, want hardware_passed", decision)
	}
	if decision.VotingKeyPassed {
		t.Fatalf("decision = %#v, want voting_key_passed false", decision)
	}
	if decision.Qualified {
		t.Fatalf("decision = %#v, want qualified false", decision)
	}
	if !containsReason(decision.FailureReasons, "voting_key_evidence_missing") {
		t.Fatalf("failure reasons = %#v, want voting_key_evidence_missing", decision.FailureReasons)
	}
}

func TestVotingKeyEvidenceFlowEnablesQualified(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()
	now := time.Now().UTC()
	dateUTC := now.Format("2006-01-02")

	node, _ := srv.store.GetNode("node-abc")
	node.LastHeartbeatTimestamp = now.Format(time.RFC3339)
	srv.store.SaveNode(node)
	srv.store.SaveCheckEvent(store.CheckEvent{
		EventID:       "check-voting-001",
		NodeID:        "node-abc",
		OverallPassed: true,
		CheckedAt:     now.Format(time.RFC3339),
	})

	for _, payload := range []map[string]any{
		{
			"schema_version":             "1",
			"probe_id":                   "probe-voting-1",
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
			"probe_id":                   "probe-voting-2",
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
		rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", payload)
		if rec.Code != http.StatusOK {
			t.Fatalf("probe status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}

	evidencePayload := map[string]any{
		"node_id":                    "node-abc",
		"evidence_ref":               "vk-evidence-001",
		"observed_at":                now.Add(10 * time.Second).Format(time.RFC3339),
		"current_epoch":              12,
		"voting_key_present":         true,
		"voting_key_valid_for_epoch": true,
		"source":                     "external_probe",
	}
	rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/voting-key-evidence", evidencePayload)
	if rec.Code != http.StatusOK {
		t.Fatalf("voting key evidence status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	dupRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/voting-key-evidence", evidencePayload)
	if dupRec.Code != http.StatusConflict {
		t.Fatalf("duplicate evidence status = %d, want 409", dupRec.Code)
	}

	decision, ok := srv.store.GetQualifiedDecisionRecord("node-abc", dateUTC)
	if !ok {
		t.Fatal("expected qualified decision record")
	}
	if !decision.VotingKeyPassed {
		t.Fatalf("decision = %#v, want voting key passed", decision)
	}
	if !decision.Qualified {
		t.Fatalf("decision = %#v, want qualified true", decision)
	}
	if containsReason(decision.FailureReasons, "voting_key_evidence_missing") {
		t.Fatalf("failure reasons = %#v, want voting key failure cleared", decision.FailureReasons)
	}
}

func TestVotingKeyEvidenceInvalidAddsReason(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()
	now := time.Now().UTC()
	dateUTC := now.Format("2006-01-02")

	node, _ := srv.store.GetNode("node-abc")
	node.LastHeartbeatTimestamp = now.Format(time.RFC3339)
	srv.store.SaveNode(node)
	srv.store.SaveCheckEvent(store.CheckEvent{
		EventID:       "check-voting-invalid-001",
		NodeID:        "node-abc",
		OverallPassed: true,
		CheckedAt:     now.Format(time.RFC3339),
	})
	for _, payload := range []map[string]any{
		{
			"schema_version":             "1",
			"probe_id":                   "probe-voting-invalid-1",
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
			"probe_id":                   "probe-voting-invalid-2",
			"node_id":                    "node-abc",
			"region_id":                  "us-va-1",
			"observed_at":                now.Add(5 * time.Second).Format(time.RFC3339),
			"endpoint":                   "https://node.example.net:3001",
			"availability_up":            true,
			"finalized_lag_blocks":       1,
			"chain_lag_blocks":           2,
			"source_height":              101,
			"peer_height":                103,
			"measurement_window_seconds": 30,
		},
	} {
		if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/probes/events", payload); rec.Code != http.StatusOK {
			t.Fatalf("probe status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
	}
	rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/voting-key-evidence", map[string]any{
		"node_id":                    "node-abc",
		"evidence_ref":               "vk-evidence-invalid-001",
		"observed_at":                now.Add(10 * time.Second).Format(time.RFC3339),
		"current_epoch":              12,
		"voting_key_present":         true,
		"voting_key_valid_for_epoch": false,
		"source":                     "external_probe",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("voting key evidence status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	decision, ok := srv.store.GetQualifiedDecisionRecord("node-abc", dateUTC)
	if !ok {
		t.Fatal("expected qualified decision record")
	}
	if decision.VotingKeyPassed {
		t.Fatalf("decision = %#v, want voting key false", decision)
	}
	if !containsReason(decision.FailureReasons, "voting_key_invalid") {
		t.Fatalf("failure reasons = %#v, want voting_key_invalid", decision.FailureReasons)
	}
}
