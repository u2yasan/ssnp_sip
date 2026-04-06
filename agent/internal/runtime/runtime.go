package runtime

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/u2yasan/ssnp_sip/agent/internal/checks/cpu"
	"github.com/u2yasan/ssnp_sip/agent/internal/checks/disk"
	"github.com/u2yasan/ssnp_sip/agent/internal/checks/hardware"
	"github.com/u2yasan/ssnp_sip/agent/internal/client"
	"github.com/u2yasan/ssnp_sip/agent/internal/config"
	agentcrypto "github.com/u2yasan/ssnp_sip/agent/internal/crypto"
	"github.com/u2yasan/ssnp_sip/agent/internal/heartbeat"
	"github.com/u2yasan/ssnp_sip/agent/internal/logger"
	"github.com/u2yasan/ssnp_sip/agent/internal/policy"
	"github.com/u2yasan/ssnp_sip/agent/internal/state"
)

const (
	warningPortalUnreachable    = "portal_unreachable"
	warningLocalCheckExecFailed = "local_check_execution_failed"
	warningVotingKeyExpiryRisk  = "voting_key_expiry_risk"
	portalFailureThreshold      = 3
	votingKeyRiskWindow         = 14 * 24 * time.Hour
)

type Agent struct {
	cfg          config.Config
	httpClient   *client.Client
	policyClient *policy.Client
	privateKey   ed25519.PrivateKey
	publicKey    ed25519.PublicKey
	fingerprint  string
}

func NewAgent(cfg config.Config) (*Agent, error) {
	privateKey, err := agentcrypto.LoadPrivateKey(cfg.AgentKeyPath)
	if err != nil {
		return nil, err
	}
	publicKey, err := agentcrypto.LoadPublicKey(cfg.AgentPublicKeyPath)
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(cfg.RequestTimeoutSeconds) * time.Second
	return &Agent{
		cfg:          cfg,
		httpClient:   client.New(cfg.PortalBaseURL, timeout),
		policyClient: policy.NewClient(cfg.PortalBaseURL, timeout),
		privateKey:   privateKey,
		publicKey:    publicKey,
		fingerprint:  agentcrypto.Fingerprint(publicKey),
	}, nil
}

func NewAgentWithClients(cfg config.Config, postClient *client.Client, policyClient *policy.Client) (*Agent, error) {
	privateKey, err := agentcrypto.LoadPrivateKey(cfg.AgentKeyPath)
	if err != nil {
		return nil, err
	}
	publicKey, err := agentcrypto.LoadPublicKey(cfg.AgentPublicKeyPath)
	if err != nil {
		return nil, err
	}
	return &Agent{
		cfg:          cfg,
		httpClient:   postClient,
		policyClient: policyClient,
		privateKey:   privateKey,
		publicKey:    publicKey,
		fingerprint:  agentcrypto.Fingerprint(publicKey),
	}, nil
}

func (a *Agent) Run(ctx context.Context) error {
	pol, err := a.policyClient.Fetch(ctx, a.cfg.NodeID, a.fingerprint)
	if err != nil {
		return err
	}
	st, err := state.Load(a.cfg.StatePath)
	if err != nil {
		return err
	}
	st.AgentKeyFingerprint = a.fingerprint
	st.LastPolicyVersion = pol.PolicyVersion
	if err := state.Save(a.cfg.StatePath, st); err != nil {
		return err
	}
	if err := a.maybeEmitVotingKeyExpiryRisk(ctx, pol.PolicyVersion); err != nil {
		logger.Log("error", "runtime", "telemetry_submit_failed", a.cfg.NodeID, map[string]any{"error": err.Error(), "warning_flag": warningVotingKeyExpiryRisk})
	}

	jitter := time.Duration(rand.Intn(a.cfg.HeartbeatJitterSecondsMax+1)) * time.Second
	if jitter > 0 {
		time.Sleep(jitter)
	}

	ticker := time.NewTicker(time.Duration(pol.HeartbeatIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		if err := a.sendHeartbeat(ctx); err != nil {
			if stateErr := a.recordPortalFailure(); stateErr != nil {
				logger.Log("error", "runtime", "state_save_failed", a.cfg.NodeID, map[string]any{"error": stateErr.Error()})
			}
			logger.Log("error", "runtime", "heartbeat_failed", a.cfg.NodeID, map[string]any{"error": err.Error()})
		} else {
			if err := a.handlePortalRecovery(ctx, pol.PolicyVersion); err != nil {
				logger.Log("error", "runtime", "warning_flush_failed", a.cfg.NodeID, map[string]any{"error": err.Error()})
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

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
		"checked_at":                time.Now().UTC().Format(time.RFC3339),
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

func (a *Agent) SubmitTelemetry(ctx context.Context, warningFlags []string) error {
	if len(warningFlags) == 0 {
		return errors.New("missing warning flags")
	}
	pol, err := a.policyClient.Fetch(ctx, a.cfg.NodeID, a.fingerprint)
	if err != nil {
		return err
	}

	return a.submitTelemetryWithPolicy(ctx, pol.PolicyVersion, warningFlags)
}

func (a *Agent) submitTelemetryWithPolicy(ctx context.Context, policyVersion string, warningFlags []string) error {
	payload := map[string]any{
		"schema_version":        "1",
		"node_id":               a.cfg.NodeID,
		"agent_key_fingerprint": a.fingerprint,
		"telemetry_timestamp":   time.Now().UTC().Format(time.RFC3339),
		"policy_version":        policyVersion,
		"warning_flags":         warningFlags,
	}
	sig, err := signMap(a.privateKey, payload)
	if err != nil {
		return err
	}
	payload["signature"] = sig

	if err := a.httpClient.PostJSON(ctx, "/api/v1/agent/telemetry", payload); err != nil {
		return err
	}

	enc := json.NewEncoder(os.Stdout)
	return enc.Encode(map[string]any{
		"policy_version": policyVersion,
		"warning_flags":  warningFlags,
		"submitted":      true,
	})
}

func (a *Agent) sendHeartbeat(ctx context.Context) error {
	st, err := state.Load(a.cfg.StatePath)
	if err != nil {
		return err
	}
	if st.AgentKeyFingerprint != "" && st.AgentKeyFingerprint != a.fingerprint {
		return errors.New("state fingerprint mismatch")
	}

	payload := heartbeat.New(
		a.cfg.NodeID,
		a.fingerprint,
		a.cfg.AgentVersion,
		a.cfg.EnrollmentGeneration,
		st.SequenceNumber+1,
		[]string{},
	)
	canonical, err := payload.CanonicalBytes()
	if err != nil {
		return err
	}
	payload.Signature = agentcrypto.Sign(a.privateKey, canonical)

	if err := a.httpClient.PostJSON(ctx, "/api/v1/agent/heartbeat", payload); err != nil {
		return err
	}

	st.SequenceNumber++
	st.AgentKeyFingerprint = a.fingerprint
	return state.Save(a.cfg.StatePath, st)
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

func (a *Agent) recordPortalFailure() error {
	st, err := state.Load(a.cfg.StatePath)
	if err != nil {
		return err
	}
	ensureWarningMaps(&st)
	st.ConsecutivePortalFailures++
	if st.ConsecutivePortalFailures >= portalFailureThreshold && !st.ActiveWarnings[warningPortalUnreachable] {
		st.PendingWarnings[warningPortalUnreachable] = true
	}
	return state.Save(a.cfg.StatePath, st)
}

func (a *Agent) handlePortalRecovery(ctx context.Context, policyVersion string) error {
	st, err := state.Load(a.cfg.StatePath)
	if err != nil {
		return err
	}
	ensureWarningMaps(&st)
	st.ConsecutivePortalFailures = 0
	if st.PendingWarnings[warningPortalUnreachable] && !st.ActiveWarnings[warningPortalUnreachable] {
		if err := a.submitTelemetryWithPolicy(ctx, policyVersion, []string{warningPortalUnreachable}); err != nil {
			return err
		}
		st.ActiveWarnings[warningPortalUnreachable] = true
		delete(st.PendingWarnings, warningPortalUnreachable)
	}
	return state.Save(a.cfg.StatePath, st)
}

func (a *Agent) markWarning(ctx context.Context, policyVersion, warningFlag string) error {
	st, err := state.Load(a.cfg.StatePath)
	if err != nil {
		return err
	}
	ensureWarningMaps(&st)
	if st.ActiveWarnings[warningFlag] || st.PendingWarnings[warningFlag] {
		return nil
	}
	if err := a.submitTelemetryWithPolicy(ctx, policyVersion, []string{warningFlag}); err != nil {
		st.PendingWarnings[warningFlag] = true
		_ = state.Save(a.cfg.StatePath, st)
		return err
	}
	st.ActiveWarnings[warningFlag] = true
	return state.Save(a.cfg.StatePath, st)
}

func ensureWarningMaps(st *state.State) {
	if st.ActiveWarnings == nil {
		st.ActiveWarnings = map[string]bool{}
	}
	if st.PendingWarnings == nil {
		st.PendingWarnings = map[string]bool{}
	}
}

func localCheckExecutionFailed(hw hardware.Result, cpuResult cpu.Result, diskResult disk.Result) bool {
	return hw.VisibleCPUThreads == 0 ||
		hw.VisibleMemoryBytes == 0 ||
		hw.VisibleStorageBytes == 0 ||
		(cpuResult.NormalizedScore == 0 && !cpuResult.Passed) ||
		(diskResult.MeasuredIOPS == 0 && !diskResult.Passed)
}

func (a *Agent) maybeEmitVotingKeyExpiryRisk(ctx context.Context, policyVersion string) error {
	expiryRaw := strings.TrimSpace(a.cfg.VotingKeyExpiryAt)
	if expiryRaw == "" {
		return nil
	}
	expiryAt, err := time.Parse(time.RFC3339, expiryRaw)
	if err != nil {
		return fmt.Errorf("invalid voting_key_expiry_at: %w", err)
	}
	if time.Until(expiryAt) >= votingKeyRiskWindow {
		return nil
	}
	return a.markWarning(ctx, policyVersion, warningVotingKeyExpiryRisk)
}
