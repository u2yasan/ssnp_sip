package symbol

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type VotingKeyStatus struct {
	NearExpiry bool
}

type nodeInfoResponse struct {
	PublicKey string           `json:"publicKey"`
	Node      nodeInfoEmbedded `json:"node"`
}

type nodeInfoEmbedded struct {
	PublicKey string `json:"publicKey"`
}

type chainInfoResponse struct {
	Height string `json:"height"`
}

type networkPropertiesResponse struct {
	Chain   chainProperties      `json:"chain"`
	Network networkPropertiesTop `json:"network"`
}

type networkPropertiesTop struct {
	Chain chainProperties `json:"chain"`
}

type chainProperties struct {
	VotingSetGrouping         string `json:"votingSetGrouping"`
	BlockGenerationTargetTime string `json:"blockGenerationTargetTime"`
}

type accountInfoResponse struct {
	Account accountInfo `json:"account"`
}

type accountInfo struct {
	SupplementalPublicKeys supplementalPublicKeys `json:"supplementalPublicKeys"`
}

type supplementalPublicKeys struct {
	Voting votingPublicKeys `json:"voting"`
}

type votingPublicKeys struct {
	PublicKeys []votingKey `json:"publicKeys"`
}

type votingKey struct {
	PublicKey  string `json:"publicKey"`
	StartEpoch any    `json:"startEpoch"`
	EndEpoch   any    `json:"endEpoch"`
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func NewClientWithHTTP(baseURL string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

func (c *Client) HasVotingKeyExpiryRisk(ctx context.Context, riskWindow time.Duration) (VotingKeyStatus, error) {
	publicKey, err := c.fetchNodePublicKey(ctx)
	if err != nil {
		return VotingKeyStatus{}, err
	}

	height, err := c.fetchChainHeight(ctx)
	if err != nil {
		return VotingKeyStatus{}, err
	}

	grouping, blockTargetTime, err := c.fetchEpochProperties(ctx)
	if err != nil {
		return VotingKeyStatus{}, err
	}

	endEpoch, ok, err := c.fetchEarliestActiveVotingEndEpoch(ctx, publicKey, currentVotingEpoch(height, grouping))
	if err != nil {
		return VotingKeyStatus{}, err
	}
	if !ok {
		return VotingKeyStatus{}, nil
	}

	currentEpoch := currentVotingEpoch(height, grouping)
	remainingEpochs := endEpoch - currentEpoch
	if remainingEpochs < 0 {
		remainingEpochs = 0
	}
	epochDuration := time.Duration(grouping) * blockTargetTime
	if epochDuration <= 0 {
		return VotingKeyStatus{}, fmt.Errorf("invalid epoch duration")
	}

	return VotingKeyStatus{
		NearExpiry: time.Duration(remainingEpochs)*epochDuration < riskWindow,
	}, nil
}

func (c *Client) fetchNodePublicKey(ctx context.Context) (string, error) {
	var resp nodeInfoResponse
	if err := c.getJSON(ctx, "/node/info", &resp); err != nil {
		return "", err
	}
	publicKey := strings.TrimSpace(resp.PublicKey)
	if publicKey == "" {
		publicKey = strings.TrimSpace(resp.Node.PublicKey)
	}
	if publicKey == "" {
		return "", fmt.Errorf("node info missing publicKey")
	}
	return publicKey, nil
}

func (c *Client) fetchChainHeight(ctx context.Context) (int64, error) {
	var resp chainInfoResponse
	if err := c.getJSON(ctx, "/chain/info", &resp); err != nil {
		return 0, err
	}
	height, err := parseIntString(resp.Height)
	if err != nil {
		return 0, fmt.Errorf("invalid chain height: %w", err)
	}
	return height, nil
}

func (c *Client) fetchEpochProperties(ctx context.Context) (int64, time.Duration, error) {
	var resp networkPropertiesResponse
	if err := c.getJSON(ctx, "/network/properties", &resp); err != nil {
		return 0, 0, err
	}
	props := resp.Chain
	if props.VotingSetGrouping == "" {
		props = resp.Network.Chain
	}
	grouping, err := parseIntString(props.VotingSetGrouping)
	if err != nil || grouping <= 0 {
		return 0, 0, fmt.Errorf("invalid votingSetGrouping")
	}
	blockTargetTime, err := time.ParseDuration(strings.TrimSpace(props.BlockGenerationTargetTime))
	if err != nil || blockTargetTime <= 0 {
		return 0, 0, fmt.Errorf("invalid blockGenerationTargetTime")
	}
	return grouping, blockTargetTime, nil
}

func (c *Client) fetchEarliestActiveVotingEndEpoch(ctx context.Context, publicKey string, currentEpoch int64) (int64, bool, error) {
	var resp accountInfoResponse
	if err := c.getJSON(ctx, "/accounts/"+url.PathEscape(publicKey), &resp); err != nil {
		return 0, false, err
	}
	earliest := int64(0)
	found := false
	for _, key := range resp.Account.SupplementalPublicKeys.Voting.PublicKeys {
		startEpoch, err := parseIntLike(key.StartEpoch)
		if err != nil {
			return 0, false, fmt.Errorf("invalid voting startEpoch: %w", err)
		}
		endEpoch, err := parseIntLike(key.EndEpoch)
		if err != nil {
			return 0, false, fmt.Errorf("invalid voting endEpoch: %w", err)
		}
		if currentEpoch < startEpoch || currentEpoch > endEpoch {
			continue
		}
		if !found || endEpoch < earliest {
			earliest = endEpoch
			found = true
		}
	}
	return earliest, found, nil
}

func currentVotingEpoch(height, grouping int64) int64 {
	if height <= 0 || grouping <= 0 {
		return 0
	}
	return ((height - 1) / grouping) + 1
}

func parseIntString(value string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, fmt.Errorf("empty value")
	}
	return strconv.ParseInt(trimmed, 10, 64)
}

func parseIntLike(value any) (int64, error) {
	switch typed := value.(type) {
	case string:
		return parseIntString(typed)
	case float64:
		return int64(typed), nil
	case int:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case int64:
		return typed, nil
	case json.Number:
		return typed.Int64()
	default:
		return 0, fmt.Errorf("unsupported integer type %T", value)
	}
}

func (c *Client) getJSON(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("symbol api %s returned %s", path, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return err
	}
	return nil
}
