package symbol

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestHasVotingKeyExpiryRiskNearExpiry(t *testing.T) {
	client := NewClientWithHTTP("http://symbol.node", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/node/info":
				return jsonResponse(http.StatusOK, `{"publicKey":"NODE_MAIN_PUBLIC_KEY"}`)
			case "/chain/info":
				return jsonResponse(http.StatusOK, `{"height":"40320"}`)
			case "/network/properties":
				return jsonResponse(http.StatusOK, `{"network":{"chain":{"votingSetGrouping":"1440","blockGenerationTargetTime":"30s"}}}`)
			case "/accounts/NODE_MAIN_PUBLIC_KEY":
				return jsonResponse(http.StatusOK, `{
					"account":{
						"supplementalPublicKeys":{
							"voting":{
								"publicKeys":[
									{"publicKey":"VOTE_A","startEpoch":"1","endEpoch":"34"},
									{"publicKey":"VOTE_B","startEpoch":"35","endEpoch":"60"}
								]
							}
						}
					}
				}`)
			default:
				return jsonResponse(http.StatusNotFound, `{"status":"error"}`)
			}
		}),
	})

	status, err := client.HasVotingKeyExpiryRisk(context.Background(), 14*24*time.Hour)
	if err != nil {
		t.Fatalf("HasVotingKeyExpiryRisk() error = %v", err)
	}
	if !status.NearExpiry {
		t.Fatal("NearExpiry = false, want true")
	}
}

func TestHasVotingKeyExpiryRiskNoRiskWhenOutsideWindow(t *testing.T) {
	client := NewClientWithHTTP("http://symbol.node", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/node/info":
				return jsonResponse(http.StatusOK, `{"publicKey":"NODE_MAIN_PUBLIC_KEY"}`)
			case "/chain/info":
				return jsonResponse(http.StatusOK, `{"height":"1440"}`)
			case "/network/properties":
				return jsonResponse(http.StatusOK, `{"chain":{"votingSetGrouping":"1440","blockGenerationTargetTime":"30s"}}`)
			case "/accounts/NODE_MAIN_PUBLIC_KEY":
				return jsonResponse(http.StatusOK, `{
					"account":{
						"supplementalPublicKeys":{
							"voting":{
								"publicKeys":[
									{"publicKey":"VOTE_A","startEpoch":"1","endEpoch":"90"}
								]
							}
						}
					}
				}`)
			default:
				return jsonResponse(http.StatusNotFound, `{"status":"error"}`)
			}
		}),
	})

	status, err := client.HasVotingKeyExpiryRisk(context.Background(), 14*24*time.Hour)
	if err != nil {
		t.Fatalf("HasVotingKeyExpiryRisk() error = %v", err)
	}
	if status.NearExpiry {
		t.Fatal("NearExpiry = true, want false")
	}
}

func TestHasVotingKeyExpiryRiskNoOpWhenVotingKeysEmpty(t *testing.T) {
	client := NewClientWithHTTP("http://symbol.node", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/node/info":
				return jsonResponse(http.StatusOK, `{"publicKey":"NODE_MAIN_PUBLIC_KEY"}`)
			case "/chain/info":
				return jsonResponse(http.StatusOK, `{"height":"1440"}`)
			case "/network/properties":
				return jsonResponse(http.StatusOK, `{"network":{"chain":{"votingSetGrouping":"1440","blockGenerationTargetTime":"30s"}}}`)
			case "/accounts/NODE_MAIN_PUBLIC_KEY":
				return jsonResponse(http.StatusOK, `{"account":{"supplementalPublicKeys":{"voting":{"publicKeys":[]}}}}`)
			default:
				return jsonResponse(http.StatusNotFound, `{"status":"error"}`)
			}
		}),
	})

	status, err := client.HasVotingKeyExpiryRisk(context.Background(), 14*24*time.Hour)
	if err != nil {
		t.Fatalf("HasVotingKeyExpiryRisk() error = %v", err)
	}
	if status.NearExpiry {
		t.Fatal("NearExpiry = true, want false")
	}
}

func TestHasVotingKeyExpiryRiskFailsOnMalformedJSON(t *testing.T) {
	client := NewClientWithHTTP("http://symbol.node", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/node/info" {
				return jsonResponse(http.StatusOK, `{"publicKey":`)
			}
			return jsonResponse(http.StatusNotFound, `{"status":"error"}`)
		}),
	})

	if _, err := client.HasVotingKeyExpiryRisk(context.Background(), 14*24*time.Hour); err == nil {
		t.Fatal("HasVotingKeyExpiryRisk() error = nil, want malformed JSON failure")
	}
}

func TestHasVotingKeyExpiryRiskFailsOnHTTPError(t *testing.T) {
	client := NewClientWithHTTP("http://symbol.node", &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path == "/node/info" {
				return jsonResponse(http.StatusServiceUnavailable, `{"status":"error"}`)
			}
			return jsonResponse(http.StatusNotFound, `{"status":"error"}`)
		}),
	})

	if _, err := client.HasVotingKeyExpiryRisk(context.Background(), 14*24*time.Hour); err == nil {
		t.Fatal("HasVotingKeyExpiryRisk() error = nil, want HTTP error")
	}
}

func TestCurrentVotingEpoch(t *testing.T) {
	tests := []struct {
		name     string
		height   int64
		grouping int64
		want     int64
	}{
		{name: "first block", height: 1, grouping: 1440, want: 1},
		{name: "epoch boundary", height: 1440, grouping: 1440, want: 1},
		{name: "next epoch", height: 1441, grouping: 1440, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := currentVotingEpoch(tt.height, tt.grouping); got != tt.want {
				t.Fatalf("currentVotingEpoch() = %d, want %d", got, tt.want)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}
