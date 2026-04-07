package server

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/notifier"
	"github.com/u2yasan/ssnp_sip/portal/internal/store"
	"github.com/u2yasan/ssnp_sip/portal/internal/verify"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return mustNewServer(t, testPolicyPath(), &notifier.Recorder{})
}

func newTestServerWithNotifier(t *testing.T, n notifier.Notifier) *Server {
	t.Helper()
	return mustNewServer(t, testPolicyPath(), n)
}

func mustNewServer(t *testing.T, policyPath string, n notifier.Notifier) *Server {
	t.Helper()
	dir := t.TempDir()
	nodesPath := writeNodesConfig(t, dir)
	srv, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              policyPath,
		NodesConfigPath:         nodesPath,
		StatePath:               filepath.Join(dir, "portal-state.json"),
		AllowedClockSkewSeconds: 300,
		NotificationEmailTo:     "ops@example.invalid",
		Notifier:                n,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	now := time.Now().UTC()
	for _, nodeID := range []string{"node-abc", "node-def"} {
		node, ok := srv.store.GetNode(nodeID)
		if !ok {
			continue
		}
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
	}
	return srv
}

func issueEnrollmentChallenge(t *testing.T, handler http.Handler, nodeID string) string {
	t.Helper()
	rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/enrollment-challenges", map[string]any{
		"node_id": nodeID,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("challenge status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		ChallengeID string `json:"challenge_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload.ChallengeID == "" {
		t.Fatal("expected challenge_id")
	}
	return payload.ChallengeID
}

func writeNodesConfigNoEmail(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "nodes-no-email.yaml")
	content := `nodes:
  - node_id: "node-abc"
    display_name: "Node ABC"
    enabled: true
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(nodes) error = %v", err)
	}
	return path
}

func testPolicyPath() string {
	return filepath.Clean("../../../docs/policies/program_agent_policy.v2026-04.yaml")
}

func smokePolicyPath() string {
	return filepath.Clean("../../../testdata/smoke/policy.yaml")
}

func smokeStateSeedPath() string {
	return filepath.Clean("../../../testdata/smoke/portal-state.json")
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

func assertTelemetryItemsLength(t *testing.T, body []byte, want int) {
	t.Helper()
	var payload struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(payload.Items) != want {
		t.Fatalf("len(items) = %d, want %d", len(payload.Items), want)
	}
}

func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, wantStatus, rec.Body.String())
	}
	var payload struct {
		Status    string `json:"status"`
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if payload.Status != "error" {
		t.Fatalf("status field = %q, want error", payload.Status)
	}
	if payload.ErrorCode != wantCode {
		t.Fatalf("error_code = %q, want %q", payload.ErrorCode, wantCode)
	}
	if payload.Message == "" {
		t.Fatal("message = empty, want non-empty")
	}
}

func containsReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}

func buildAgentBinary(t *testing.T, agentDir string) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "program-agent")
	if runtime.GOOS == "windows" {
		binaryPath += ".exe"
	}
	buildCache, err := filepath.Abs(filepath.Join(agentDir, ".cache", "go-build"))
	if err != nil {
		t.Fatalf("filepath.Abs(go-build) error = %v", err)
	}
	modCache, err := filepath.Abs(filepath.Join(agentDir, ".cache", "go-mod"))
	if err != nil {
		t.Fatalf("filepath.Abs(go-mod) error = %v", err)
	}
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/program-agent")
	cmd.Dir = agentDir
	cmd.Env = append(
		os.Environ(),
		"GOCACHE="+buildCache,
		"GOMODCACHE="+modCache,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build agent error = %v\noutput:\n%s", err, string(output))
	}
	return binaryPath
}

func runAgentCommand(t *testing.T, agentBinary, agentDir, configPath string, args ...string) string {
	t.Helper()
	cmd := exec.Command(agentBinary, append([]string{"--config", configPath}, args...)...)
	cmd.Dir = agentDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s error = %v\noutput:\n%s", strings.Join(args, " "), err, string(output))
	}
	return string(output)
}

func writeE2EPolicy(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "policy.yaml")
	content := `policy_version: "2026-04"
heartbeat_interval_seconds: 1
cpu_profile:
  id: "cpu-check-v1"
  duration_seconds: 3
  warmup_seconds: 1
  measured_seconds: 1
  cooldown_seconds: 1
  worker_cap: 8
  workload_mix:
    hashing: 0.5
    integer: 0.3
    matrix: 0.2
  acceptance_floor:
    type: "normalized_score"
    minimum: 0
disk_profile:
  id: "disk-check-v1"
  duration_seconds: 3
  warmup_seconds: 1
  measured_seconds: 1
  cooldown_seconds: 1
  block_size_bytes: 4096
  queue_depth: 32
  concurrency: 1
  read_ratio: 0.7
  write_ratio: 0.3
  acceptance_floor:
    type: "measured_iops"
    minimum: 0
hardware_thresholds:
  cpu_cores_min: 1
  ram_gb_min: 1
  storage_gb_min: 1
  ssd_required: false
probe_thresholds:
  finalized_lag_max_blocks: 2
  chain_lag_max_blocks: 5
reference_environment:
  id: "ref-env-2026-04"
  os_image_id: "ubuntu-24.04-lts"
  agent_version: "1.0.0"
  cpu_profile_id: "cpu-check-v1"
  disk_profile_id: "disk-check-v1"
  baseline_source_date: "2026-04-06"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(policy) error = %v", err)
	}
	return path
}

func writeNodesConfig(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "nodes.yaml")
	content := `nodes:
  - node_id: "node-abc"
    display_name: "Node ABC"
    operator_email: "ops@example.invalid"
    enabled: true
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(nodes config) error = %v", err)
	}
	return path
}

func writeAgentKeys(t *testing.T, dir string) (string, string) {
	t.Helper()
	pub, priv := newKeyPair(t)
	privDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error = %v", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatalf("MarshalPKIXPublicKey() error = %v", err)
	}
	privateKeyPath := filepath.Join(dir, "agent_private_key.pem")
	publicKeyPath := filepath.Join(dir, "agent_public_key.pem")
	if err := os.WriteFile(privateKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}), 0o600); err != nil {
		t.Fatalf("WriteFile(private key) error = %v", err)
	}
	if err := os.WriteFile(publicKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), 0o600); err != nil {
		t.Fatalf("WriteFile(public key) error = %v", err)
	}
	return privateKeyPath, publicKeyPath
}

func writeAgentConfig(t *testing.T, dir, portalURL, privateKeyPath, publicKeyPath string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	content := "node_id: \"node-abc\"\n" +
		"portal_base_url: \"" + portalURL + "\"\n" +
		"agent_key_path: \"" + privateKeyPath + "\"\n" +
		"agent_public_key_path: \"" + publicKeyPath + "\"\n" +
		"monitored_endpoint: \"https://node-01.example.net:3001\"\n" +
		"state_path: \"" + filepath.Join(dir, "state.json") + "\"\n" +
		"temp_dir: \"" + dir + "\"\n" +
		"request_timeout_seconds: 5\n" +
		"heartbeat_jitter_seconds_max: 0\n" +
		"agent_version: \"1.0.0\"\n" +
		"enrollment_generation: 1\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	return path
}

func waitForHeartbeat(t *testing.T, srv *Server, timeout time.Duration) {
	t.Helper()
	waitForHeartbeatSequence(t, srv, timeout, 1)
}

func waitForHeartbeatSequence(t *testing.T, srv *Server, timeout time.Duration, wantSequence int) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		node, ok := srv.store.GetNode("node-abc")
		if ok && node.LastHeartbeatSequence >= wantSequence {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	node, _ := srv.store.GetNode("node-abc")
	t.Fatalf("heartbeat sequence = %d, want >= %d", node.LastHeartbeatSequence, wantSequence)
}

func copyFile(t *testing.T, srcPath, dstPath string) {
	t.Helper()
	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", srcPath, err)
	}
	if err := os.WriteFile(dstPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", dstPath, err)
	}
}
