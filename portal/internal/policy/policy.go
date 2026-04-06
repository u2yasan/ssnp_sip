package policy

import (
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
	return nil
}

type errInvalidPolicy string

func (e errInvalidPolicy) Error() string {
	return string(e)
}
