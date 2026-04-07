package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u2yasan/ssnp_sip/portal/internal/store"
)

func smokeNodesConfigPath() string {
	return filepath.Clean("../../nodes.example.yaml")
}

func TestSmokeSeedMatchesNodesExampleConfig(t *testing.T) {
	seedNodes, err := store.LoadNodesConfig(smokeNodesConfigPath())
	if err != nil {
		t.Fatalf("LoadNodesConfig() error = %v", err)
	}
	if len(seedNodes) != 1 {
		t.Fatalf("len(seedNodes) = %d, want 1", len(seedNodes))
	}

	data, err := os.ReadFile(smokeStateSeedPath())
	if err != nil {
		t.Fatalf("ReadFile(smokeStateSeedPath) error = %v", err)
	}
	var payload struct {
		Nodes []struct {
			NodeID        string `json:"node_id"`
			DisplayName   string `json:"display_name"`
			OperatorEmail string `json:"operator_email"`
			Enabled       bool   `json:"enabled"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(payload.Nodes) != 1 {
		t.Fatalf("len(payload.Nodes) = %d, want 1", len(payload.Nodes))
	}

	want := seedNodes[0]
	got := payload.Nodes[0]
	if got.NodeID != want.NodeID {
		t.Fatalf("smoke node_id = %q, want %q", got.NodeID, want.NodeID)
	}
	if got.DisplayName != want.DisplayName {
		t.Fatalf("smoke display_name = %q, want %q", got.DisplayName, want.DisplayName)
	}
	if got.OperatorEmail != want.OperatorEmail {
		t.Fatalf("smoke operator_email = %q, want %q", got.OperatorEmail, want.OperatorEmail)
	}
	if got.Enabled != want.Enabled {
		t.Fatalf("smoke enabled = %t, want %t", got.Enabled, want.Enabled)
	}
}
