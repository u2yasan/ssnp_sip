package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/u2yasan/ssnp_sip/probe/internal/config"
)

func TestRunOncePostsDerivedProbeEvents(t *testing.T) {
	var submitted []map[string]any
	portalServer := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/probes/events" {
			http.NotFound(w, r)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		submitted = append(submitted, payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer portalServer.Close()

	sourceServer := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chain/info" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"height":"120","latestFinalizedBlock":{"height":"118"}}`))
	}))
	defer sourceServer.Close()

	targetServer := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/node/health":
			_, _ = w.Write([]byte(`{"status":{"apiNode":"up","node":"up"}}`))
		case "/chain/info":
			_, _ = w.Write([]byte(`{"height":"118","latestFinalizedBlock":{"height":"117"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer targetServer.Close()

	cfg := config.Config{
		PortalBaseURL:         portalServer.URL,
		RegionID:              "ap-sg-1",
		SourceEndpoint:        sourceServer.URL,
		RequestTimeoutSeconds: 2,
		PollIntervalSeconds:   30,
		Targets: []config.Target{
			{NodeID: "node-abc", Endpoint: targetServer.URL},
		},
	}

	logger := bytes.NewBuffer(nil)
	worker := New(cfg, nil)
	worker.logger.SetOutput(logger)
	worker.now = func() time.Time {
		return time.Date(2026, 4, 7, 1, 2, 3, 0, time.UTC)
	}

	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(submitted) != 1 {
		t.Fatalf("len(submitted) = %d, want 1", len(submitted))
	}
	payload := submitted[0]
	if payload["node_id"] != "node-abc" || payload["region_id"] != "ap-sg-1" {
		t.Fatalf("payload = %#v", payload)
	}
	if payload["availability_up"] != true {
		t.Fatalf("payload availability = %#v, want true", payload["availability_up"])
	}
	if payload["finalized_lag_blocks"] != float64(1) || payload["chain_lag_blocks"] != float64(2) {
		t.Fatalf("payload lag fields = %#v", payload)
	}
}

func TestRunOncePostsUnavailableEventOnTargetFailure(t *testing.T) {
	var submitted []map[string]any
	portalServer := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		submitted = append(submitted, payload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer portalServer.Close()

	sourceServer := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"height":"120","latestFinalizedBlock":{"height":"118"}}`))
	}))
	defer sourceServer.Close()

	targetServer := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer targetServer.Close()

	cfg := config.Config{
		PortalBaseURL:         portalServer.URL,
		RegionID:              "us-va-1",
		SourceEndpoint:        sourceServer.URL,
		RequestTimeoutSeconds: 2,
		PollIntervalSeconds:   30,
		Targets: []config.Target{
			{NodeID: "node-abc", Endpoint: targetServer.URL},
		},
	}

	worker := New(cfg, nil)
	worker.now = func() time.Time {
		return time.Date(2026, 4, 7, 1, 2, 3, 0, time.UTC)
	}
	if err := worker.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if len(submitted) != 1 {
		t.Fatalf("len(submitted) = %d, want 1", len(submitted))
	}
	if submitted[0]["availability_up"] != false {
		t.Fatalf("payload = %#v, want availability_up=false", submitted[0])
	}
	if submitted[0]["error_code"] != "health_request_failed" {
		t.Fatalf("payload = %#v, want health_request_failed", submitted[0])
	}
}

func newIPv4Server(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("sandbox does not allow local listener: %v", err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	return server
}
