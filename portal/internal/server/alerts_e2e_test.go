package server

import (
	"bytes"
	"context"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/notifier"
)

func TestTelemetryNotificationCooldownAndDeliveryFailure(t *testing.T) {
	recorder := &notifier.Recorder{}
	srv := newTestServerWithNotifier(t, recorder)
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
	enrollPayload["enrollment_challenge_id"] = issueEnrollmentChallenge(t, failingHandler, "node-abc")
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
		"enrollment_challenge_id": issueEnrollmentChallenge(t, handler, "node-abc"),
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
		"enrollment_challenge_id": issueEnrollmentChallenge(t, handler, "node-abc"),
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
	if last.AlertCode != alertHeartbeatStale || last.Severity != notifier.SeverityWarning {
		t.Fatalf("last notification = %#v, want heartbeat_stale warning", last)
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
		"enrollment_challenge_id": issueEnrollmentChallenge(t, handler, "node-abc"),
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
	agentDir := agentPythonDir(t, repoRoot)

	tempDir := t.TempDir()
	policyPath := writeE2EPolicy(t, tempDir)
	srv := mustNewServer(t, policyPath, &notifier.Recorder{})
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("sandbox does not allow local listener: %v", err)
	}
	httpServer := &http.Server{Handler: srv.Handler()}
	go func() {
		_ = httpServer.Serve(listener)
	}()
	defer httpServer.Shutdown(context.Background())

	privateKeyPath, publicKeyPath := writeAgentKeys(t, tempDir)
	configPath := writeAgentConfig(t, tempDir, "http://"+listener.Addr().String(), privateKeyPath, publicKeyPath)

	challengeID := issueEnrollmentChallenge(t, srv.Handler(), "node-abc")
	runAgentCommand(t, agentDir, configPath, "enroll", "--challenge-id", challengeID)

	runCmd := exec.Command("python3", "-m", "ssnp_agent", "--config", configPath, "run")
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

	runAgentCommand(t, agentDir, configPath, "check", "--event-type", "registration", "--event-id", "check-e2e-001")

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
