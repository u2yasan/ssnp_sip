package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/notifier"
	"github.com/u2yasan/ssnp_sip/portal/internal/policy"
	"github.com/u2yasan/ssnp_sip/portal/internal/store"
)

const (
	alertHeartbeatStale             = "heartbeat_stale"
	alertHeartbeatFailed            = "heartbeat_failed"
	alertNodeOutage                 = "node_outage"
	alertFinalizedLag               = "finalized_lag"
	operationalEventDeliveryFailure = "notification_delivery_failed"
	defaultEnrollmentChallengeTTL   = 10 * time.Minute
	observationWindow               = 72 * time.Hour
	healthyHeartbeatWindow          = 15 * time.Minute
	failedHeartbeatWindow           = 30 * time.Minute
)

type Config struct {
	ListenAddr              string
	PolicyPath              string
	NodesConfigPath         string
	StatePath               string
	AllowedClockSkewSeconds int
	NotificationEmailTo     string
	SMTPHost                string
	SMTPPort                int
	SMTPUsername            string
	SMTPPassword            string
	SMTPFrom                string
	HeartbeatStaleAfter     time.Duration
	HeartbeatFailedAfter    time.Duration
	AlertScanInterval       time.Duration
	EnrollmentChallengeTTL  time.Duration
	NominalDailyPool        float64
	Notifier                notifier.Notifier
}

type Server struct {
	cfg      Config
	policy   policy.Document
	store    *store.Store
	notifier notifier.Notifier
}

type publicNodeStatusView struct {
	NodeID         string `json:"node_id"`
	DateUTC        string `json:"date_utc"`
	Qualified      bool   `json:"qualified"`
	RankPosition   *int   `json:"rank_position,omitempty"`
	RewardEligible bool   `json:"reward_eligible"`
	StatusReason   string `json:"status_reason,omitempty"`
}

type operatorNodeStatusView struct {
	NodeID          string   `json:"node_id"`
	DateUTC         string   `json:"date_utc"`
	Qualified       bool     `json:"qualified"`
	RankPosition    *int     `json:"rank_position,omitempty"`
	RewardEligible  bool     `json:"reward_eligible"`
	StatusReason    string   `json:"status_reason,omitempty"`
	FailureReasons  []string `json:"failure_reasons,omitempty"`
	HeartbeatPassed bool     `json:"heartbeat_passed"`
	HardwarePassed  bool     `json:"hardware_passed"`
	VotingKeyPassed bool     `json:"voting_key_passed"`
	OperatorGroupID string   `json:"operator_group_id,omitempty"`
	ExclusionReason string   `json:"exclusion_reason,omitempty"`
}

type antiConcentrationEvidenceView struct {
	NodeID                      string `json:"node_id"`
	DateUTC                     string `json:"date_utc"`
	OperatorGroupID             string `json:"operator_group_id,omitempty"`
	RegistrableDomain           string `json:"registrable_domain,omitempty"`
	SharedControlPlaneID        string `json:"shared_control_plane_id,omitempty"`
	SharedControlClassification string `json:"shared_control_classification,omitempty"`
}

func New(cfg Config) (*Server, error) {
	doc, err := policy.Load(cfg.PolicyPath)
	if err != nil {
		return nil, err
	}
	if cfg.ListenAddr == "" {
		return nil, errors.New("missing listen address")
	}
	if cfg.NodesConfigPath == "" {
		return nil, errors.New("missing nodes config path")
	}
	if cfg.StatePath == "" {
		return nil, errors.New("missing state path")
	}
	if cfg.AllowedClockSkewSeconds <= 0 {
		cfg.AllowedClockSkewSeconds = 300
	}
	if cfg.HeartbeatStaleAfter <= 0 {
		cfg.HeartbeatStaleAfter = 15 * time.Minute
	}
	if cfg.HeartbeatFailedAfter <= 0 {
		cfg.HeartbeatFailedAfter = 30 * time.Minute
	}
	if cfg.HeartbeatFailedAfter <= cfg.HeartbeatStaleAfter {
		return nil, errors.New("heartbeat failed threshold must be greater than stale threshold")
	}
	if cfg.AlertScanInterval <= 0 {
		cfg.AlertScanInterval = time.Minute
	}
	if cfg.EnrollmentChallengeTTL <= 0 {
		cfg.EnrollmentChallengeTTL = defaultEnrollmentChallengeTTL
	}
	if cfg.Notifier == nil {
		if strings.TrimSpace(cfg.NotificationEmailTo) == "" {
			return nil, errors.New("missing fallback notification email")
		}
		if strings.TrimSpace(cfg.SMTPHost) == "" || strings.TrimSpace(cfg.SMTPUsername) == "" || strings.TrimSpace(cfg.SMTPFrom) == "" {
			return nil, errors.New("missing smtp configuration")
		}
		if cfg.SMTPPort <= 0 {
			cfg.SMTPPort = 587
		}
		if cfg.SMTPPassword == "" {
			return nil, errors.New("missing smtp password")
		}
		cfg.Notifier = notifier.SMTPNotifier{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
			Timeout:  10 * time.Second,
		}
	}
	seedNodes, err := store.LoadNodesConfig(cfg.NodesConfigPath)
	if err != nil {
		return nil, err
	}
	st, err := store.Load(seedNodes, cfg.StatePath)
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:      cfg,
		policy:   doc,
		store:    st,
		notifier: cfg.Notifier,
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/agent/policy", s.handlePolicy)
	mux.HandleFunc("/api/v1/agent/enrollment-challenges", s.handleEnrollmentChallenge)
	mux.HandleFunc("/api/v1/agent/enroll", s.handleEnroll)
	mux.HandleFunc("/api/v1/agent/heartbeat", s.handleHeartbeat)
	mux.HandleFunc("/api/v1/agent/checks", s.handleChecks)
	mux.HandleFunc("/api/v1/agent/telemetry", s.handleTelemetry)
	mux.HandleFunc("/api/v1/probes/events", s.handleProbeEvents)
	mux.HandleFunc("/api/v1/probes/daily-summaries/", s.handleDailyProbeSummary)
	mux.HandleFunc("/api/v1/voting-key-evidence", s.handleVotingKeyEvidence)
	mux.HandleFunc("/api/v1/decentralization-evidence", s.handleDecentralizationEvidence)
	mux.HandleFunc("/api/v1/domain-evidence", s.handleDomainEvidence)
	mux.HandleFunc("/api/v1/shared-control-plane-evidence", s.handleSharedControlPlaneEvidence)
	mux.HandleFunc("/api/v1/operator-group-evidence", s.handleOperatorGroupEvidence)
	mux.HandleFunc("/api/v1/rankings/", s.handleRankingRead)
	mux.HandleFunc("/api/v1/reward-eligibility/", s.handleRewardEligibilityRead)
	mux.HandleFunc("/api/v1/anti-concentration-evidence/", s.handleAntiConcentrationEvidenceRead)
	mux.HandleFunc("/api/v1/reward-allocations/", s.handleRewardAllocationRead)
	mux.HandleFunc("/api/v1/public-node-status/", s.handlePublicNodeStatusRead)
	mux.HandleFunc("/api/v1/operator-node-status/", s.handleOperatorNodeStatusRead)
	return mux
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:    s.cfg.ListenAddr,
		Handler: s.Handler(),
	}
	go s.runAlertLoop(ctx)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
