package policy

import (
	"fmt"
	"math"
	"os"

	"gopkg.in/yaml.v3"
)

type AcceptanceFloor struct {
	Type    string  `yaml:"type" json:"type"`
	Minimum float64 `yaml:"minimum" json:"minimum"`
}

type CPUWorkloadMix struct {
	Hashing float64 `yaml:"hashing" json:"hashing"`
	Integer float64 `yaml:"integer" json:"integer"`
	Matrix  float64 `yaml:"matrix" json:"matrix"`
}

type CPUProfile struct {
	ID              string          `yaml:"id" json:"id"`
	DurationSeconds int             `yaml:"duration_seconds" json:"duration_seconds"`
	WarmupSeconds   int             `yaml:"warmup_seconds" json:"warmup_seconds"`
	MeasuredSeconds int             `yaml:"measured_seconds" json:"measured_seconds"`
	CooldownSeconds int             `yaml:"cooldown_seconds" json:"cooldown_seconds"`
	WorkerCap       int             `yaml:"worker_cap" json:"worker_cap"`
	WorkloadMix     CPUWorkloadMix  `yaml:"workload_mix" json:"workload_mix"`
	AcceptanceFloor AcceptanceFloor `yaml:"acceptance_floor" json:"acceptance_floor"`
}

type DiskProfile struct {
	ID              string          `yaml:"id" json:"id"`
	DurationSeconds int             `yaml:"duration_seconds" json:"duration_seconds"`
	WarmupSeconds   int             `yaml:"warmup_seconds" json:"warmup_seconds"`
	MeasuredSeconds int             `yaml:"measured_seconds" json:"measured_seconds"`
	CooldownSeconds int             `yaml:"cooldown_seconds" json:"cooldown_seconds"`
	BlockSizeBytes  int             `yaml:"block_size_bytes" json:"block_size_bytes"`
	QueueDepth      int             `yaml:"queue_depth" json:"queue_depth"`
	Concurrency     int             `yaml:"concurrency" json:"concurrency"`
	ReadRatio       float64         `yaml:"read_ratio" json:"read_ratio"`
	WriteRatio      float64         `yaml:"write_ratio" json:"write_ratio"`
	AcceptanceFloor AcceptanceFloor `yaml:"acceptance_floor" json:"acceptance_floor"`
}

type HardwareThresholds struct {
	CPUCoresMin  int  `yaml:"cpu_cores_min" json:"cpu_cores_min"`
	RAMGBMin     int  `yaml:"ram_gb_min" json:"ram_gb_min"`
	StorageGBMin int  `yaml:"storage_gb_min" json:"storage_gb_min"`
	SSDRequired  bool `yaml:"ssd_required" json:"ssd_required"`
}

type ProbeThresholds struct {
	FinalizedLagMaxBlocks int `yaml:"finalized_lag_max_blocks" json:"finalized_lag_max_blocks"`
	ChainLagMaxBlocks     int `yaml:"chain_lag_max_blocks" json:"chain_lag_max_blocks"`
}

type ReferenceEnvironment struct {
	ID                 string `yaml:"id" json:"id"`
	OSImageID          string `yaml:"os_image_id" json:"os_image_id"`
	AgentVersion       string `yaml:"agent_version" json:"agent_version"`
	CPUProfileID       string `yaml:"cpu_profile_id" json:"cpu_profile_id"`
	DiskProfileID      string `yaml:"disk_profile_id" json:"disk_profile_id"`
	BaselineSourceDate string `yaml:"baseline_source_date" json:"baseline_source_date"`
}

type Document struct {
	PolicyVersion            string               `yaml:"policy_version" json:"policy_version"`
	HeartbeatIntervalSeconds int                  `yaml:"heartbeat_interval_seconds" json:"heartbeat_interval_seconds"`
	CPUProfile               CPUProfile           `yaml:"cpu_profile" json:"cpu_profile"`
	DiskProfile              DiskProfile          `yaml:"disk_profile" json:"disk_profile"`
	HardwareThresholds       HardwareThresholds   `yaml:"hardware_thresholds" json:"hardware_thresholds"`
	ProbeThresholds          ProbeThresholds      `yaml:"probe_thresholds" json:"probe_thresholds"`
	ReferenceEnvironment     ReferenceEnvironment `yaml:"reference_environment" json:"reference_environment"`
}

func Load(path string) (Document, error) {
	var doc Document
	data, err := os.ReadFile(path)
	if err != nil {
		return doc, err
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return doc, err
	}
	return doc, validate(doc)
}

func validate(doc Document) error {
	if doc.PolicyVersion == "" {
		return errInvalidPolicy("missing policy_version")
	}
	if doc.HeartbeatIntervalSeconds <= 0 {
		return errInvalidPolicy("heartbeat_interval_seconds must be positive")
	}
	if doc.CPUProfile.ID == "" || doc.DiskProfile.ID == "" {
		return errInvalidPolicy("missing profile id")
	}
	if doc.CPUProfile.DurationSeconds <= 0 || doc.CPUProfile.WarmupSeconds <= 0 || doc.CPUProfile.MeasuredSeconds <= 0 || doc.CPUProfile.CooldownSeconds <= 0 {
		return errInvalidPolicy("cpu_profile durations must be positive")
	}
	if doc.CPUProfile.WorkerCap <= 0 {
		return errInvalidPolicy("cpu_profile.worker_cap must be positive")
	}
	if !approximatelyEqual(doc.CPUProfile.WorkloadMix.Hashing+doc.CPUProfile.WorkloadMix.Integer+doc.CPUProfile.WorkloadMix.Matrix, 1.0) {
		return errInvalidPolicy("cpu_profile.workload_mix must sum to 1.0")
	}
	if doc.CPUProfile.AcceptanceFloor.Type == "" {
		return errInvalidPolicy("missing cpu_profile.acceptance_floor.type")
	}
	if doc.DiskProfile.DurationSeconds <= 0 || doc.DiskProfile.WarmupSeconds <= 0 || doc.DiskProfile.MeasuredSeconds <= 0 || doc.DiskProfile.CooldownSeconds <= 0 {
		return errInvalidPolicy("disk_profile durations must be positive")
	}
	if doc.DiskProfile.BlockSizeBytes <= 0 || doc.DiskProfile.QueueDepth <= 0 || doc.DiskProfile.Concurrency <= 0 {
		return errInvalidPolicy("disk_profile performance parameters must be positive")
	}
	if !approximatelyEqual(doc.DiskProfile.ReadRatio+doc.DiskProfile.WriteRatio, 1.0) {
		return errInvalidPolicy("disk_profile read/write ratios must sum to 1.0")
	}
	if doc.DiskProfile.AcceptanceFloor.Type == "" {
		return errInvalidPolicy("missing disk_profile.acceptance_floor.type")
	}
	if doc.HardwareThresholds.CPUCoresMin <= 0 || doc.HardwareThresholds.RAMGBMin <= 0 || doc.HardwareThresholds.StorageGBMin <= 0 {
		return errInvalidPolicy("hardware thresholds must be positive")
	}
	if doc.ProbeThresholds.FinalizedLagMaxBlocks <= 0 {
		return errInvalidPolicy("probe_thresholds.finalized_lag_max_blocks must be positive")
	}
	if doc.ProbeThresholds.ChainLagMaxBlocks <= 0 {
		return errInvalidPolicy("probe_thresholds.chain_lag_max_blocks must be positive")
	}
	if doc.ReferenceEnvironment.ID == "" || doc.ReferenceEnvironment.OSImageID == "" || doc.ReferenceEnvironment.AgentVersion == "" || doc.ReferenceEnvironment.CPUProfileID == "" || doc.ReferenceEnvironment.DiskProfileID == "" || doc.ReferenceEnvironment.BaselineSourceDate == "" {
		return errInvalidPolicy("missing reference_environment fields")
	}
	if doc.ReferenceEnvironment.CPUProfileID != doc.CPUProfile.ID || doc.ReferenceEnvironment.DiskProfileID != doc.DiskProfile.ID {
		return errInvalidPolicy("reference_environment profile ids must match active profiles")
	}
	return nil
}

func approximatelyEqual(left, right float64) bool {
	return math.Abs(left-right) <= 1e-9
}

type errInvalidPolicy string

func (e errInvalidPolicy) Error() string {
	return fmt.Sprintf("invalid policy: %s", string(e))
}
