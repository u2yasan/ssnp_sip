package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	NodeID                    string `yaml:"node_id"`
	PortalBaseURL             string `yaml:"portal_base_url"`
	AgentKeyPath              string `yaml:"agent_key_path"`
	AgentPublicKeyPath        string `yaml:"agent_public_key_path"`
	MonitoredEndpoint         string `yaml:"monitored_endpoint"`
	StatePath                 string `yaml:"state_path"`
	TempDir                   string `yaml:"temp_dir"`
	RequestTimeoutSeconds     int    `yaml:"request_timeout_seconds"`
	HeartbeatJitterSecondsMax int    `yaml:"heartbeat_jitter_seconds_max"`
	AgentVersion              string `yaml:"agent_version"`
	EnrollmentGeneration      int    `yaml:"enrollment_generation"`
}

func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	switch {
	case c.NodeID == "":
		return errors.New("config: node_id is required")
	case c.PortalBaseURL == "":
		return errors.New("config: portal_base_url is required")
	case c.AgentKeyPath == "":
		return errors.New("config: agent_key_path is required")
	case c.AgentPublicKeyPath == "":
		return errors.New("config: agent_public_key_path is required")
	case c.MonitoredEndpoint == "":
		return errors.New("config: monitored_endpoint is required")
	case c.StatePath == "":
		return errors.New("config: state_path is required")
	case c.TempDir == "":
		return errors.New("config: temp_dir is required")
	case c.RequestTimeoutSeconds <= 0:
		return errors.New("config: request_timeout_seconds must be > 0")
	case c.HeartbeatJitterSecondsMax < 0:
		return errors.New("config: heartbeat_jitter_seconds_max must be >= 0")
	case c.AgentVersion == "":
		return errors.New("config: agent_version is required")
	case c.EnrollmentGeneration <= 0:
		return errors.New("config: enrollment_generation must be > 0")
	default:
		return nil
	}
}
