package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/u2yasan/ssnp_sip/portal/internal/store"
	"github.com/u2yasan/ssnp_sip/portal/internal/verify"
)

func (s *Server) authorizeMapPayload(node store.Node, payload map[string]any) error {
	if node.AgentPublicKey == "" {
		return errors.New("unknown fingerprint")
	}
	if stringField(payload, "agent_key_fingerprint") != node.ActiveAgentKeyFingerprint {
		return errors.New("unknown fingerprint")
	}
	canonical, err := verify.CanonicalJSONWithoutSignature(payload)
	if err != nil {
		return err
	}
	return verify.VerifyHexPublicKeySignature(node.AgentPublicKey, stringField(payload, "signature"), canonical)
}

func (s *Server) validateTimestamp(raw string) error {
	ts, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return err
	}
	maxSkew := time.Duration(s.cfg.AllowedClockSkewSeconds) * time.Second
	now := time.Now().UTC()
	if ts.Before(now.Add(-maxSkew)) || ts.After(now.Add(maxSkew)) {
		return fmt.Errorf("timestamp outside allowed clock skew")
	}
	return nil
}

func decodeObject(r *http.Request) (map[string]any, error) {
	defer r.Body.Close()
	var payload map[string]any
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func stringField(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return value
}

func intField(payload map[string]any, key string) (int, bool) {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return 0, false
	}
	switch value := raw.(type) {
	case json.Number:
		intValue, err := value.Int64()
		if err != nil {
			return 0, false
		}
		return int(intValue), true
	case float64:
		if value != float64(int(value)) {
			return 0, false
		}
		return int(value), true
	default:
		return 0, false
	}
}

func optionalNonNegativeIntField(payload map[string]any, key string) (*int, bool, error) {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return nil, false, nil
	}
	var intValue int
	switch value := raw.(type) {
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			return nil, false, fmt.Errorf("%s must be an integer", key)
		}
		intValue = int(parsed)
	case float64:
		if value != float64(int(value)) {
			return nil, false, fmt.Errorf("%s must be an integer", key)
		}
		intValue = int(value)
	default:
		return nil, false, fmt.Errorf("%s must be an integer", key)
	}
	if intValue < 0 {
		return nil, false, fmt.Errorf("%s must be >= 0", key)
	}
	return &intValue, true, nil
}
