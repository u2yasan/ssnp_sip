package heartbeat

import (
	"encoding/json"
	"time"
)

type Payload struct {
	NodeID               string   `json:"node_id"`
	AgentKeyFingerprint  string   `json:"agent_key_fingerprint"`
	HeartbeatTimestamp   string   `json:"heartbeat_timestamp"`
	SequenceNumber       int      `json:"sequence_number"`
	AgentVersion         string   `json:"agent_version"`
	EnrollmentGeneration int      `json:"enrollment_generation"`
	LocalObservationFlags []string `json:"local_observation_flags"`
	Signature            string   `json:"signature,omitempty"`
}

func New(nodeID, fingerprint, agentVersion string, enrollmentGeneration, sequence int, flags []string) Payload {
	return Payload{
		NodeID:                nodeID,
		AgentKeyFingerprint:   fingerprint,
		HeartbeatTimestamp:    time.Now().UTC().Format(time.RFC3339),
		SequenceNumber:        sequence,
		AgentVersion:          agentVersion,
		EnrollmentGeneration:  enrollmentGeneration,
		LocalObservationFlags: flags,
	}
}

func (p Payload) CanonicalBytes() ([]byte, error) {
	copyPayload := p
	copyPayload.Signature = ""
	return json.Marshal(copyPayload)
}
