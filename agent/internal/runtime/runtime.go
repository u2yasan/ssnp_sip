package runtime

import (
	"context"
	"crypto/ed25519"
	"math/rand"
	"time"

	"github.com/u2yasan/ssnp_sip/agent/internal/client"
	"github.com/u2yasan/ssnp_sip/agent/internal/config"
	agentcrypto "github.com/u2yasan/ssnp_sip/agent/internal/crypto"
	"github.com/u2yasan/ssnp_sip/agent/internal/logger"
	"github.com/u2yasan/ssnp_sip/agent/internal/policy"
	"github.com/u2yasan/ssnp_sip/agent/internal/state"
	"github.com/u2yasan/ssnp_sip/agent/internal/symbol"
)

const (
	warningPortalUnreachable     = "portal_unreachable"
	warningLocalCheckExecFailed  = "local_check_execution_failed"
	warningVotingKeyExpiryRisk   = "voting_key_expiry_risk"
	warningCertificateExpiryRisk = "certificate_expiry_risk"
	portalFailureThreshold       = 3
	votingKeyRiskWindow          = 14 * 24 * time.Hour
	certificateRiskWindow        = 14 * 24 * time.Hour
	maxHeartbeatAttempts         = 3
)

type Agent struct {
	cfg          config.Config
	httpClient   *client.Client
	policyClient *policy.Client
	symbolClient *symbol.Client
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
		symbolClient: symbol.NewClient(cfg.MonitoredEndpoint, timeout),
		privateKey:   privateKey,
		publicKey:    publicKey,
		fingerprint:  agentcrypto.Fingerprint(publicKey),
	}, nil
}

func NewAgentWithClients(cfg config.Config, postClient *client.Client, policyClient *policy.Client, symbolClient *symbol.Client) (*Agent, error) {
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
		symbolClient: symbolClient,
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
	jitter := time.Duration(rand.Intn(a.cfg.HeartbeatJitterSecondsMax+1)) * time.Second
	if jitter > 0 {
		time.Sleep(jitter)
	}

	ticker := time.NewTicker(time.Duration(pol.HeartbeatIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		if err := a.runRecurringChecks(ctx, pol.PolicyVersion); err != nil {
			logger.Log("error", "runtime", "recurring_checks_failed", a.cfg.NodeID, map[string]any{"error": err.Error()})
		}
		if err := a.sendHeartbeatWithRetry(ctx); err != nil {
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

func (a *Agent) runRecurringChecks(ctx context.Context, policyVersion string) error {
	if err := a.maybeEmitVotingKeyExpiryRisk(ctx, policyVersion); err != nil {
		return err
	}
	if err := a.maybeEmitCertificateExpiryRisk(ctx, policyVersion); err != nil {
		return err
	}
	return nil
}
