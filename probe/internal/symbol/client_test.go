package symbol

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchChainStateAndAvailability(t *testing.T) {
	server := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/node/health":
			_, _ = w.Write([]byte(`{"status":{"apiNode":"up","node":"up"}}`))
		case "/chain/info":
			_, _ = w.Write([]byte(`{"height":"120","latestFinalizedBlock":{"height":"118"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := New(2 * time.Second)
	available, err := client.IsNodeAvailable(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("IsNodeAvailable() error = %v", err)
	}
	if !available {
		t.Fatal("IsNodeAvailable() = false, want true")
	}

	state, err := client.FetchChainState(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("FetchChainState() error = %v", err)
	}
	if state.Height != 120 || state.FinalizedHeight != 118 {
		t.Fatalf("state = %#v, want height=120 finalized=118", state)
	}
}

func TestDeriveProbeMetrics(t *testing.T) {
	metrics, err := DeriveProbeMetrics(125, ChainState{Height: 120, FinalizedHeight: 118})
	if err != nil {
		t.Fatalf("DeriveProbeMetrics() error = %v", err)
	}
	if metrics.SourceHeight != 125 || metrics.PeerHeight != 120 || metrics.FinalizedLagBlocks != 2 || metrics.ChainLagBlocks != 5 {
		t.Fatalf("metrics = %#v", metrics)
	}
}

func TestDeriveProbeMetricsClampsNegativeLags(t *testing.T) {
	metrics, err := DeriveProbeMetrics(100, ChainState{Height: 120, FinalizedHeight: 121})
	if err != nil {
		t.Fatalf("DeriveProbeMetrics() error = %v", err)
	}
	if metrics.FinalizedLagBlocks != 0 || metrics.ChainLagBlocks != 0 {
		t.Fatalf("metrics = %#v, want zeroed lags", metrics)
	}
}

func newIPv4Server(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("sandbox does not allow local listener: %v", err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	return server
}
