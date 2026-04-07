package config

import "testing"

func TestConfigValidate(t *testing.T) {
	valid := Config{
		PortalBaseURL:         "https://portal.example.net",
		RegionID:              "ap-sg-1",
		SourceEndpoint:        "https://source.example.net:3001",
		RequestTimeoutSeconds: 5,
		PollIntervalSeconds:   30,
		Targets: []Target{
			{NodeID: "node-abc", Endpoint: "https://node-abc.example.net:3001"},
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidateRejectsBrokenInput(t *testing.T) {
	tests := []Config{
		{},
		{
			PortalBaseURL:         "https://portal.example.net",
			RegionID:              "ap-sg-1",
			SourceEndpoint:        "https://source.example.net:3001",
			RequestTimeoutSeconds: 5,
			PollIntervalSeconds:   30,
		},
		{
			PortalBaseURL:         "https://portal.example.net",
			RegionID:              "ap-sg-1",
			SourceEndpoint:        "https://source.example.net:3001",
			RequestTimeoutSeconds: 5,
			PollIntervalSeconds:   30,
			Targets:               []Target{{NodeID: "", Endpoint: "https://node-abc.example.net:3001"}},
		},
		{
			PortalBaseURL:         "https://portal.example.net",
			RegionID:              "ap-sg-1",
			SourceEndpoint:        "https://source.example.net:3001",
			RequestTimeoutSeconds: 5,
			PollIntervalSeconds:   30,
			Targets: []Target{
				{NodeID: "node-abc", Endpoint: "https://node-abc.example.net:3001"},
				{NodeID: "node-abc", Endpoint: "https://node-def.example.net:3001"},
			},
		},
	}

	for _, tc := range tests {
		if err := tc.Validate(); err == nil {
			t.Fatalf("Validate() error = nil for %#v", tc)
		}
	}
}
