package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AcceptanceFloor struct {
	Type    string  `json:"type"`
	Minimum float64 `json:"minimum"`
}

type CPUWorkloadMix struct {
	Hashing float64 `json:"hashing"`
	Integer float64 `json:"integer"`
	Matrix  float64 `json:"matrix"`
}

type CPUProfile struct {
	ID              string          `json:"id"`
	DurationSeconds int             `json:"duration_seconds"`
	WarmupSeconds   int             `json:"warmup_seconds"`
	MeasuredSeconds int             `json:"measured_seconds"`
	CooldownSeconds int             `json:"cooldown_seconds"`
	WorkerCap       int             `json:"worker_cap"`
	WorkloadMix     CPUWorkloadMix  `json:"workload_mix"`
	AcceptanceFloor AcceptanceFloor `json:"acceptance_floor"`
}

type DiskProfile struct {
	ID              string          `json:"id"`
	DurationSeconds int             `json:"duration_seconds"`
	WarmupSeconds   int             `json:"warmup_seconds"`
	MeasuredSeconds int             `json:"measured_seconds"`
	CooldownSeconds int             `json:"cooldown_seconds"`
	BlockSizeBytes  int             `json:"block_size_bytes"`
	QueueDepth      int             `json:"queue_depth"`
	Concurrency     int             `json:"concurrency"`
	ReadRatio       float64         `json:"read_ratio"`
	WriteRatio      float64         `json:"write_ratio"`
	AcceptanceFloor AcceptanceFloor `json:"acceptance_floor"`
}

type HardwareThresholds struct {
	CPUCoresMin  int  `json:"cpu_cores_min"`
	RAMGBMin     int  `json:"ram_gb_min"`
	StorageGBMin int  `json:"storage_gb_min"`
	SSDRequired  bool `json:"ssd_required"`
}

type ProbeThresholds struct {
	FinalizedLagMaxBlocks int `json:"finalized_lag_max_blocks"`
	ChainLagMaxBlocks     int `json:"chain_lag_max_blocks"`
}

type ReferenceEnvironment struct {
	ID                 string `json:"id"`
	OSImageID          string `json:"os_image_id"`
	AgentVersion       string `json:"agent_version"`
	CPUProfileID       string `json:"cpu_profile_id"`
	DiskProfileID      string `json:"disk_profile_id"`
	BaselineSourceDate string `json:"baseline_source_date"`
}

type Response struct {
	PolicyVersion            string               `json:"policy_version"`
	HeartbeatIntervalSeconds int                  `json:"heartbeat_interval_seconds"`
	CPUProfile               CPUProfile           `json:"cpu_profile"`
	DiskProfile              DiskProfile          `json:"disk_profile"`
	HardwareThresholds       HardwareThresholds   `json:"hardware_thresholds"`
	ProbeThresholds          ProbeThresholds      `json:"probe_thresholds"`
	ReferenceEnvironment     ReferenceEnvironment `json:"reference_environment"`
}

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

func NewClientWithHTTP(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    httpClient,
	}
}

func (c *Client) Fetch(ctx context.Context, nodeID, fingerprint string) (Response, error) {
	var out Response
	endpoint := c.baseURL + "/api/v1/agent/policy"
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return out, err
	}
	q := reqURL.Query()
	q.Set("node_id", nodeID)
	q.Set("agent_key_fingerprint", fingerprint)
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return out, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("policy fetch failed: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	if err := validate(out); err != nil {
		return out, err
	}
	return out, nil
}

func validate(doc Response) error {
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
	if doc.DiskProfile.DurationSeconds <= 0 || doc.DiskProfile.WarmupSeconds <= 0 || doc.DiskProfile.MeasuredSeconds <= 0 || doc.DiskProfile.CooldownSeconds <= 0 {
		return errInvalidPolicy("disk_profile durations must be positive")
	}
	if doc.DiskProfile.BlockSizeBytes <= 0 || doc.DiskProfile.QueueDepth <= 0 || doc.DiskProfile.Concurrency <= 0 {
		return errInvalidPolicy("disk_profile performance parameters must be positive")
	}
	if !approximatelyEqual(doc.DiskProfile.ReadRatio+doc.DiskProfile.WriteRatio, 1.0) {
		return errInvalidPolicy("disk_profile read/write ratios must sum to 1.0")
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
	return string(e)
}
