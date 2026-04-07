package symbol

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	http *http.Client
}

type ChainState struct {
	Height          int64
	FinalizedHeight int64
}

type ProbeMetrics struct {
	PeerHeight         int64
	SourceHeight       int64
	FinalizedLagBlocks int64
	ChainLagBlocks     int64
}

type healthResponse struct {
	Status healthStatus `json:"status"`
}

type healthStatus struct {
	Node    string `json:"node"`
	APINode string `json:"apiNode"`
	DB      string `json:"db"`
}

type chainInfoResponse struct {
	Height               any                  `json:"height"`
	LatestFinalizedBlock latestFinalizedBlock `json:"latestFinalizedBlock"`
}

type latestFinalizedBlock struct {
	Height any `json:"height"`
}

func New(timeout time.Duration) *Client {
	return &Client{
		http: &http.Client{Timeout: timeout},
	}
}

func (c *Client) FetchSourceHeight(ctx context.Context, endpoint string) (int64, error) {
	state, err := c.FetchChainState(ctx, endpoint)
	if err != nil {
		return 0, err
	}
	return state.Height, nil
}

func (c *Client) IsNodeAvailable(ctx context.Context, endpoint string) (bool, error) {
	var resp healthResponse
	if err := c.getJSON(ctx, endpoint, "/node/health", &resp); err != nil {
		return false, err
	}
	primary := strings.ToLower(strings.TrimSpace(resp.Status.APINode))
	if primary == "" {
		primary = strings.ToLower(strings.TrimSpace(resp.Status.Node))
	}
	if primary == "" {
		return false, fmt.Errorf("node health missing status")
	}
	return primary == "up", nil
}

func (c *Client) FetchChainState(ctx context.Context, endpoint string) (ChainState, error) {
	var resp chainInfoResponse
	if err := c.getJSON(ctx, endpoint, "/chain/info", &resp); err != nil {
		return ChainState{}, err
	}
	height, err := parseIntLike(resp.Height)
	if err != nil || height <= 0 {
		return ChainState{}, fmt.Errorf("invalid chain height")
	}
	finalizedHeight, err := parseIntLike(resp.LatestFinalizedBlock.Height)
	if err != nil || finalizedHeight < 0 {
		return ChainState{}, fmt.Errorf("invalid latest finalized height")
	}
	return ChainState{
		Height:          height,
		FinalizedHeight: finalizedHeight,
	}, nil
}

func DeriveProbeMetrics(sourceHeight int64, state ChainState) (ProbeMetrics, error) {
	if sourceHeight <= 0 {
		return ProbeMetrics{}, fmt.Errorf("invalid source height")
	}
	if state.Height <= 0 {
		return ProbeMetrics{}, fmt.Errorf("invalid peer height")
	}
	if state.FinalizedHeight < 0 {
		return ProbeMetrics{}, fmt.Errorf("invalid finalized height")
	}
	finalizedLag := state.Height - state.FinalizedHeight
	if finalizedLag < 0 {
		finalizedLag = 0
	}
	chainLag := sourceHeight - state.Height
	if chainLag < 0 {
		chainLag = 0
	}
	return ProbeMetrics{
		PeerHeight:         state.Height,
		SourceHeight:       sourceHeight,
		FinalizedLagBlocks: finalizedLag,
		ChainLagBlocks:     chainLag,
	}, nil
}

func (c *Client) getJSON(ctx context.Context, endpoint, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint, "/")+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("get %s failed: %s", path, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return err
	}
	return nil
}

func parseIntLike(value any) (int64, error) {
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, fmt.Errorf("empty value")
		}
		return strconv.ParseInt(trimmed, 10, 64)
	case float64:
		return int64(typed), nil
	case int:
		return int64(typed), nil
	case int64:
		return typed, nil
	case json.Number:
		return typed.Int64()
	case nil:
		return 0, fmt.Errorf("missing value")
	default:
		return 0, fmt.Errorf("unsupported integer type %T", value)
	}
}
