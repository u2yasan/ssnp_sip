package runtime

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/u2yasan/ssnp_sip/agent/internal/checks/cpu"
	"github.com/u2yasan/ssnp_sip/agent/internal/checks/disk"
	"github.com/u2yasan/ssnp_sip/agent/internal/checks/hardware"
	agentcrypto "github.com/u2yasan/ssnp_sip/agent/internal/crypto"
	"github.com/u2yasan/ssnp_sip/agent/internal/logger"
)

func (a *Agent) Enroll(ctx context.Context, challengeID string) error {
	payload := map[string]any{
		"node_id":                 a.cfg.NodeID,
		"enrollment_challenge_id": challengeID,
		"agent_public_key":        hex.EncodeToString(a.publicKey),
		"agent_version":           a.cfg.AgentVersion,
	}
	sig, err := signMap(a.privateKey, payload)
	if err != nil {
		return err
	}
	payload["signature"] = sig
	return a.httpClient.PostJSON(ctx, "/api/v1/agent/enroll", payload)
}

func (a *Agent) RunChecks(ctx context.Context, eventType, eventID string) error {
	if eventType != "registration" && eventType != "voting_key_renewal" && eventType != "recheck" {
		return fmt.Errorf("invalid event type: %s", eventType)
	}
	pol, err := a.policyClient.Fetch(ctx, a.cfg.NodeID, a.fingerprint)
	if err != nil {
		return err
	}

	hw := hardware.Run(a.cfg.TempDir, pol.HardwareThresholds)
	cpuResult := cpu.Run(ctx, pol.CPUProfile)
	diskResult := disk.Run(ctx, a.cfg.TempDir, pol.DiskProfile)
	if localCheckExecutionFailed(hw, cpuResult, diskResult) {
		if err := a.markWarning(ctx, pol.PolicyVersion, warningLocalCheckExecFailed); err != nil {
			logger.Log("error", "runtime", "telemetry_submit_failed", a.cfg.NodeID, map[string]any{"error": err.Error(), "warning_flag": warningLocalCheckExecFailed})
		}
	}
	overall := hw.CPUCheckPassed &&
		hw.RAMCheckPassed &&
		hw.StorageSizeCheckPassed &&
		hw.SSDCheckPassed &&
		cpuResult.Passed &&
		diskResult.Passed

	payload := map[string]any{
		"schema_version":            "1",
		"node_id":                   a.cfg.NodeID,
		"agent_key_fingerprint":     a.fingerprint,
		"event_type":                eventType,
		"event_id":                  eventID,
		"policy_version":            pol.PolicyVersion,
		"cpu_profile_id":            pol.CPUProfile.ID,
		"disk_profile_id":           pol.DiskProfile.ID,
		"checked_at":                timeNowRFC3339(),
		"cpu_check_passed":          hw.CPUCheckPassed,
		"disk_check_passed":         diskResult.Passed,
		"ram_check_passed":          hw.RAMCheckPassed,
		"storage_size_check_passed": hw.StorageSizeCheckPassed,
		"ssd_check_passed":          hw.SSDCheckPassed,
		"cpu_load_test_passed":      cpuResult.Passed,
		"overall_passed":            overall,
		"agent_version":             a.cfg.AgentVersion,
		"normalized_cpu_score":      cpuResult.NormalizedScore,
		"measured_iops":             diskResult.MeasuredIOPS,
		"measured_latency_p95":      diskResult.MeasuredLatencyP95,
		"visible_cpu_threads":       hw.VisibleCPUThreads,
		"visible_memory_bytes":      hw.VisibleMemoryBytes,
		"visible_storage_bytes":     hw.VisibleStorageBytes,
	}
	sig, err := signMap(a.privateKey, payload)
	if err != nil {
		return err
	}
	payload["signature"] = sig

	if err := a.httpClient.PostJSON(ctx, "/api/v1/agent/checks", payload); err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(map[string]any{
		"event_id":          eventID,
		"policy_version":    pol.PolicyVersion,
		"cpu_check_passed":  hw.CPUCheckPassed,
		"disk_check_passed": diskResult.Passed,
		"overall_passed":    overall,
		"submitted":         true,
	})
}

func signMap(privateKey ed25519.PrivateKey, payload map[string]any) (string, error) {
	copyMap := map[string]any{}
	for k, v := range payload {
		copyMap[k] = v
	}
	delete(copyMap, "signature")
	data, err := json.Marshal(copyMap)
	if err != nil {
		return "", err
	}
	return agentcrypto.Sign(privateKey, data), nil
}
