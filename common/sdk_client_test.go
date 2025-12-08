package common

import (
	"testing"
)

func TestNewAPIClient(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "US1 site",
			config: Config{
				APIKey: "test-api-key",
				AppKey: "test-app-key",
				Site:   "datadoghq.com",
				Source: "datadog",
			},
		},
		{
			name: "EU site",
			config: Config{
				APIKey: "test-api-key",
				AppKey: "test-app-key",
				Site:   "datadoghq.eu",
				Source: "datadog",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewAPIClient(tt.config)
			if client == nil {
				t.Error("NewAPIClient() returned nil")
			}
		})
	}
}

func TestNewAPIContext(t *testing.T) {
	config := Config{
		APIKey: "test-api-key",
		AppKey: "test-app-key",
		Site:   "datadoghq.com",
		Source: "datadog",
	}

	ctx := NewAPIContext(config)
	if ctx == nil {
		t.Error("NewAPIContext() returned nil")
	}
}
