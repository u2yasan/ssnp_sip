package server

import (
	"bytes"
	"context"
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
	"syscall"
	"testing"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/notifier"
	"github.com/u2yasan/ssnp_sip/portal/internal/verify"
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

func TestTelemetryNotificationCooldownAndDeliveryFailure(t *testing.T) {
	recorder := &notifier.Recorder{}
	srv := newTestServerWithNotifier(t, recorder)
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
	if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/enroll", enrollPayload); rec.Code != http.StatusOK {
		t.Fatalf("enroll status = %d, want 200", rec.Code)
	}

	payload := map[string]any{
		"schema_version":        "1",
		"node_id":               "node-abc",
		"agent_key_fingerprint": fingerprint,
		"telemetry_timestamp":   time.Now().UTC().Format(time.RFC3339),
		"policy_version":        "2026-04",
		"warning_flags":         []string{"portal_unreachable"},
	}
	signPayload(t, priv, payload)
	if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/telemetry", payload); rec.Code != http.StatusOK {
		t.Fatalf("telemetry status = %d, want 200", rec.Code)
	}
	signPayload(t, priv, payload)
	if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/telemetry", payload); rec.Code != http.StatusOK {
		t.Fatalf("second telemetry status = %d, want 200", rec.Code)
	}
	if recorder.Count() != 1 {
		t.Fatalf("notification count = %d, want 1 due to warning cooldown", recorder.Count())
	}

	failing := notifier.FailingRecorder("smtp down")
	failingSrv := newTestServerWithNotifier(t, failing)
	failingHandler := failingSrv.Handler()
	signPayload(t, priv, enrollPayload)
	if rec := doJSONRequest(t, failingHandler, http.MethodPost, "/api/v1/agent/enroll", enrollPayload); rec.Code != http.StatusOK {
		t.Fatalf("failing enroll status = %d, want 200", rec.Code)
	}
	signPayload(t, priv, payload)
	if rec := doJSONRequest(t, failingHandler, http.MethodPost, "/api/v1/agent/telemetry", payload); rec.Code != http.StatusOK {
		t.Fatalf("failing telemetry status = %d, want 200", rec.Code)
	}
	events := failingSrv.store.ListOperationalEvents("node-abc", operationalEventDeliveryFailure)
	if len(events) != 1 {
		t.Fatalf("operational events = %d, want 1", len(events))
	}
	deliveries := failingSrv.store.ListNotificationDeliveries("node-abc", "portal_unreachable")
	if len(deliveries) != 1 || deliveries[0].Status != "failed" {
		t.Fatalf("deliveries = %#v, want single failed delivery", deliveries)
	}
}

func TestTelemetryUsesFallbackRecipientWhenNodeEmailMissing(t *testing.T) {
	recorder := &notifier.Recorder{}
	dir := t.TempDir()
	nodesPath := writeNodesConfigNoEmail(t, dir)
	srv, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              testPolicyPath(),
		NodesConfigPath:         nodesPath,
		StatePath:               filepath.Join(dir, "portal-state.json"),
		AllowedClockSkewSeconds: 300,
		NotificationEmailTo:     "fallback@example.invalid",
		Notifier:                recorder,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
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
	if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/enroll", enrollPayload); rec.Code != http.StatusOK {
		t.Fatalf("enroll status = %d, want 200", rec.Code)
	}

	payload := map[string]any{
		"schema_version":        "1",
		"node_id":               "node-abc",
		"agent_key_fingerprint": fingerprint,
		"telemetry_timestamp":   time.Now().UTC().Format(time.RFC3339),
		"policy_version":        "2026-04",
		"warning_flags":         []string{"portal_unreachable"},
	}
	signPayload(t, priv, payload)
	if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/telemetry", payload); rec.Code != http.StatusOK {
		t.Fatalf("telemetry status = %d, want 200", rec.Code)
	}
	last, ok := recorder.Last()
	if !ok {
		t.Fatal("expected notification")
	}
	if last.Recipient != "fallback@example.invalid" {
		t.Fatalf("recipient = %q, want fallback@example.invalid", last.Recipient)
	}
}

func TestTelemetryRecordsFailureWhenNoRecipientExists(t *testing.T) {
	recorder := &notifier.Recorder{}
	dir := t.TempDir()
	nodesPath := writeNodesConfigNoEmail(t, dir)
	srv, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              testPolicyPath(),
		NodesConfigPath:         nodesPath,
		StatePath:               filepath.Join(dir, "portal-state.json"),
		AllowedClockSkewSeconds: 300,
		Notifier:                recorder,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
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
	if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/enroll", enrollPayload); rec.Code != http.StatusOK {
		t.Fatalf("enroll status = %d, want 200", rec.Code)
	}

	payload := map[string]any{
		"schema_version":        "1",
		"node_id":               "node-abc",
		"agent_key_fingerprint": fingerprint,
		"telemetry_timestamp":   time.Now().UTC().Format(time.RFC3339),
		"policy_version":        "2026-04",
		"warning_flags":         []string{"portal_unreachable"},
	}
	signPayload(t, priv, payload)
	if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/telemetry", payload); rec.Code != http.StatusOK {
		t.Fatalf("telemetry status = %d, want 200", rec.Code)
	}
	if recorder.Count() != 0 {
		t.Fatalf("notification count = %d, want 0", recorder.Count())
	}
	events := srv.store.ListOperationalEvents("node-abc", operationalEventDeliveryFailure)
	if len(events) != 1 {
		t.Fatalf("operational events = %d, want 1", len(events))
	}
	deliveries := srv.store.ListNotificationDeliveries("node-abc", "portal_unreachable")
	if len(deliveries) != 1 || deliveries[0].Status != "failed" {
		t.Fatalf("deliveries = %#v, want single failed delivery", deliveries)
	}
}

func TestHeartbeatAlertScannerSendsCriticalNotifications(t *testing.T) {
	recorder := &notifier.Recorder{}
	srv := mustNewServer(t, testPolicyPath(), recorder)
	node, ok := srv.store.GetNode("node-abc")
	if !ok {
		t.Fatal("node-abc missing")
	}
	node.AgentPublicKey = strings.Repeat("ab", 32)
	node.ActiveAgentKeyFingerprint = "fp"
	now := time.Now().UTC()
	node.LastHeartbeatTimestamp = now.Add(-16 * time.Minute).Format(time.RFC3339)
	srv.store.SaveNode(node)

	srv.evaluateHeartbeatAlerts(context.Background(), now)
	if recorder.Count() != 1 {
		t.Fatalf("notification count = %d, want 1 stale alert", recorder.Count())
	}
	last, _ := recorder.Last()
	if last.AlertCode != alertHeartbeatStale || last.Severity != notifier.SeverityCritical {
		t.Fatalf("last notification = %#v, want heartbeat_stale critical", last)
	}

	srv.evaluateHeartbeatAlerts(context.Background(), now.Add(10*time.Minute))
	if recorder.Count() != 1 {
		t.Fatalf("notification count = %d, want stale cooldown suppression", recorder.Count())
	}

	node.LastHeartbeatTimestamp = now.Add(-31 * time.Minute).Format(time.RFC3339)
	srv.store.SaveNode(node)
	srv.evaluateHeartbeatAlerts(context.Background(), now.Add(31*time.Minute))
	if recorder.Count() != 2 {
		t.Fatalf("notification count = %d, want failed alert added", recorder.Count())
	}
	last, _ = recorder.Last()
	if last.AlertCode != alertHeartbeatFailed {
		t.Fatalf("last notification = %#v, want heartbeat_failed", last)
	}
}

func TestPersistenceAcrossRestartKeepsCooldownAndState(t *testing.T) {
	dir := t.TempDir()
	nodesPath := writeNodesConfig(t, dir)
	statePath := filepath.Join(dir, "portal-state.json")
	recorder := &notifier.Recorder{}

	srv, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              testPolicyPath(),
		NodesConfigPath:         nodesPath,
		StatePath:               statePath,
		AllowedClockSkewSeconds: 300,
		NotificationEmailTo:     "ops@example.invalid",
		Notifier:                recorder,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
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
	if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/enroll", enrollPayload); rec.Code != http.StatusOK {
		t.Fatalf("enroll status = %d, want 200", rec.Code)
	}

	payload := map[string]any{
		"schema_version":        "1",
		"node_id":               "node-abc",
		"agent_key_fingerprint": fingerprint,
		"telemetry_timestamp":   time.Now().UTC().Format(time.RFC3339),
		"policy_version":        "2026-04",
		"warning_flags":         []string{"portal_unreachable"},
	}
	signPayload(t, priv, payload)
	if rec := doJSONRequest(t, handler, http.MethodPost, "/api/v1/agent/telemetry", payload); rec.Code != http.StatusOK {
		t.Fatalf("telemetry status = %d, want 200", rec.Code)
	}
	if recorder.Count() != 1 {
		t.Fatalf("notification count = %d, want 1", recorder.Count())
	}

	restartedRecorder := &notifier.Recorder{}
	restarted, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              testPolicyPath(),
		NodesConfigPath:         nodesPath,
		StatePath:               statePath,
		AllowedClockSkewSeconds: 300,
		NotificationEmailTo:     "ops@example.invalid",
		Notifier:                restartedRecorder,
	})
	if err != nil {
		t.Fatalf("New(restart) error = %v", err)
	}
	restartedHandler := restarted.Handler()
	signPayload(t, priv, payload)
	if rec := doJSONRequest(t, restartedHandler, http.MethodPost, "/api/v1/agent/telemetry", payload); rec.Code != http.StatusOK {
		t.Fatalf("restarted telemetry status = %d, want 200", rec.Code)
	}
	if restartedRecorder.Count() != 0 {
		t.Fatalf("notification count after restart = %d, want cooldown suppression", restartedRecorder.Count())
	}
	node, ok := restarted.store.GetNode("node-abc")
	if !ok || node.ActiveAgentKeyFingerprint != fingerprint {
		t.Fatalf("restarted node = %#v, want enrolled fingerprint", node)
	}
	if len(restarted.store.ListTelemetry("node-abc", "portal_unreachable")) != 2 {
		t.Fatal("expected telemetry history to survive restart")
	}
}

func TestAgentAndPortalEndToEndOverHTTP(t *testing.T) {
	repoRoot, err := filepath.Abs("../../../")
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	agentDir := filepath.Join(repoRoot, "agent")
	agentBinary := buildAgentBinary(t, agentDir)

	tempDir := t.TempDir()
	policyPath := writeE2EPolicy(t, tempDir)
	srv := mustNewServer(t, policyPath, &notifier.Recorder{})
	httpServer := httptest.NewServer(srv.Handler())
	defer httpServer.Close()

	privateKeyPath, publicKeyPath := writeAgentKeys(t, tempDir)
	configPath := writeAgentConfig(t, tempDir, httpServer.URL, privateKeyPath, publicKeyPath)

	runAgentCommand(t, agentBinary, agentDir, configPath, "enroll", "--challenge-id", "enroll-001")

	runCmd := exec.Command(agentBinary, "--config", configPath, "run")
	runCmd.Dir = agentDir
	runCmd.Env = append(os.Environ(), "HOME="+tempDir)
	runOutput := &bytes.Buffer{}
	runCmd.Stdout = runOutput
	runCmd.Stderr = runOutput
	if err := runCmd.Start(); err != nil {
		t.Fatalf("run Start() error = %v", err)
	}
	defer func() {
		if runCmd.ProcessState == nil || !runCmd.ProcessState.Exited() {
			_ = runCmd.Process.Kill()
			_, _ = runCmd.Process.Wait()
		}
	}()

	waitForHeartbeat(t, srv, 5*time.Second)

	if err := runCmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("run Signal() error = %v", err)
	}
	if err := runCmd.Wait(); err != nil {
		t.Fatalf("run Wait() error = %v\noutput:\n%s", err, runOutput.String())
	}

	runAgentCommand(t, agentBinary, agentDir, configPath, "check", "--event-type", "registration", "--event-id", "check-e2e-001")

	node, ok := srv.store.GetNode("node-abc")
	if !ok {
		t.Fatal("node-abc missing from store")
	}
	if node.LastHeartbeatSequence < 1 {
		t.Fatalf("LastHeartbeatSequence = %d, want >= 1", node.LastHeartbeatSequence)
	}
	if _, exists := srv.store.GetCheckEvent("check-e2e-001"); !exists {
		t.Fatal("expected check-e2e-001 in store")
	}
}

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
	return srv
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
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		node, ok := srv.store.GetNode("node-abc")
		if ok && node.LastHeartbeatSequence >= 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	node, _ := srv.store.GetNode("node-abc")
	t.Fatalf("heartbeat not observed, last sequence = %d", node.LastHeartbeatSequence)
}
