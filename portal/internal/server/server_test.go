package server

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/verify"
)

func TestNewFailsOnBrokenPolicy(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "broken.yaml")
	if err := os.WriteFile(policyPath, []byte("policy_version:"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              policyPath,
		AllowedClockSkewSeconds: 300,
	}); err == nil {
		t.Fatal("New() error = nil, want broken policy failure")
	}
}

func TestPortalHandlerFlow(t *testing.T) {
	srv := newTestServer(t)
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
		"enrollment_challenge_id": "enroll-001",
		"agent_public_key":        pubHex,
		"agent_version":           "1.0.0",
	}
	signPayload(t, priv, enrollPayload)
	enrollRec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/enroll", enrollPayload)
	if enrollRec.Code != http.StatusOK {
		t.Fatalf("enroll status = %d, want 200, body=%s", enrollRec.Code, enrollRec.Body.String())
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
}

func TestChecksRejectPolicyMismatchAndTelemetryRejectsInvalidSignature(t *testing.T) {
	srv := newTestServer(t)
	handler := srv.Handler()

	pub, priv := newKeyPair(t)
	pubHex := hex.EncodeToString(pub)
	fingerprint := mustFingerprint(t, pubHex)

	enrollPayload := map[string]any{
		"node_id":                 "node-abc",
		"enrollment_challenge_id": "enroll-001",
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

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return mustNewServer(t, testPolicyPath())
}

func mustNewServer(t *testing.T, policyPath string) *Server {
	t.Helper()
	srv, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              policyPath,
		AllowedClockSkewSeconds: 300,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return srv
}

func testPolicyPath() string {
	return filepath.Clean("../../../docs/policies/program_agent_policy.v2026-04.yaml")
}

func newKeyPair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	return pub, priv
}

func signPayload(t *testing.T, priv ed25519.PrivateKey, payload map[string]any) {
	t.Helper()
	copyMap := map[string]any{}
	for k, v := range payload {
		if k == "signature" {
			continue
		}
		copyMap[k] = v
	}
	data, err := json.Marshal(copyMap)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	payload["signature"] = hex.EncodeToString(ed25519.Sign(priv, data))
}

func signHeartbeatPayload(t *testing.T, priv ed25519.PrivateKey, payload map[string]any) {
	t.Helper()
	ordered := struct {
		NodeID                string   `json:"node_id"`
		AgentKeyFingerprint   string   `json:"agent_key_fingerprint"`
		HeartbeatTimestamp    string   `json:"heartbeat_timestamp"`
		SequenceNumber        int      `json:"sequence_number"`
		AgentVersion          string   `json:"agent_version"`
		EnrollmentGeneration  int      `json:"enrollment_generation"`
		LocalObservationFlags []string `json:"local_observation_flags"`
		Signature             string   `json:"signature,omitempty"`
	}{
		NodeID:                payload["node_id"].(string),
		AgentKeyFingerprint:   payload["agent_key_fingerprint"].(string),
		HeartbeatTimestamp:    payload["heartbeat_timestamp"].(string),
		SequenceNumber:        payload["sequence_number"].(int),
		AgentVersion:          payload["agent_version"].(string),
		EnrollmentGeneration:  payload["enrollment_generation"].(int),
		LocalObservationFlags: payload["local_observation_flags"].([]string),
	}
	data, err := json.Marshal(ordered)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	payload["signature"] = hex.EncodeToString(ed25519.Sign(priv, data))
}

func doJSONRequest(t *testing.T, handler http.Handler, method, path string, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func mustFingerprint(t *testing.T, pubHex string) string {
	t.Helper()
	fingerprint, err := verify.FingerprintFromHexPublicKey(pubHex)
	if err != nil {
		t.Fatalf("FingerprintFromHexPublicKey() error = %v", err)
	}
	return fingerprint
}
