package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/u2yasan/ssnp_sip/agent/internal/config"
	"github.com/u2yasan/ssnp_sip/agent/internal/runtime"
)

func TestRunGenKeyWritesLoadableEd25519KeyPair(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer

	if err := runGenKey([]string{"--out-dir", dir}, &stdout); err != nil {
		t.Fatalf("runGenKey() error = %v", err)
	}

	var payload struct {
		PrivateKeyPath string `json:"private_key_path"`
		PublicKeyPath  string `json:"public_key_path"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.PrivateKeyPath == "" || payload.PublicKeyPath == "" {
		t.Fatalf("runGenKey() payload = %#v, want key paths", payload)
	}

	cfgPath := filepath.Join(dir, "config.yaml")
	cfgYAML := "" +
		"node_id: \"node-abc\"\n" +
		"portal_base_url: \"http://127.0.0.1:8080\"\n" +
		"agent_key_path: \"" + filepath.Join(dir, "agent_private_key.pem") + "\"\n" +
		"agent_public_key_path: \"" + filepath.Join(dir, "agent_public_key.pem") + "\"\n" +
		"monitored_endpoint: \"https://node-01.example.net:3001\"\n" +
		"state_path: \"" + filepath.Join(dir, "state.json") + "\"\n" +
		"temp_dir: \"" + dir + "\"\n" +
		"request_timeout_seconds: 5\n" +
		"heartbeat_jitter_seconds_max: 0\n" +
		"agent_version: \"1.0.0\"\n" +
		"enrollment_generation: 1\n"
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	if _, err := runtime.NewAgent(cfg); err != nil {
		t.Fatalf("runtime.NewAgent() error = %v", err)
	}
}
