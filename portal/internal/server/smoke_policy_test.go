package server

import (
	"testing"

	policydoc "github.com/u2yasan/ssnp_sip/portal/internal/policy"
)

func TestSmokePolicyUsesExpectedFastPathSettings(t *testing.T) {
	doc, err := policydoc.Load(smokePolicyPath())
	if err != nil {
		t.Fatalf("policy.Load() error = %v", err)
	}

	if doc.PolicyVersion != "2026-04-smoke" {
		t.Fatalf("PolicyVersion = %q, want %q", doc.PolicyVersion, "2026-04-smoke")
	}
	if doc.HeartbeatIntervalSeconds != 1 {
		t.Fatalf("HeartbeatIntervalSeconds = %d, want 1", doc.HeartbeatIntervalSeconds)
	}
	if doc.CPUProfile.ID != "cpu-check-v1" {
		t.Fatalf("CPUProfile.ID = %q, want %q", doc.CPUProfile.ID, "cpu-check-v1")
	}
	if doc.CPUProfile.DurationSeconds != 3 || doc.CPUProfile.WarmupSeconds != 1 || doc.CPUProfile.MeasuredSeconds != 1 || doc.CPUProfile.CooldownSeconds != 1 {
		t.Fatalf("CPUProfile durations = %#v, want 3/1/1/1", doc.CPUProfile)
	}
	if doc.CPUProfile.AcceptanceFloor.Minimum != 0 {
		t.Fatalf("CPUProfile.AcceptanceFloor.Minimum = %v, want 0", doc.CPUProfile.AcceptanceFloor.Minimum)
	}
	if doc.DiskProfile.ID != "disk-check-v1" {
		t.Fatalf("DiskProfile.ID = %q, want %q", doc.DiskProfile.ID, "disk-check-v1")
	}
	if doc.DiskProfile.DurationSeconds != 3 || doc.DiskProfile.WarmupSeconds != 1 || doc.DiskProfile.MeasuredSeconds != 1 || doc.DiskProfile.CooldownSeconds != 1 {
		t.Fatalf("DiskProfile durations = %#v, want 3/1/1/1", doc.DiskProfile)
	}
	if doc.DiskProfile.Concurrency != 1 {
		t.Fatalf("DiskProfile.Concurrency = %d, want 1", doc.DiskProfile.Concurrency)
	}
	if doc.DiskProfile.AcceptanceFloor.Minimum != 0 {
		t.Fatalf("DiskProfile.AcceptanceFloor.Minimum = %v, want 0", doc.DiskProfile.AcceptanceFloor.Minimum)
	}
	if doc.HardwareThresholds.CPUCoresMin != 1 || doc.HardwareThresholds.RAMGBMin != 1 || doc.HardwareThresholds.StorageGBMin != 1 || doc.HardwareThresholds.SSDRequired {
		t.Fatalf("HardwareThresholds = %#v, want 1/1/1 and ssd_required=false", doc.HardwareThresholds)
	}
	if doc.ProbeThresholds.FinalizedLagMaxBlocks != 2 || doc.ProbeThresholds.ChainLagMaxBlocks != 5 {
		t.Fatalf("ProbeThresholds = %#v, want finalized=2 chain=5", doc.ProbeThresholds)
	}
	if doc.ReferenceEnvironment.ID != "ref-env-2026-04-smoke" {
		t.Fatalf("ReferenceEnvironment.ID = %q, want %q", doc.ReferenceEnvironment.ID, "ref-env-2026-04-smoke")
	}
}
