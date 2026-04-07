package config

import (
	"errors"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PortalBaseURL         string   `yaml:"portal_base_url"`
	RegionID              string   `yaml:"region_id"`
	SourceEndpoint        string   `yaml:"source_endpoint"`
	RequestTimeoutSeconds int      `yaml:"request_timeout_seconds"`
	PollIntervalSeconds   int      `yaml:"poll_interval_seconds"`
	Targets               []Target `yaml:"targets"`
}

type Target struct {
	NodeID   string `yaml:"node_id"`
	Endpoint string `yaml:"endpoint"`
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
	case strings.TrimSpace(c.PortalBaseURL) == "":
		return errors.New("config: portal_base_url is required")
	case strings.TrimSpace(c.RegionID) == "":
		return errors.New("config: region_id is required")
	case strings.TrimSpace(c.SourceEndpoint) == "":
		return errors.New("config: source_endpoint is required")
	case c.RequestTimeoutSeconds <= 0:
		return errors.New("config: request_timeout_seconds must be > 0")
	case c.PollIntervalSeconds <= 0:
		return errors.New("config: poll_interval_seconds must be > 0")
	case len(c.Targets) == 0:
		return errors.New("config: targets must not be empty")
	}

	seen := map[string]struct{}{}
	for _, target := range c.Targets {
		nodeID := strings.TrimSpace(target.NodeID)
		endpoint := strings.TrimSpace(target.Endpoint)
		if nodeID == "" {
			return errors.New("config: targets[].node_id is required")
		}
		if endpoint == "" {
			return errors.New("config: targets[].endpoint is required")
		}
		if _, exists := seen[nodeID]; exists {
			return errors.New("config: duplicate targets[].node_id")
		}
		seen[nodeID] = struct{}{}
	}
	return nil
}
