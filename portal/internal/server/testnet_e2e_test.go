package server

import (
	"bytes"
	"context"
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

func TestTestnetOperableE2E(t *testing.T) {
	repoRoot, err := filepath.Abs("../../../")
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}
	agentDir := agentPythonDir(t, repoRoot)
	probeDir := filepath.Join(repoRoot, "probe")
	probeBinary := buildProbeBinary(t, probeDir)

	sourceURL, closeSource := startTestHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/chain/info":
			_, _ = w.Write([]byte(`{"height":"103","latestFinalizedBlock":{"height":"102"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer closeSource()

	targetURL, closeTarget := startTestHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/node/health":
			_, _ = w.Write([]byte(`{"status":{"apiNode":"up","node":"up"}}`))
		case "/chain/info":
			_, _ = w.Write([]byte(`{"height":"101","latestFinalizedBlock":{"height":"100"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer closeTarget()

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
	configPath := writeAgentConfigWithEndpoint(t, tempDir, "http://"+listener.Addr().String(), privateKeyPath, publicKeyPath, targetURL)

	challengeID := issueEnrollmentChallengeHTTP(t, "http://"+listener.Addr().String(), "node-abc")
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

	waitForHeartbeatSequence(t, srv, 6*time.Second, 2)
	runAgentCommand(t, agentDir, configPath, "check", "--event-type", "registration", "--event-id", "testnet-check-001")
	runAgentCommand(t, agentDir, configPath, "telemetry", "--warning-flag", "portal_unreachable")

	probeConfigAP := writeProbeConfig(t, tempDir, "http://"+listener.Addr().String(), sourceURL, "ap-sg-1", map[string]string{
		"node-abc": targetURL,
	})
	probeConfigUS := writeProbeConfig(t, tempDir, "http://"+listener.Addr().String(), sourceURL, "us-va-1", map[string]string{
		"node-abc": targetURL,
	})
	runProbeCommand(t, probeBinary, probeDir, probeConfigAP, "run-once")
	runProbeCommand(t, probeBinary, probeDir, probeConfigUS, "run-once")

	now := time.Now().UTC()
	postJSONOK(t, "http://"+listener.Addr().String()+"/api/v1/voting-key-evidence", map[string]any{
		"node_id":                    "node-abc",
		"evidence_ref":               "testnet-vk-001",
		"observed_at":                now.Add(2 * time.Second).Format(time.RFC3339),
		"current_epoch":              12,
		"voting_key_present":         true,
		"voting_key_valid_for_epoch": true,
		"source":                     "external_probe",
	})

	if err := runCmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("run Signal() error = %v", err)
	}
	if err := runCmd.Wait(); err != nil {
		t.Fatalf("run Wait() error = %v\noutput:\n%s", err, runOutput.String())
	}

	dateUTC := now.Format("2006-01-02")

	var publicStatus struct {
		Items []struct {
			NodeID            string  `json:"node_id"`
			Qualified         bool    `json:"qualified"`
			RewardEligible    bool    `json:"reward_eligible"`
			AvailabilityRatio float64 `json:"availability_ratio"`
			FinalizedLagRatio float64 `json:"finalized_lag_ratio"`
			ChainLagRatio     float64 `json:"chain_lag_ratio"`
			QualifiedRank     int     `json:"qualified_rank"`
		} `json:"items"`
	}
	getJSONOK(t, "http://"+listener.Addr().String()+"/api/v1/public-node-status/"+dateUTC, &publicStatus)
	if len(publicStatus.Items) != 1 ||
		publicStatus.Items[0].NodeID != "node-abc" ||
		!publicStatus.Items[0].Qualified ||
		!publicStatus.Items[0].RewardEligible ||
		publicStatus.Items[0].AvailabilityRatio < 0.99 ||
		publicStatus.Items[0].FinalizedLagRatio < 0.95 ||
		publicStatus.Items[0].ChainLagRatio < 0.95 ||
		publicStatus.Items[0].QualifiedRank != 1 {
		t.Fatalf("public status = %#v", publicStatus)
	}

	var rewardAllocations struct {
		Items []struct {
			NodeID          string  `json:"node_id"`
			Qualified       bool    `json:"qualified"`
			RewardEligible  bool    `json:"reward_eligible"`
			AllocationShare float64 `json:"allocation_share"`
			AllocationUnits float64 `json:"allocation_units"`
		} `json:"items"`
	}
	getJSONOK(t, "http://"+listener.Addr().String()+"/api/v1/reward-allocations/"+dateUTC, &rewardAllocations)
	if len(rewardAllocations.Items) != 1 ||
		rewardAllocations.Items[0].NodeID != "node-abc" ||
		!rewardAllocations.Items[0].Qualified ||
		!rewardAllocations.Items[0].RewardEligible ||
		rewardAllocations.Items[0].AllocationShare <= 0 ||
		rewardAllocations.Items[0].AllocationUnits <= 0 {
		t.Fatalf("reward allocations = %#v", rewardAllocations)
	}

	var antiConcentration struct {
		Items []map[string]any `json:"items"`
	}
	getJSONOK(t, "http://"+listener.Addr().String()+"/api/v1/anti-concentration-evidence/"+dateUTC, &antiConcentration)
	if len(antiConcentration.Items) != 0 {
		t.Fatalf("anti concentration evidence = %#v, want empty for seed-only node", antiConcentration)
	}
}

func startTestHTTPServer(t *testing.T, handler http.Handler) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("sandbox does not allow local listener: %v", err)
	}
	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()
	return "http://" + listener.Addr().String(), func() {
		_ = server.Shutdown(context.Background())
	}
}
