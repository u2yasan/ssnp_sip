package runtime

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/u2yasan/ssnp_sip/agent/internal/client"
	"github.com/u2yasan/ssnp_sip/agent/internal/config"
	"github.com/u2yasan/ssnp_sip/agent/internal/policy"
	"github.com/u2yasan/ssnp_sip/agent/internal/symbol"
)

func TestAgentEnrollHeartbeatAndChecks(t *testing.T) {
	tempDir := t.TempDir()
	privateKeyPath, publicKeyPath := writeTestKeys(t, tempDir)

	var mu sync.Mutex
	var enrollCalls int
	var heartbeatCalls int
	var checkCalls int
	var telemetryCalls int

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/api/v1/agent/policy":
				return jsonResponse(http.StatusOK, `{
				"policy_version":"2026-04",
				"heartbeat_interval_seconds":300,
				"cpu_profile":{
					"id":"cpu-check-v1",
					"duration_seconds":3,
					"warmup_seconds":1,
					"measured_seconds":1,
					"cooldown_seconds":1,
					"worker_cap":8,
					"workload_mix":{"hashing":0.50,"integer":0.30,"matrix":0.20},
					"acceptance_floor":{"type":"normalized_score","minimum":0.0}
				},
				"disk_profile":{
					"id":"disk-check-v1",
					"duration_seconds":3,
					"warmup_seconds":1,
					"measured_seconds":1,
					"cooldown_seconds":1,
					"block_size_bytes":4096,
					"queue_depth":32,
					"concurrency":1,
					"read_ratio":0.70,
					"write_ratio":0.30,
					"acceptance_floor":{"type":"measured_iops","minimum":0}
				},
				"hardware_thresholds":{
					"cpu_cores_min":1,
					"ram_gb_min":1,
					"storage_gb_min":1,
					"ssd_required":false
				},
				"probe_thresholds":{
					"finalized_lag_max_blocks":2,
					"chain_lag_max_blocks":5
				},
				"reference_environment":{
					"id":"ref-env-2026-04",
					"os_image_id":"ubuntu-24.04-lts",
					"agent_version":"1.0.0",
					"cpu_profile_id":"cpu-check-v1",
					"disk_profile_id":"disk-check-v1",
					"baseline_source_date":"2026-04-06"
				}
				}`)
			case "/api/v1/agent/enroll":
				mu.Lock()
				enrollCalls++
				mu.Unlock()
				assertJSONFieldExists(t, r, "signature")
				return jsonResponse(http.StatusOK, `{"status":"ok","node_id":"node-abc","agent_key_fingerprint":"fp","policy_version":"2026-04"}`)
			case "/api/v1/agent/heartbeat":
				mu.Lock()
				heartbeatCalls++
				mu.Unlock()
				assertJSONFieldExists(t, r, "signature")
				return jsonResponse(http.StatusOK, `{"status":"accepted","node_id":"node-abc","received_at":"2026-04-06T10:35:00Z"}`)
			case "/api/v1/agent/checks":
				mu.Lock()
				checkCalls++
				mu.Unlock()
				assertJSONFieldExists(t, r, "overall_passed")
				return jsonResponse(http.StatusOK, `{"status":"accepted","node_id":"node-abc","event_id":"check-001","overall_passed":true,"received_at":"2026-04-06T10:30:05Z"}`)
			case "/api/v1/agent/telemetry":
				mu.Lock()
				telemetryCalls++
				mu.Unlock()
				assertJSONFieldExists(t, r, "warning_flags")
				return jsonResponse(http.StatusOK, `{"status":"accepted","node_id":"node-abc","received_at":"2026-04-06T10:40:00Z"}`)
			default:
				return jsonResponse(http.StatusNotFound, `{"status":"error","error_code":"unknown","message":"not found"}`)
			}
		}),
	}

	cfg := config.Config{
		NodeID:                    "node-abc",
		PortalBaseURL:             "http://mock.portal",
		AgentKeyPath:              privateKeyPath,
		AgentPublicKeyPath:        publicKeyPath,
		MonitoredEndpoint:         "https://node-01.example.net:3001",
		StatePath:                 filepath.Join(tempDir, "state.json"),
		TempDir:                   tempDir,
		RequestTimeoutSeconds:     5,
		HeartbeatJitterSecondsMax: 0,
		AgentVersion:              "1.0.0",
		EnrollmentGeneration:      1,
	}

	postClient := client.NewWithHTTPClient(cfg.PortalBaseURL, httpClient)
	policyClient := policy.NewClientWithHTTP(cfg.PortalBaseURL, httpClient)
	symbolClient := symbol.NewClientWithHTTP(cfg.MonitoredEndpoint, httpClient)
	agent, err := NewAgentWithClients(cfg, postClient, policyClient, symbolClient)
	if err != nil {
		t.Fatalf("NewAgent() error = %v", err)
	}

	ctx := context.Background()

	if err := agent.Enroll(ctx, "enroll-001"); err != nil {
		t.Fatalf("Enroll() error = %v", err)
	}

	if err := agent.sendHeartbeat(ctx); err != nil {
		t.Fatalf("sendHeartbeat() error = %v", err)
	}

	if err := agent.RunChecks(ctx, "registration", "check-001"); err != nil {
		t.Fatalf("RunChecks() error = %v", err)
	}
	if err := agent.SubmitTelemetry(ctx, []string{"voting_key_expiry_risk"}); err != nil {
		t.Fatalf("SubmitTelemetry() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if enrollCalls != 1 {
		t.Fatalf("enrollCalls = %d, want 1", enrollCalls)
	}
	if heartbeatCalls != 1 {
		t.Fatalf("heartbeatCalls = %d, want 1", heartbeatCalls)
	}
	if checkCalls != 1 {
		t.Fatalf("checkCalls = %d, want 1", checkCalls)
	}
	if telemetryCalls < 1 {
		t.Fatalf("telemetryCalls = %d, want at least 1", telemetryCalls)
	}
}

func TestAgentRunFailsWhenPolicyFetchFails(t *testing.T) {
	tempDir := t.TempDir()
	privateKeyPath, publicKeyPath := writeTestKeys(t, tempDir)

	cfg := config.Config{
		NodeID:                    "node-abc",
		PortalBaseURL:             "http://mock.portal",
		AgentKeyPath:              privateKeyPath,
		AgentPublicKeyPath:        publicKeyPath,
		MonitoredEndpoint:         "https://node-01.example.net:3001",
		StatePath:                 filepath.Join(tempDir, "state.json"),
		TempDir:                   tempDir,
		RequestTimeoutSeconds:     5,
		HeartbeatJitterSecondsMax: 0,
		AgentVersion:              "1.0.0",
		EnrollmentGeneration:      1,
	}

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/api/v1/agent/policy" {
				return jsonResponse(http.StatusInternalServerError, `{"status":"error","error_code":"policy_unavailable"}`)
			}
			return jsonResponse(http.StatusNotFound, `{"status":"error","error_code":"unknown"}`)
		}),
	}

	agent, err := NewAgentWithClients(
		cfg,
		client.NewWithHTTPClient(cfg.PortalBaseURL, httpClient),
		policy.NewClientWithHTTP(cfg.PortalBaseURL, httpClient),
		symbol.NewClientWithHTTP(cfg.MonitoredEndpoint, httpClient),
	)
	if err != nil {
		t.Fatalf("NewAgentWithClients() error = %v", err)
	}

	err = agent.Run(context.Background())
	if err == nil {
		t.Fatal("Run() error = nil, want policy fetch failure")
	}
	if !strings.Contains(err.Error(), "policy fetch failed") {
		t.Fatalf("Run() error = %v, want policy fetch failed", err)
	}
}

func TestAgentRunFailsWhenPolicyJSONIsInvalid(t *testing.T) {
	tempDir := t.TempDir()
	privateKeyPath, publicKeyPath := writeTestKeys(t, tempDir)

	cfg := config.Config{
		NodeID:                    "node-abc",
		PortalBaseURL:             "http://mock.portal",
		AgentKeyPath:              privateKeyPath,
		AgentPublicKeyPath:        publicKeyPath,
		MonitoredEndpoint:         "https://node-01.example.net:3001",
		StatePath:                 filepath.Join(tempDir, "state.json"),
		TempDir:                   tempDir,
		RequestTimeoutSeconds:     5,
		HeartbeatJitterSecondsMax: 0,
		AgentVersion:              "1.0.0",
		EnrollmentGeneration:      1,
	}

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/api/v1/agent/policy" {
				return jsonResponse(http.StatusOK, `{"policy_version":`)
			}
			return jsonResponse(http.StatusNotFound, `{"status":"error","error_code":"unknown"}`)
		}),
	}

	agent, err := NewAgentWithClients(
		cfg,
		client.NewWithHTTPClient(cfg.PortalBaseURL, httpClient),
		policy.NewClientWithHTTP(cfg.PortalBaseURL, httpClient),
		symbol.NewClientWithHTTP(cfg.MonitoredEndpoint, httpClient),
	)
	if err != nil {
		t.Fatalf("NewAgentWithClients() error = %v", err)
	}

	err = agent.Run(context.Background())
	if err == nil {
		t.Fatal("Run() error = nil, want JSON decode failure")
	}
}

func TestAgentRunChecksFailsWhenPortalRejectsPolicyVersion(t *testing.T) {
	tempDir := t.TempDir()
	privateKeyPath, publicKeyPath := writeTestKeys(t, tempDir)

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/api/v1/agent/policy":
				return jsonResponse(http.StatusOK, `{
				"policy_version":"2026-04",
				"heartbeat_interval_seconds":300,
				"cpu_profile":{
					"id":"cpu-check-v1",
					"duration_seconds":3,
					"warmup_seconds":1,
					"measured_seconds":1,
					"cooldown_seconds":1,
					"worker_cap":8,
					"workload_mix":{"hashing":0.50,"integer":0.30,"matrix":0.20},
					"acceptance_floor":{"type":"normalized_score","minimum":0.0}
				},
				"disk_profile":{
					"id":"disk-check-v1",
					"duration_seconds":3,
					"warmup_seconds":1,
					"measured_seconds":1,
					"cooldown_seconds":1,
					"block_size_bytes":4096,
					"queue_depth":32,
					"concurrency":1,
					"read_ratio":0.70,
					"write_ratio":0.30,
					"acceptance_floor":{"type":"measured_iops","minimum":0}
				},
				"hardware_thresholds":{
					"cpu_cores_min":1,
					"ram_gb_min":1,
					"storage_gb_min":1,
					"ssd_required":false
				},
				"probe_thresholds":{
					"finalized_lag_max_blocks":2,
					"chain_lag_max_blocks":5
				},
				"reference_environment":{
					"id":"ref-env-2026-04",
					"os_image_id":"ubuntu-24.04-lts",
					"agent_version":"1.0.0",
					"cpu_profile_id":"cpu-check-v1",
					"disk_profile_id":"disk-check-v1",
					"baseline_source_date":"2026-04-06"
				}
				}`)
			case "/api/v1/agent/checks":
				return jsonResponse(http.StatusConflict, `{"status":"error","error_code":"policy_version_mismatch"}`)
			default:
				return jsonResponse(http.StatusNotFound, `{"status":"error","error_code":"unknown"}`)
			}
		}),
	}

	cfg := config.Config{
		NodeID:                    "node-abc",
		PortalBaseURL:             "http://mock.portal",
		AgentKeyPath:              privateKeyPath,
		AgentPublicKeyPath:        publicKeyPath,
		MonitoredEndpoint:         "https://node-01.example.net:3001",
		StatePath:                 filepath.Join(tempDir, "state.json"),
		TempDir:                   tempDir,
		RequestTimeoutSeconds:     5,
		HeartbeatJitterSecondsMax: 0,
		AgentVersion:              "1.0.0",
		EnrollmentGeneration:      1,
	}

	agent, err := NewAgentWithClients(
		cfg,
		client.NewWithHTTPClient(cfg.PortalBaseURL, httpClient),
		policy.NewClientWithHTTP(cfg.PortalBaseURL, httpClient),
		symbol.NewClientWithHTTP(cfg.MonitoredEndpoint, httpClient),
	)
	if err != nil {
		t.Fatalf("NewAgentWithClients() error = %v", err)
	}

	err = agent.RunChecks(context.Background(), "registration", "check-001")
	if err == nil {
		t.Fatal("RunChecks() error = nil, want portal reject")
	}
	if !strings.Contains(err.Error(), "post /api/v1/agent/checks failed") {
		t.Fatalf("RunChecks() error = %v, want checks post failure", err)
	}
}
