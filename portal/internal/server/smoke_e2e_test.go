package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/notifier"
)

func TestSmokeE2E(t *testing.T) {
	repoRoot, err := filepath.Abs("../../../")
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	agentDir := filepath.Join(repoRoot, "agent")
	agentBinary := buildAgentBinary(t, agentDir)

	tempDir := t.TempDir()
	nodesPath := writeNodesConfig(t, tempDir)
	statePath := filepath.Join(tempDir, "portal-state.json")
	copyFile(t, smokeStateSeedPath(), statePath)
	srv, err := New(Config{
		ListenAddr:              "127.0.0.1:8080",
		PolicyPath:              smokePolicyPath(),
		NodesConfigPath:         nodesPath,
		StatePath:               statePath,
		AllowedClockSkewSeconds: 300,
		NotificationEmailTo:     "ops@example.invalid",
		Notifier:                notifier.StdoutNotifier{Writer: &bytes.Buffer{}},
		NominalDailyPool:        1000,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

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

	challengeID := issueEnrollmentChallengeHTTP(t, "http://"+listener.Addr().String(), "node-abc")
	runAgentCommand(t, agentBinary, agentDir, configPath, "enroll", "--challenge-id", challengeID)

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

	waitForHeartbeatSequence(t, srv, 6*time.Second, 2)

	if err := runCmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("run Signal() error = %v", err)
	}
	if err := runCmd.Wait(); err != nil {
		t.Fatalf("run Wait() error = %v\noutput:\n%s", err, runOutput.String())
	}

	runAgentCommand(t, agentBinary, agentDir, configPath, "check", "--event-type", "registration", "--event-id", "smoke-check-001")
	runAgentCommand(t, agentBinary, agentDir, configPath, "telemetry", "--warning-flag", "portal_unreachable")

	now := time.Now().UTC()
	postJSONOK(t, "http://"+listener.Addr().String()+"/api/v1/probes/events", map[string]any{
		"schema_version":             "1",
		"probe_id":                   "smoke-probe-1",
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
	})
	postJSONOK(t, "http://"+listener.Addr().String()+"/api/v1/probes/events", map[string]any{
		"schema_version":             "1",
		"probe_id":                   "smoke-probe-2",
		"node_id":                    "node-abc",
		"region_id":                  "us-va-1",
		"observed_at":                now.Add(time.Second).Format(time.RFC3339),
		"endpoint":                   "https://node.example.net:3001",
		"availability_up":            true,
		"finalized_lag_blocks":       2,
		"chain_lag_blocks":           5,
		"source_height":              101,
		"peer_height":                103,
		"measurement_window_seconds": 30,
	})
	postJSONOK(t, "http://"+listener.Addr().String()+"/api/v1/voting-key-evidence", map[string]any{
		"node_id":                    "node-abc",
		"evidence_ref":               "smoke-vk-001",
		"observed_at":                now.Add(2 * time.Second).Format(time.RFC3339),
		"current_epoch":              12,
		"voting_key_present":         true,
		"voting_key_valid_for_epoch": true,
		"source":                     "external_probe",
	})

	dateUTC := now.Format("2006-01-02")
	var publicStatus struct {
		Items []struct {
			NodeID string `json:"node_id"`
		} `json:"items"`
	}
	getJSONOK(t, "http://"+listener.Addr().String()+"/api/v1/public-node-status/"+dateUTC, &publicStatus)
	if len(publicStatus.Items) != 1 || publicStatus.Items[0].NodeID != "node-abc" {
		t.Fatalf("public status = %#v, want node-abc", publicStatus)
	}

	var rankings struct {
		Items []struct {
			NodeID string `json:"node_id"`
		} `json:"items"`
	}
	getJSONOK(t, "http://"+listener.Addr().String()+"/api/v1/rankings/"+dateUTC, &rankings)
	if len(rankings.Items) != 1 || rankings.Items[0].NodeID != "node-abc" {
		t.Fatalf("rankings = %#v, want node-abc", rankings)
	}

	var operatorStatus struct {
		NodeID         string `json:"node_id"`
		Qualified      bool   `json:"qualified"`
		RewardEligible bool   `json:"reward_eligible"`
	}
	getJSONOK(t, "http://"+listener.Addr().String()+"/api/v1/operator-node-status/node-abc/"+dateUTC, &operatorStatus)
	if operatorStatus.NodeID != "node-abc" || !operatorStatus.Qualified || !operatorStatus.RewardEligible {
		t.Fatalf("operator status = %#v, want qualified eligible node-abc", operatorStatus)
	}
}

func issueEnrollmentChallengeHTTP(t *testing.T, baseURL, nodeID string) string {
	t.Helper()
	var payload struct {
		ChallengeID string `json:"challenge_id"`
	}
	postJSONDecode(t, baseURL+"/api/v1/agent/enrollment-challenges", map[string]any{"node_id": nodeID}, &payload)
	if payload.ChallengeID == "" {
		t.Fatal("expected challenge_id")
	}
	return payload.ChallengeID
}

func postJSONOK(t *testing.T, url string, payload any) {
	t.Helper()
	var response map[string]any
	postJSONDecode(t, url, payload, &response)
}

func postJSONDecode(t *testing.T, url string, payload any, target any) {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s error = %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		t.Fatalf("POST %s status = %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("Decode(%s) error = %v", url, err)
	}
}

func getJSONOK(t *testing.T, url string, target any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s error = %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		t.Fatalf("GET %s status = %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("Decode(%s) error = %v", url, err)
	}
}
