package runtime

import (
	"context"
	"errors"
	"time"

	agentcrypto "github.com/u2yasan/ssnp_sip/agent/internal/crypto"
	"github.com/u2yasan/ssnp_sip/agent/internal/heartbeat"
	"github.com/u2yasan/ssnp_sip/agent/internal/state"
)

func (a *Agent) sendHeartbeat(ctx context.Context) error {
	st, err := state.Load(a.cfg.StatePath)
	if err != nil {
		return err
	}
	if st.AgentKeyFingerprint != "" && st.AgentKeyFingerprint != a.fingerprint {
		return errors.New("state fingerprint mismatch")
	}

	flags := a.collectLocalObservationFlags(ctx)
	payload := heartbeat.New(
		a.cfg.NodeID,
		a.fingerprint,
		a.cfg.AgentVersion,
		a.cfg.EnrollmentGeneration,
		st.SequenceNumber+1,
		flags,
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

func (a *Agent) sendHeartbeatWithRetry(ctx context.Context) error {
	var lastErr error
	backoff := time.Second
	for attempt := 1; attempt <= maxHeartbeatAttempts; attempt++ {
		if err := a.sendHeartbeat(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt == maxHeartbeatAttempts {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 4*time.Second {
			backoff *= 2
		}
	}
	return lastErr
}

func timeNowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
