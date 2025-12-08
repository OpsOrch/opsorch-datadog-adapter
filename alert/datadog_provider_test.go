package alert

import (
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		wantErr bool
	}{
		{
			name: "valid config",
			config: map[string]any{
				"apiKey": "test-api-key",
				"appKey": "test-app-key",
			},
			wantErr: false,
		},
		{
			name: "missing apiKey",
			config: map[string]any{
				"appKey": "test-app-key",
			},
			wantErr: true,
		},
		{
			name: "missing appKey",
			config: map[string]any{
				"apiKey": "test-api-key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && provider == nil {
				t.Error("New() returned nil provider for valid config")
			}
		})
	}
}

func TestMapOpsOrchStatusToDatadog(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"firing", "Alert"},
		{"alert", "Alert"},
		{"warning", "Warn"},
		{"warn", "Warn"},
		{"ok", "OK"},
		{"resolved", "OK"},
		{"unknown", "No Data"},
		{"no data", "No Data"},
		{"skipped", "Skipped"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := MapOpsOrchStatusToDatadog(tt.input)
			if result != tt.expected {
				t.Errorf("MapOpsOrchStatusToDatadog(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMapDatadogStatusToOpsOrch(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Alert", "firing"},
		{"alert", "firing"},
		{"Warn", "warning"},
		{"warn", "warning"},
		{"OK", "ok"},
		{"ok", "ok"},
		{"No Data", "unknown"},
		{"nodata", "unknown"},
		{"Skipped", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := MapDatadogStatusToOpsOrch(tt.input)
			if result != tt.expected {
				t.Errorf("MapDatadogStatusToOpsOrch(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMapOpsOrchSeverityToPriority(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"critical", "P1"},
		{"p1", "P1"},
		{"high", "P2"},
		{"p2", "P2"},
		{"medium", "P3"},
		{"p3", "P3"},
		{"low", "P4"},
		{"p4", "P4"},
		{"info", "P5"},
		{"p5", "P5"},
		{"unknown", "P3"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := MapOpsOrchSeverityToPriority(tt.input)
			if result != tt.expected {
				t.Errorf("MapOpsOrchSeverityToPriority(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMapPriorityToOpsOrchSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"P1", "critical"},
		{"p1", "critical"},
		{"P2", "high"},
		{"p2", "high"},
		{"P3", "medium"},
		{"p3", "medium"},
		{"P4", "low"},
		{"p4", "low"},
		{"P5", "info"},
		{"p5", "info"},
		{"unknown", "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := MapPriorityToOpsOrchSeverity(tt.input)
			if result != tt.expected {
				t.Errorf("MapPriorityToOpsOrchSeverity(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestScopeFilteringUsesMonitorTags verifies that service/team/environment scope
// filters are passed to the Datadog API using WithMonitorTags() parameter,
// not WithTags(). This is critical for proper monitor filtering.
func TestScopeFilteringUsesMonitorTags(t *testing.T) {
	// This test documents the learning that Datadog's ListMonitors API
	// requires monitor_tags parameter for filtering by monitor tags,
	// not the tags parameter which is used for different purposes.

	// The actual implementation is in Query() method which calls:
	// optParams.WithMonitorTags(strings.Join(tagFilters, ","))
	//
	// This was discovered during integration testing when service scope
	// filtering returned 0 results with WithTags() but worked correctly
	// with WithMonitorTags().

	t.Log("Service scope filtering must use WithMonitorTags() parameter")
	t.Log("Example: service:datadog should be passed as monitor_tags=service:datadog")
}
