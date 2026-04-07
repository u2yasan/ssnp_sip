package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPolicySuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	content := `policy_version: "2026-04"
heartbeat_interval_seconds: 300
cpu_profile:
  id: "cpu-check-v1"
  duration_seconds: 180
  warmup_seconds: 30
  measured_seconds: 120
  cooldown_seconds: 30
  worker_cap: 8
  workload_mix:
    hashing: 0.5
    integer: 0.3
    matrix: 0.2
  acceptance_floor:
    type: "normalized_score"
    minimum: 1
disk_profile:
  id: "disk-check-v1"
  duration_seconds: 60
  warmup_seconds: 10
  measured_seconds: 40
  cooldown_seconds: 10
  block_size_bytes: 4096
  queue_depth: 32
  concurrency: 4
  read_ratio: 0.7
  write_ratio: 0.3
  acceptance_floor:
    type: "measured_iops"
    minimum: 1500
hardware_thresholds:
  cpu_cores_min: 8
  ram_gb_min: 32
  storage_gb_min: 750
  ssd_required: true
probe_thresholds:
  finalized_lag_max_blocks: 2
  chain_lag_max_blocks: 5
reference_environment:
  id: "ref-env-2026-04"
  os_image_id: "ubuntu-24.04-lts"
  agent_version: "1.0.0"
  cpu_profile_id: "cpu-check-v1"
  disk_profile_id: "disk-check-v1"
  baseline_source_date: "2026-04-06"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	doc, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if doc.PolicyVersion != "2026-04" {
		t.Fatalf("PolicyVersion = %q, want 2026-04", doc.PolicyVersion)
	}
}

func TestLoadPolicyFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(`heartbeat_interval_seconds: 300`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want invalid policy")
	}
}

func TestLoadPolicyFailureOnInvalidProbeThresholds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	content := `policy_version: "2026-04"
heartbeat_interval_seconds: 300
cpu_profile:
  id: "cpu-check-v1"
disk_profile:
  id: "disk-check-v1"
probe_thresholds:
  finalized_lag_max_blocks: 2
  chain_lag_max_blocks: 0
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want invalid policy")
	}
}
