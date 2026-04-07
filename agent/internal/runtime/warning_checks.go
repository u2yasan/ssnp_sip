package runtime

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/u2yasan/ssnp_sip/agent/internal/checks/cpu"
	"github.com/u2yasan/ssnp_sip/agent/internal/checks/disk"
	"github.com/u2yasan/ssnp_sip/agent/internal/checks/hardware"
	"github.com/u2yasan/ssnp_sip/agent/internal/logger"
	"github.com/u2yasan/ssnp_sip/agent/internal/state"
)

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
		"telemetry_timestamp":   timeNowRFC3339(),
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
	status, err := a.symbolClient.HasVotingKeyExpiryRisk(ctx, votingKeyRiskWindow)
	if err != nil {
		logger.Log("warn", "runtime", "voting_key_expiry_check_skipped", a.cfg.NodeID, map[string]any{"error": err.Error()})
		return nil
	}
	if !status.NearExpiry {
		return nil
	}
	return a.markWarning(ctx, policyVersion, warningVotingKeyExpiryRisk)
}

func (a *Agent) collectLocalObservationFlags(ctx context.Context) []string {
	flags := []string{}
	status, err := a.symbolClient.HasVotingKeyExpiryRisk(ctx, votingKeyRiskWindow)
	switch {
	case err != nil:
		flags = append(flags, "local_api_unreachable")
	case status.NearExpiry:
		flags = append(flags, warningVotingKeyExpiryRisk)
	}

	endpoint := strings.TrimSpace(a.cfg.MonitoredEndpoint)
	if endpoint != "" {
		parsed, err := url.Parse(endpoint)
		if err == nil && parsed.Scheme == "https" {
			notAfter, ok := fetchLeafCertificateNotAfter(parsed)
			if ok && time.Until(notAfter) < certificateRiskWindow {
				flags = append(flags, warningCertificateExpiryRisk)
			}
		}
	}

	st, err := state.Load(a.cfg.StatePath)
	if err == nil {
		if st.PendingWarnings[warningPortalUnreachable] || st.ActiveWarnings[warningPortalUnreachable] {
			flags = append(flags, warningPortalUnreachable)
		}
	}
	return flags
}

func (a *Agent) maybeEmitCertificateExpiryRisk(ctx context.Context, policyVersion string) error {
	endpoint := strings.TrimSpace(a.cfg.MonitoredEndpoint)
	if endpoint == "" {
		return nil
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid monitored_endpoint: %w", err)
	}
	if parsed.Scheme != "https" {
		return nil
	}
	notAfter, ok := fetchLeafCertificateNotAfter(parsed)
	if !ok {
		return nil
	}
	if time.Until(notAfter) >= certificateRiskWindow {
		return nil
	}
	return a.markWarning(ctx, policyVersion, warningCertificateExpiryRisk)
}

func fetchLeafCertificateNotAfter(endpoint *url.URL) (time.Time, bool) {
	host := endpoint.Hostname()
	if host == "" {
		return time.Time{}, false
	}
	port := endpoint.Port()
	if port == "" {
		port = "443"
	}
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", net.JoinHostPort(host, port), &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // v0.1 only checks expiry metadata
		ServerName:         host,
	})
	if err != nil {
		return time.Time{}, false
	}
	defer conn.Close()
	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return time.Time{}, false
	}
	return state.PeerCertificates[0].NotAfter, true
}
