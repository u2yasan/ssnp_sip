package runtime

import (
	"context"
	"crypto/tls"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/u2yasan/ssnp_sip/agent/internal/client"
	"github.com/u2yasan/ssnp_sip/agent/internal/config"
	"github.com/u2yasan/ssnp_sip/agent/internal/policy"
	"github.com/u2yasan/ssnp_sip/agent/internal/symbol"
)

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
	listener, err := tls.Listen("tcp4", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Skipf("sandbox does not allow local TLS listener: %v", err)
	}
	tlsServer := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})}
	go func() {
		_ = tlsServer.Serve(listener)
	}()
	defer tlsServer.Shutdown(context.Background())

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
		MonitoredEndpoint:         "https://" + listener.Addr().String(),
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
