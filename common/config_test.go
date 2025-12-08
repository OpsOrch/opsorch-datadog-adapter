package common

import (
	"testing"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected Config
	}{
		{
			name: "full config",
			input: map[string]any{
				"apiKey": "test-api-key",
				"appKey": "test-app-key",
				"site":   "datadoghq.eu",
				"source": "custom-source",
			},
			expected: Config{
				APIKey: "test-api-key",
				AppKey: "test-app-key",
				Site:   "datadoghq.eu",
				Source: "custom-source",
			},
		},
		{
			name: "minimal config with defaults",
			input: map[string]any{
				"apiKey": "test-api-key",
				"appKey": "test-app-key",
			},
			expected: Config{
				APIKey: "test-api-key",
				AppKey: "test-app-key",
				Site:   "datadoghq.com",
				Source: "datadog",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseConfig(tt.input)
			if result.APIKey != tt.expected.APIKey {
				t.Errorf("APIKey = %v, want %v", result.APIKey, tt.expected.APIKey)
			}
			if result.AppKey != tt.expected.AppKey {
				t.Errorf("AppKey = %v, want %v", result.AppKey, tt.expected.AppKey)
			}
			if result.Site != tt.expected.Site {
				t.Errorf("Site = %v, want %v", result.Site, tt.expected.Site)
			}
			if result.Source != tt.expected.Source {
				t.Errorf("Source = %v, want %v", result.Source, tt.expected.Source)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				APIKey: "test-api-key",
				AppKey: "test-app-key",
				Site:   "datadoghq.com",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: Config{
				AppKey: "test-app-key",
				Site:   "datadoghq.com",
			},
			wantErr: true,
		},
		{
			name: "missing App key",
			config: Config{
				APIKey: "test-api-key",
				Site:   "datadoghq.com",
			},
			wantErr: true,
		},
		{
			name: "empty API key",
			config: Config{
				APIKey: "",
				AppKey: "test-app-key",
				Site:   "datadoghq.com",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBaseURL(t *testing.T) {
	tests := []struct {
		name     string
		site     string
		expected string
	}{
		{
			name:     "US1 site",
			site:     "datadoghq.com",
			expected: "https://api.datadoghq.com",
		},
		{
			name:     "EU site",
			site:     "datadoghq.eu",
			expected: "https://api.datadoghq.eu",
		},
		{
			name:     "US3 site",
			site:     "us3.datadoghq.com",
			expected: "https://api.us3.datadoghq.com",
		},
		{
			name:     "unknown site defaults to US1",
			site:     "custom.example.com",
			expected: "https://api.datadoghq.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Site: tt.site}
			result := cfg.BaseURL()
			if result != tt.expected {
				t.Errorf("BaseURL() = %v, want %v", result, tt.expected)
			}
		})
	}
}
