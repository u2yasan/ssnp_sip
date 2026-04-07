package policy

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFetchPolicySuccessWithProbeThresholds(t *testing.T) {
	client := NewClientWithHTTP("http://mock.portal", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonHTTPResponse(http.StatusOK, `{
			"policy_version":"2026-04",
			"heartbeat_interval_seconds":300,
			"cpu_profile":{"id":"cpu-check-v1"},
			"disk_profile":{"id":"disk-check-v1"},
			"hardware_thresholds":{"cpu_cores_min":8,"ram_gb_min":32,"storage_gb_min":750,"ssd_required":true},
			"probe_thresholds":{"finalized_lag_max_blocks":2,"chain_lag_max_blocks":5},
			"reference_environment":{"id":"ref-env-2026-04","os_image_id":"ubuntu-24.04-lts","agent_version":"1.0.0","cpu_profile_id":"cpu-check-v1","disk_profile_id":"disk-check-v1","baseline_source_date":"2026-04-06"}
		}`)
		}),
	})
	got, err := client.Fetch(context.Background(), "node-1", "fp-1")
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
	if got.ProbeThresholds.FinalizedLagMaxBlocks != 2 {
		t.Fatalf("FinalizedLagMaxBlocks = %d, want 2", got.ProbeThresholds.FinalizedLagMaxBlocks)
	}
	if got.ProbeThresholds.ChainLagMaxBlocks != 5 {
		t.Fatalf("ChainLagMaxBlocks = %d, want 5", got.ProbeThresholds.ChainLagMaxBlocks)
	}
}

func TestFetchPolicyFailsClosedOnInvalidProbeThresholds(t *testing.T) {
	tests := []struct {
		name      string
		threshold string
	}{
		{
			name:      "missing finalized threshold",
			threshold: `"probe_thresholds":{"chain_lag_max_blocks":5}`,
		},
		{
			name:      "exact boundary invalid finalized threshold",
			threshold: `"probe_thresholds":{"finalized_lag_max_blocks":0,"chain_lag_max_blocks":5}`,
		},
		{
			name:      "exact boundary invalid chain threshold",
			threshold: `"probe_thresholds":{"finalized_lag_max_blocks":2,"chain_lag_max_blocks":0}`,
		},
		{
			name:      "below boundary invalid chain threshold",
			threshold: `"probe_thresholds":{"finalized_lag_max_blocks":2,"chain_lag_max_blocks":-1}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClientWithHTTP("http://mock.portal", &http.Client{
				Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					return jsonHTTPResponse(http.StatusOK, `{
					"policy_version":"2026-04",
					"heartbeat_interval_seconds":300,
					"cpu_profile":{"id":"cpu-check-v1"},
					"disk_profile":{"id":"disk-check-v1"},
					"hardware_thresholds":{"cpu_cores_min":8,"ram_gb_min":32,"storage_gb_min":750,"ssd_required":true},
					`+tt.threshold+`,
					"reference_environment":{"id":"ref-env-2026-04","os_image_id":"ubuntu-24.04-lts","agent_version":"1.0.0","cpu_profile_id":"cpu-check-v1","disk_profile_id":"disk-check-v1","baseline_source_date":"2026-04-06"}
				}`)
				}),
			})
			if _, err := client.Fetch(context.Background(), "node-1", "fp-1"); err == nil {
				t.Fatal("Fetch() error = nil, want invalid policy")
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonHTTPResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}
