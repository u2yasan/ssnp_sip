package runtime

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u2yasan/ssnp_sip/agent/internal/client"
	"github.com/u2yasan/ssnp_sip/agent/internal/config"
	"github.com/u2yasan/ssnp_sip/agent/internal/policy"
	"github.com/u2yasan/ssnp_sip/agent/internal/symbol"
)

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
