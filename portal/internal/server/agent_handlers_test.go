package server

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/notifier"
)

func TestNewFailsOnBrokenPolicy(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "broken.yaml")
	nodesPath := writeNodesConfig(t, dir)
	if err := os.WriteFile(policyPath, []byte("policy_version:"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              policyPath,
		NodesConfigPath:         nodesPath,
		StatePath:               filepath.Join(dir, "portal-state.json"),
		AllowedClockSkewSeconds: 300,
		Notifier:                &notifier.Recorder{},
	}); err == nil {
		t.Fatal("New() error = nil, want broken policy failure")
	}
}

func TestNewFailsOnBrokenNodesConfigAndSnapshot(t *testing.T) {
	dir := t.TempDir()
	policyPath := testPolicyPath()
	nodesPath := filepath.Join(dir, "nodes.yaml")
	statePath := filepath.Join(dir, "portal-state.json")
	if err := os.WriteFile(nodesPath, []byte("nodes:"), 0o600); err != nil {
		t.Fatalf("WriteFile(nodes) error = %v", err)
	}
	if _, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              policyPath,
		NodesConfigPath:         nodesPath,
		StatePath:               statePath,
		AllowedClockSkewSeconds: 300,
		Notifier:                &notifier.Recorder{},
	}); err == nil {
		t.Fatal("New() error = nil, want broken nodes config failure")
	}

	nodesPath = writeNodesConfig(t, dir)
	if err := os.WriteFile(statePath, []byte("{"), 0o600); err != nil {
		t.Fatalf("WriteFile(state) error = %v", err)
	}
	if _, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              policyPath,
		NodesConfigPath:         nodesPath,
		StatePath:               statePath,
		AllowedClockSkewSeconds: 300,
		Notifier:                &notifier.Recorder{},
	}); err == nil {
		t.Fatal("New() error = nil, want broken snapshot failure")
	}
}

func TestNewAllowsNonSMTPNotifierModesWithoutSMTPConfig(t *testing.T) {
	dir := t.TempDir()
	nodesPath := writeNodesConfig(t, dir)
	for _, n := range []notifier.Notifier{notifier.StdoutNotifier{}, notifier.NoopNotifier{}} {
		if _, err := New(Config{
			ListenAddr:              "127.0.0.1:8080",
			PolicyPath:              testPolicyPath(),
			NodesConfigPath:         nodesPath,
			StatePath:               filepath.Join(dir, "portal-state-"+strings.ReplaceAll(strings.ToLower(reflect.TypeOf(n).Name()), "notifier", "")+".json"),
			AllowedClockSkewSeconds: 300,
			Notifier:                n,
		}); err != nil {
			t.Fatalf("New() error with %T = %v", n, err)
		}
	}
}

func TestPortalHandlerFlow(t *testing.T) {
	recorder := &notifier.Recorder{}
	srv := newTestServerWithNotifier(t, recorder)
	handler := srv.Handler()

	pub, priv := newKeyPair(t)
	pubHex := hex.EncodeToString(pub)
	fingerprint := mustFingerprint(t, pubHex)

	policyReq := httptest.NewRequest(http.MethodGet, "/api/v1/agent/policy?node_id=node-abc&agent_key_fingerprint=pre-enroll", nil)
	policyRec := httptest.NewRecorder()
	handler.ServeHTTP(policyRec, policyReq)
	if policyRec.Code != http.StatusOK {
		t.Fatalf("policy status = %d, want 200", policyRec.Code)
	}

	enrollPayload := map[string]any{
		"node_id":                 "node-abc",
		"enrollment_challenge_id": issueEnrollmentChallenge(t, handler, "node-abc"),
		"agent_public_key":        pubHex,
		"agent_version":           "1.0.0",
	}
	signPayload(t, priv, enrollPayload)
	enrollRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/enroll", enrollPayload)
	if enrollRec.Code != http.StatusOK {
		t.Fatalf("enroll status = %d, want 200, body=%s", enrollRec.Code, enrollRec.Body.String())
	}
	node, _ := srv.store.GetNode("node-abc")
	if node.ValidatedRegistrationAt == "" {
		t.Fatal("expected validated registration time after enroll")
	}

	heartbeatPayload := map[string]any{
		"node_id":                 "node-abc",
		"agent_key_fingerprint":   fingerprint,
		"heartbeat_timestamp":     time.Now().UTC().Format(time.RFC3339),
		"sequence_number":         1,
		"agent_version":           "1.0.0",
		"enrollment_generation":   1,
		"local_observation_flags": []string{},
	}
	signHeartbeatPayload(t, priv, heartbeatPayload)
	heartbeatRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/heartbeat", heartbeatPayload)
	if heartbeatRec.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d, want 200, body=%s", heartbeatRec.Code, heartbeatRec.Body.String())
	}

	replayRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/heartbeat", heartbeatPayload)
	if replayRec.Code != http.StatusConflict {
		t.Fatalf("replay status = %d, want 409", replayRec.Code)
	}

	checkPayload := map[string]any{
		"schema_version":            "1",
		"node_id":                   "node-abc",
		"agent_key_fingerprint":     fingerprint,
		"event_type":                "registration",
		"event_id":                  "check-001",
		"policy_version":            "2026-04",
		"cpu_profile_id":            "cpu-check-v1",
		"disk_profile_id":           "disk-check-v1",
		"checked_at":                time.Now().UTC().Format(time.RFC3339),
		"cpu_check_passed":          true,
		"disk_check_passed":         true,
		"ram_check_passed":          true,
		"storage_size_check_passed": true,
		"ssd_check_passed":          true,
		"cpu_load_test_passed":      true,
		"overall_passed":            true,
		"agent_version":             "1.0.0",
	}
	signPayload(t, priv, checkPayload)
	checkRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/checks", checkPayload)
	if checkRec.Code != http.StatusOK {
		t.Fatalf("checks status = %d, want 200, body=%s", checkRec.Code, checkRec.Body.String())
	}

	duplicateCheckRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/checks", checkPayload)
	if duplicateCheckRec.Code != http.StatusConflict {
		t.Fatalf("duplicate checks status = %d, want 409", duplicateCheckRec.Code)
	}

	telemetryPayload := map[string]any{
		"schema_version":        "1",
		"node_id":               "node-abc",
		"agent_key_fingerprint": fingerprint,
		"telemetry_timestamp":   time.Now().UTC().Format(time.RFC3339),
		"policy_version":        "2026-04",
		"warning_flags":         []string{"portal_unreachable"},
	}
	signPayload(t, priv, telemetryPayload)
	telemetryRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/telemetry", telemetryPayload)
	if telemetryRec.Code != http.StatusOK {
		t.Fatalf("telemetry status = %d, want 200, body=%s", telemetryRec.Code, telemetryRec.Body.String())
	}
	if recorder.Count() != 1 {
		t.Fatalf("notification count = %d, want 1", recorder.Count())
	}

	telemetryListReq := httptest.NewRequest(http.MethodGet, "/api/v1/agent/telemetry?node_id=node-abc&warning_code=portal_unreachable", nil)
	telemetryListRec := httptest.NewRecorder()
	handler.ServeHTTP(telemetryListRec, telemetryListReq)
	if telemetryListRec.Code != http.StatusOK {
		t.Fatalf("telemetry list status = %d, want 200", telemetryListRec.Code)
	}
	assertTelemetryItemsLength(t, telemetryListRec.Body.Bytes(), 1)

	latestReq := httptest.NewRequest(http.MethodGet, "/api/v1/agent/telemetry?view=latest", nil)
	latestRec := httptest.NewRecorder()
	handler.ServeHTTP(latestRec, latestReq)
	if latestRec.Code != http.StatusOK {
		t.Fatalf("latest telemetry status = %d, want 200", latestRec.Code)
	}
	assertTelemetryItemsLength(t, latestRec.Body.Bytes(), 1)
}

func TestChecksRejectPolicyMismatchAndTelemetryRejectsInvalidSignature(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	pub, priv := newKeyPair(t)
	pubHex := hex.EncodeToString(pub)
	fingerprint := mustFingerprint(t, pubHex)

	enrollPayload := map[string]any{
		"node_id":                 "node-abc",
		"enrollment_challenge_id": issueEnrollmentChallenge(t, handler, "node-abc"),
		"agent_public_key":        pubHex,
		"agent_version":           "1.0.0",
	}
	signPayload(t, priv, enrollPayload)
	enrollRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/enroll", enrollPayload)
	if enrollRec.Code != http.StatusOK {
		t.Fatalf("enroll status = %d, want 200", enrollRec.Code)
	}

	checkPayload := map[string]any{
		"schema_version":            "1",
		"node_id":                   "node-abc",
		"agent_key_fingerprint":     fingerprint,
		"event_type":                "registration",
		"event_id":                  "check-002",
		"policy_version":            "wrong-policy",
		"cpu_profile_id":            "cpu-check-v1",
		"disk_profile_id":           "disk-check-v1",
		"checked_at":                time.Now().UTC().Format(time.RFC3339),
		"cpu_check_passed":          true,
		"disk_check_passed":         true,
		"ram_check_passed":          true,
		"storage_size_check_passed": true,
		"ssd_check_passed":          true,
		"cpu_load_test_passed":      true,
		"overall_passed":            true,
		"agent_version":             "1.0.0",
	}
	signPayload(t, priv, checkPayload)
	checkRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/checks", checkPayload)
	if checkRec.Code != http.StatusConflict {
		t.Fatalf("checks status = %d, want 409", checkRec.Code)
	}

	telemetryPayload := map[string]any{
		"schema_version":        "1",
		"node_id":               "node-abc",
		"agent_key_fingerprint": fingerprint,
		"telemetry_timestamp":   time.Now().UTC().Format(time.RFC3339),
		"policy_version":        "2026-04",
		"warning_flags":         []string{"portal_unreachable"},
		"signature":             "deadbeef",
	}
	telemetryRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/telemetry", telemetryPayload)
	if telemetryRec.Code != http.StatusUnauthorized {
		t.Fatalf("telemetry status = %d, want 401", telemetryRec.Code)
	}
}
