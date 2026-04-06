package runtime

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/u2yasan/ssnp_sip/agent/internal/client"
	"github.com/u2yasan/ssnp_sip/agent/internal/config"
	"github.com/u2yasan/ssnp_sip/agent/internal/policy"
)

func TestAgentEnrollHeartbeatAndChecks(t *testing.T) {
	tempDir := t.TempDir()
	privateKeyPath, publicKeyPath := writeTestKeys(t, tempDir)

	var mu sync.Mutex
	var enrollCalls int
	var heartbeatCalls int
	var checkCalls int

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
	agent, err := NewAgentWithClients(cfg, postClient, policyClient)
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
