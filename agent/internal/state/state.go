package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type State struct {
	SequenceNumber      int    `json:"sequence_number"`
	LastPolicyVersion   string `json:"last_policy_version"`
	AgentKeyFingerprint string `json:"agent_key_fingerprint"`
}

func Load(path string) (State, error) {
	var st State
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, err
	}
	return st, nil
}

func Save(path string, st State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
