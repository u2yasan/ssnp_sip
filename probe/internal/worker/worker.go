package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/u2yasan/ssnp_sip/probe/internal/config"
	"github.com/u2yasan/ssnp_sip/probe/internal/portal"
	"github.com/u2yasan/ssnp_sip/probe/internal/symbol"
)

type Worker struct {
	cfg                 config.Config
	portalClient        *portal.Client
	symbolClient        *symbol.Client
	logger              *log.Logger
	now                 func() time.Time
	lastSuccessAt       time.Time
	consecutiveFailures int
}

func New(cfg config.Config, logger *log.Logger) *Worker {
	timeout := time.Duration(cfg.RequestTimeoutSeconds) * time.Second
	if logger == nil {
		logger = log.Default()
	}
	return &Worker{
		cfg:          cfg,
		portalClient: portal.New(cfg.PortalBaseURL, timeout),
		symbolClient: symbol.New(timeout),
		logger:       logger,
		now:          time.Now,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	ticker := time.NewTicker(time.Duration(w.cfg.PollIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		_ = w.RunOnce(ctx)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (w *Worker) RunOnce(ctx context.Context) error {
	observedAt := w.now().UTC().Truncate(time.Second)
	w.logCycleEvent("cycle_started", observedAt, "")
	sourceHeight, err := w.symbolClient.FetchSourceHeight(ctx, w.cfg.SourceEndpoint)
	if err != nil {
		return w.failCycle(observedAt, fmt.Errorf("fetch source height: %w", err))
	}

	for _, target := range w.cfg.Targets {
		payload := map[string]any{
			"schema_version":             "1",
			"probe_id":                   probeID(w.cfg.RegionID, target.NodeID, target.Endpoint, observedAt),
			"node_id":                    target.NodeID,
			"region_id":                  w.cfg.RegionID,
			"observed_at":                observedAt.Format(time.RFC3339),
			"endpoint":                   target.Endpoint,
			"measurement_window_seconds": w.cfg.PollIntervalSeconds,
		}

		available, err := w.symbolClient.IsNodeAvailable(ctx, target.Endpoint)
		if err != nil {
			payload["availability_up"] = false
			payload["error_code"] = "health_request_failed"
		} else if !available {
			payload["availability_up"] = false
		} else {
			state, err := w.symbolClient.FetchChainState(ctx, target.Endpoint)
			if err != nil {
				payload["availability_up"] = false
				payload["error_code"] = "chain_request_failed"
			} else {
				metrics, err := symbol.DeriveProbeMetrics(sourceHeight, state)
				if err != nil {
					payload["availability_up"] = false
					payload["error_code"] = "lag_derivation_failed"
				} else {
					payload["availability_up"] = true
					payload["finalized_lag_blocks"] = metrics.FinalizedLagBlocks
					payload["chain_lag_blocks"] = metrics.ChainLagBlocks
					payload["source_height"] = metrics.SourceHeight
					payload["peer_height"] = metrics.PeerHeight
				}
			}
		}

		if err := w.portalClient.SubmitProbeEvent(ctx, payload); err != nil {
			return w.failCycle(observedAt, fmt.Errorf("submit probe node_id=%s: %w", target.NodeID, err))
		}
		if payload["availability_up"] == true {
			w.logTargetEvent("probe_submitted", observedAt, target.NodeID, target.Endpoint, "")
		} else {
			w.logTargetEvent("probe_marked_down", observedAt, target.NodeID, target.Endpoint, fmt.Sprintf("%v", payload["error_code"]))
		}
	}

	w.lastSuccessAt = observedAt
	w.consecutiveFailures = 0
	w.logCycleEvent("cycle_succeeded", observedAt, "")
	return nil
}

func (w *Worker) failCycle(observedAt time.Time, err error) error {
	w.consecutiveFailures++
	w.logCycleEvent("cycle_failed", observedAt, err.Error())
	return err
}

func (w *Worker) logCycleEvent(event string, observedAt time.Time, errText string) {
	message := fmt.Sprintf(
		"event=%s region_id=%s source_endpoint=%s observed_at=%s consecutive_failures=%d last_success_at=%s",
		event,
		w.cfg.RegionID,
		w.cfg.SourceEndpoint,
		observedAt.Format(time.RFC3339),
		w.consecutiveFailures,
		formatTime(w.lastSuccessAt),
	)
	if errText != "" {
		message += " error=" + errText
	}
	w.logger.Print(message)
}

func (w *Worker) logTargetEvent(event string, observedAt time.Time, nodeID, endpoint, errText string) {
	message := fmt.Sprintf(
		"event=%s region_id=%s source_endpoint=%s observed_at=%s node_id=%s endpoint=%s consecutive_failures=%d last_success_at=%s",
		event,
		w.cfg.RegionID,
		w.cfg.SourceEndpoint,
		observedAt.Format(time.RFC3339),
		nodeID,
		endpoint,
		w.consecutiveFailures,
		formatTime(w.lastSuccessAt),
	)
	if errText != "" {
		message += " error=" + errText
	}
	w.logger.Print(message)
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "-"
	}
	return value.UTC().Format(time.RFC3339)
}

func probeID(regionID, nodeID, endpoint string, observedAt time.Time) string {
	sum := sha256.Sum256([]byte(regionID + "|" + nodeID + "|" + endpoint + "|" + observedAt.UTC().Format(time.RFC3339)))
	return "probe-" + hex.EncodeToString(sum[:8])
}
