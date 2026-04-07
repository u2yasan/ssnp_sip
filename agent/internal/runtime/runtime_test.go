package runtime

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/u2yasan/ssnp_sip/agent/internal/client"
	"github.com/u2yasan/ssnp_sip/agent/internal/config"
	"github.com/u2yasan/ssnp_sip/agent/internal/policy"
	"github.com/u2yasan/ssnp_sip/agent/internal/state"
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

func TestAgentHeartbeatFailsOnBrokenStateFile(t *testing.T) {
	tempDir := t.TempDir()
	privateKeyPath, publicKeyPath := writeTestKeys(t, tempDir)
	statePath := filepath.Join(tempDir, "state.json")
	if err := os.WriteFile(statePath, []byte("{invalid"), 0o600); err != nil {
		t.Fatalf("WriteFile(state) error = %v", err)
	}

	cfg := config.Config{
		NodeID:                    "node-abc",
		PortalBaseURL:             "http://mock.portal",
		AgentKeyPath:              privateKeyPath,
		AgentPublicKeyPath:        publicKeyPath,
		MonitoredEndpoint:         "https://node-01.example.net:3001",
		StatePath:                 statePath,
		TempDir:                   tempDir,
		RequestTimeoutSeconds:     5,
		HeartbeatJitterSecondsMax: 0,
		AgentVersion:              "1.0.0",
		EnrollmentGeneration:      1,
	}

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			t.Fatal("unexpected HTTP call")
			return nil, nil
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

	err = agent.sendHeartbeat(context.Background())
	if err == nil {
		t.Fatal("sendHeartbeat() error = nil, want broken state failure")
	}
}

func TestAgentEnrollFailsWhenPortalRejectsSignature(t *testing.T) {
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
			if r.URL.Path == "/api/v1/agent/enroll" {
				return jsonResponse(http.StatusUnauthorized, `{"status":"error","error_code":"invalid_signature"}`)
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

	err = agent.Enroll(context.Background(), "enroll-001")
	if err == nil {
		t.Fatal("Enroll() error = nil, want invalid signature rejection")
	}
	if !strings.Contains(err.Error(), "post /api/v1/agent/enroll failed") {
		t.Fatalf("Enroll() error = %v, want enroll post failure", err)
	}
}

func TestAgentHeartbeatFailsOnPortalTimeout(t *testing.T) {
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
			if r.URL.Path == "/api/v1/agent/heartbeat" {
				return nil, context.DeadlineExceeded
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

	err = agent.sendHeartbeat(context.Background())
	if err == nil {
		t.Fatal("sendHeartbeat() error = nil, want timeout failure")
	}
	if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("sendHeartbeat() error = %v, want deadline exceeded", err)
	}
}

func TestAgentSubmitTelemetryFailsWithoutWarningFlags(t *testing.T) {
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
			t.Fatal("unexpected HTTP call")
			return nil, nil
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

	err = agent.SubmitTelemetry(context.Background(), nil)
	if err == nil {
		t.Fatal("SubmitTelemetry() error = nil, want missing warning flags failure")
	}
}

func TestAgentPortalUnreachableWarningFlushesAfterRecovery(t *testing.T) {
	tempDir := t.TempDir()
	privateKeyPath, publicKeyPath := writeTestKeys(t, tempDir)

	var heartbeatCalls int
	var telemetryCalls int

	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/api/v1/agent/policy":
				return jsonResponse(http.StatusOK, `{
				"policy_version":"2026-04",
				"heartbeat_interval_seconds":300,
				"cpu_profile":{"id":"cpu-check-v1","duration_seconds":3,"warmup_seconds":1,"measured_seconds":1,"cooldown_seconds":1,"worker_cap":8,"workload_mix":{"hashing":0.50,"integer":0.30,"matrix":0.20},"acceptance_floor":{"type":"normalized_score","minimum":0.0}},
				"disk_profile":{"id":"disk-check-v1","duration_seconds":3,"warmup_seconds":1,"measured_seconds":1,"cooldown_seconds":1,"block_size_bytes":4096,"queue_depth":32,"concurrency":1,"read_ratio":0.70,"write_ratio":0.30,"acceptance_floor":{"type":"measured_iops","minimum":0}},
				"hardware_thresholds":{"cpu_cores_min":1,"ram_gb_min":1,"storage_gb_min":1,"ssd_required":false},
				"probe_thresholds":{"finalized_lag_max_blocks":2,"chain_lag_max_blocks":5},
				"reference_environment":{"id":"ref-env-2026-04","os_image_id":"ubuntu-24.04-lts","agent_version":"1.0.0","cpu_profile_id":"cpu-check-v1","disk_profile_id":"disk-check-v1","baseline_source_date":"2026-04-06"}
				}`)
			case "/api/v1/agent/heartbeat":
				heartbeatCalls++
				if heartbeatCalls <= 3 {
					return nil, context.DeadlineExceeded
				}
				return jsonResponse(http.StatusOK, `{"status":"accepted","node_id":"node-abc","received_at":"2026-04-06T10:35:00Z"}`)
			case "/api/v1/agent/telemetry":
				telemetryCalls++
				assertJSONFieldExists(t, r, "warning_flags")
				return jsonResponse(http.StatusOK, `{"status":"accepted","node_id":"node-abc","received_at":"2026-04-06T10:40:00Z"}`)
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

	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := agent.sendHeartbeat(ctx); err == nil {
			t.Fatalf("sendHeartbeat() #%d error = nil, want timeout", i+1)
		}
		if err := agent.recordPortalFailure(); err != nil {
			t.Fatalf("recordPortalFailure() error = %v", err)
		}
	}

	st, err := state.Load(cfg.StatePath)
	if err != nil {
		t.Fatalf("state.Load() error = %v", err)
	}
	if !st.PendingWarnings[warningPortalUnreachable] {
		t.Fatal("portal_unreachable not marked pending")
	}

	if err := agent.sendHeartbeat(ctx); err != nil {
		t.Fatalf("sendHeartbeat() recovery error = %v", err)
	}
	if err := agent.handlePortalRecovery(ctx, "2026-04"); err != nil {
		t.Fatalf("handlePortalRecovery() error = %v", err)
	}

	st, err = state.Load(cfg.StatePath)
	if err != nil {
		t.Fatalf("state.Load() error = %v", err)
	}
	if !st.ActiveWarnings[warningPortalUnreachable] {
		t.Fatal("portal_unreachable not marked active")
	}
	if st.PendingWarnings[warningPortalUnreachable] {
		t.Fatal("portal_unreachable still pending after recovery")
	}
	if telemetryCalls != 1 {
		t.Fatalf("telemetryCalls = %d, want 1", telemetryCalls)
	}
}

func TestAgentRunEmitsVotingKeyExpiryRiskWhenNearExpiry(t *testing.T) {
	tempDir := t.TempDir()
	privateKeyPath, publicKeyPath := writeTestKeys(t, tempDir)

	var telemetryCalls int
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/api/v1/agent/policy":
				return jsonResponse(http.StatusOK, `{
				"policy_version":"2026-04",
				"heartbeat_interval_seconds":1,
				"cpu_profile":{"id":"cpu-check-v1","duration_seconds":3,"warmup_seconds":1,"measured_seconds":1,"cooldown_seconds":1,"worker_cap":8,"workload_mix":{"hashing":0.50,"integer":0.30,"matrix":0.20},"acceptance_floor":{"type":"normalized_score","minimum":0.0}},
				"disk_profile":{"id":"disk-check-v1","duration_seconds":3,"warmup_seconds":1,"measured_seconds":1,"cooldown_seconds":1,"block_size_bytes":4096,"queue_depth":32,"concurrency":1,"read_ratio":0.70,"write_ratio":0.30,"acceptance_floor":{"type":"measured_iops","minimum":0}},
				"hardware_thresholds":{"cpu_cores_min":1,"ram_gb_min":1,"storage_gb_min":1,"ssd_required":false},
				"probe_thresholds":{"finalized_lag_max_blocks":2,"chain_lag_max_blocks":5},
				"reference_environment":{"id":"ref-env-2026-04","os_image_id":"ubuntu-24.04-lts","agent_version":"1.0.0","cpu_profile_id":"cpu-check-v1","disk_profile_id":"disk-check-v1","baseline_source_date":"2026-04-06"}
				}`)
			case "/api/v1/agent/heartbeat":
				return jsonResponse(http.StatusOK, `{"status":"accepted","node_id":"node-abc","received_at":"2026-04-06T10:35:00Z"}`)
			case "/api/v1/agent/telemetry":
				telemetryCalls++
				assertRequestWarningFlagEquals(t, r, warningVotingKeyExpiryRisk)
				return jsonResponse(http.StatusOK, `{"status":"accepted","node_id":"node-abc","received_at":"2026-04-06T10:40:00Z"}`)
			case "/node/info":
				return jsonResponse(http.StatusOK, `{"publicKey":"NODE_MAIN_PUBLIC_KEY"}`)
			case "/chain/info":
				return jsonResponse(http.StatusOK, `{"height":"40320"}`)
			case "/network/properties":
				return jsonResponse(http.StatusOK, `{"network":{"chain":{"votingSetGrouping":"1440","blockGenerationTargetTime":"30s"}}}`)
			case "/accounts/NODE_MAIN_PUBLIC_KEY":
				return jsonResponse(http.StatusOK, `{"account":{"supplementalPublicKeys":{"voting":{"publicKeys":[{"publicKey":"VOTE_A","startEpoch":"1","endEpoch":"34"}]}}}}`)
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

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	if err := agent.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if telemetryCalls != 1 {
		t.Fatalf("telemetryCalls = %d, want 1", telemetryCalls)
	}
}

func TestAgentRunDoesNotResendVotingKeyExpiryRiskWhenAlreadyActive(t *testing.T) {
	tempDir := t.TempDir()
	privateKeyPath, publicKeyPath := writeTestKeys(t, tempDir)
	statePath := filepath.Join(tempDir, "state.json")
	if err := state.Save(statePath, state.State{
		ActiveWarnings: map[string]bool{
			warningVotingKeyExpiryRisk: true,
		},
	}); err != nil {
		t.Fatalf("state.Save() error = %v", err)
	}

	var telemetryCalls int
	var heartbeatCalls int
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/api/v1/agent/policy":
				return jsonResponse(http.StatusOK, `{
				"policy_version":"2026-04",
				"heartbeat_interval_seconds":1,
				"cpu_profile":{"id":"cpu-check-v1","duration_seconds":3,"warmup_seconds":1,"measured_seconds":1,"cooldown_seconds":1,"worker_cap":8,"workload_mix":{"hashing":0.50,"integer":0.30,"matrix":0.20},"acceptance_floor":{"type":"normalized_score","minimum":0.0}},
				"disk_profile":{"id":"disk-check-v1","duration_seconds":3,"warmup_seconds":1,"measured_seconds":1,"cooldown_seconds":1,"block_size_bytes":4096,"queue_depth":32,"concurrency":1,"read_ratio":0.70,"write_ratio":0.30,"acceptance_floor":{"type":"measured_iops","minimum":0}},
				"hardware_thresholds":{"cpu_cores_min":1,"ram_gb_min":1,"storage_gb_min":1,"ssd_required":false},
				"probe_thresholds":{"finalized_lag_max_blocks":2,"chain_lag_max_blocks":5},
				"reference_environment":{"id":"ref-env-2026-04","os_image_id":"ubuntu-24.04-lts","agent_version":"1.0.0","cpu_profile_id":"cpu-check-v1","disk_profile_id":"disk-check-v1","baseline_source_date":"2026-04-06"}
				}`)
			case "/api/v1/agent/heartbeat":
				heartbeatCalls++
				return jsonResponse(http.StatusOK, `{"status":"accepted","node_id":"node-abc","received_at":"2026-04-06T10:35:00Z"}`)
			case "/api/v1/agent/telemetry":
				telemetryCalls++
				return jsonResponse(http.StatusOK, `{"status":"accepted","node_id":"node-abc","received_at":"2026-04-06T10:40:00Z"}`)
			case "/node/info":
				return jsonResponse(http.StatusOK, `{"publicKey":"NODE_MAIN_PUBLIC_KEY"}`)
			case "/chain/info":
				return jsonResponse(http.StatusOK, `{"height":"40320"}`)
			case "/network/properties":
				return jsonResponse(http.StatusOK, `{"network":{"chain":{"votingSetGrouping":"1440","blockGenerationTargetTime":"30s"}}}`)
			case "/accounts/NODE_MAIN_PUBLIC_KEY":
				return jsonResponse(http.StatusOK, `{"account":{"supplementalPublicKeys":{"voting":{"publicKeys":[{"publicKey":"VOTE_A","startEpoch":"1","endEpoch":"34"}]}}}}`)
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
		StatePath:                 statePath,
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

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	if err := agent.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if telemetryCalls != 0 {
		t.Fatalf("telemetryCalls = %d, want 0", telemetryCalls)
	}
	if heartbeatCalls == 0 {
		t.Fatal("heartbeatCalls = 0, want at least 1")
	}
}

func TestAgentRunContinuesWhenSymbolAPIUnavailable(t *testing.T) {
	tempDir := t.TempDir()
	privateKeyPath, publicKeyPath := writeTestKeys(t, tempDir)

	var telemetryCalls int
	var heartbeatCalls int
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/api/v1/agent/policy":
				return jsonResponse(http.StatusOK, `{
				"policy_version":"2026-04",
				"heartbeat_interval_seconds":1,
				"cpu_profile":{"id":"cpu-check-v1","duration_seconds":3,"warmup_seconds":1,"measured_seconds":1,"cooldown_seconds":1,"worker_cap":8,"workload_mix":{"hashing":0.50,"integer":0.30,"matrix":0.20},"acceptance_floor":{"type":"normalized_score","minimum":0.0}},
				"disk_profile":{"id":"disk-check-v1","duration_seconds":3,"warmup_seconds":1,"measured_seconds":1,"cooldown_seconds":1,"block_size_bytes":4096,"queue_depth":32,"concurrency":1,"read_ratio":0.70,"write_ratio":0.30,"acceptance_floor":{"type":"measured_iops","minimum":0}},
				"hardware_thresholds":{"cpu_cores_min":1,"ram_gb_min":1,"storage_gb_min":1,"ssd_required":false},
				"probe_thresholds":{"finalized_lag_max_blocks":2,"chain_lag_max_blocks":5},
				"reference_environment":{"id":"ref-env-2026-04","os_image_id":"ubuntu-24.04-lts","agent_version":"1.0.0","cpu_profile_id":"cpu-check-v1","disk_profile_id":"disk-check-v1","baseline_source_date":"2026-04-06"}
				}`)
			case "/api/v1/agent/heartbeat":
				heartbeatCalls++
				return jsonResponse(http.StatusOK, `{"status":"accepted","node_id":"node-abc","received_at":"2026-04-06T10:35:00Z"}`)
			case "/api/v1/agent/telemetry":
				telemetryCalls++
				return jsonResponse(http.StatusOK, `{"status":"accepted","node_id":"node-abc","received_at":"2026-04-06T10:40:00Z"}`)
			case "/node/info":
				return jsonResponse(http.StatusServiceUnavailable, `{"status":"error"}`)
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

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	if err := agent.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if telemetryCalls != 0 {
		t.Fatalf("telemetryCalls = %d, want 0", telemetryCalls)
	}
	if heartbeatCalls == 0 {
		t.Fatal("heartbeatCalls = 0, want at least 1")
	}
}

func TestAgentRunEmitsCertificateExpiryRiskWhenTLSCertNearExpiry(t *testing.T) {
	tempDir := t.TempDir()
	privateKeyPath, publicKeyPath := writeTestKeys(t, tempDir)

	certPEM, keyPEM := writeShortLivedTLSCert(t)
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair() error = %v", err)
	}
	tlsServer := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	tlsServer.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	tlsServer.StartTLS()
	defer tlsServer.Close()

	var telemetryCalls int
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/api/v1/agent/policy":
				return jsonResponse(http.StatusOK, `{
				"policy_version":"2026-04",
				"heartbeat_interval_seconds":1,
				"cpu_profile":{"id":"cpu-check-v1","duration_seconds":3,"warmup_seconds":1,"measured_seconds":1,"cooldown_seconds":1,"worker_cap":8,"workload_mix":{"hashing":0.50,"integer":0.30,"matrix":0.20},"acceptance_floor":{"type":"normalized_score","minimum":0.0}},
				"disk_profile":{"id":"disk-check-v1","duration_seconds":3,"warmup_seconds":1,"measured_seconds":1,"cooldown_seconds":1,"block_size_bytes":4096,"queue_depth":32,"concurrency":1,"read_ratio":0.70,"write_ratio":0.30,"acceptance_floor":{"type":"measured_iops","minimum":0}},
				"hardware_thresholds":{"cpu_cores_min":1,"ram_gb_min":1,"storage_gb_min":1,"ssd_required":false},
				"probe_thresholds":{"finalized_lag_max_blocks":2,"chain_lag_max_blocks":5},
				"reference_environment":{"id":"ref-env-2026-04","os_image_id":"ubuntu-24.04-lts","agent_version":"1.0.0","cpu_profile_id":"cpu-check-v1","disk_profile_id":"disk-check-v1","baseline_source_date":"2026-04-06"}
				}`)
			case "/api/v1/agent/heartbeat":
				return jsonResponse(http.StatusOK, `{"status":"accepted","node_id":"node-abc","received_at":"2026-04-06T10:35:00Z"}`)
			case "/api/v1/agent/telemetry":
				telemetryCalls++
				assertRequestWarningFlagEquals(t, r, warningCertificateExpiryRisk)
				return jsonResponse(http.StatusOK, `{"status":"accepted","node_id":"node-abc","received_at":"2026-04-06T10:40:00Z"}`)
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
		MonitoredEndpoint:         tlsServer.URL,
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

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()
	if err := agent.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if telemetryCalls != 1 {
		t.Fatalf("telemetryCalls = %d, want 1", telemetryCalls)
	}
}

func writeTestKeys(t *testing.T, dir string) (string, string) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}

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
		t.Fatalf("WriteFile(privateKey) error = %v", err)
	}
	if err := os.WriteFile(publicKeyPath, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER}), 0o600); err != nil {
		t.Fatalf("WriteFile(publicKey) error = %v", err)
	}

	return privateKeyPath, publicKeyPath
}

func assertJSONFieldExists(t *testing.T, r *http.Request, field string) {
	t.Helper()

	defer r.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if _, ok := payload[field]; !ok {
		t.Fatalf("payload missing field %q", field)
	}
}

func assertRequestWarningFlagEquals(t *testing.T, r *http.Request, expected string) {
	t.Helper()

	defer r.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	flagsAny, ok := payload["warning_flags"].([]any)
	if !ok || len(flagsAny) != 1 {
		t.Fatalf("warning_flags = %#v, want single value %q", payload["warning_flags"], expected)
	}
	if flag, ok := flagsAny[0].(string); !ok || flag != expected {
		t.Fatalf("warning flag = %#v, want %q", flagsAny[0], expected)
	}
}

func writeShortLivedTLSCert(t *testing.T) ([]byte, []byte) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		NotBefore:   time.Now().Add(-1 * time.Hour),
		NotAfter:    time.Now().Add(7 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	return certPEM, keyPEM
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}
