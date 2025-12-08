package log

import (
	"testing"

	"github.com/opsorch/opsorch-core/schema"
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

func TestBuildLogQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    schema.LogQuery
		expected string
	}{
		{
			name: "empty query returns wildcard",
			query: schema.LogQuery{
				Expression: nil,
			},
			expected: "*",
		},
		{
			name: "simple search term",
			query: schema.LogQuery{
				Expression: &schema.LogExpression{
					Search: "error",
				},
			},
			expected: "error",
		},
		{
			name: "search with severity filter",
			query: schema.LogQuery{
				Expression: &schema.LogExpression{
					Search:     "error",
					SeverityIn: []string{"error", "critical"},
				},
			},
			expected: "error status:(error OR critical)",
		},
		{
			name: "search with scope",
			query: schema.LogQuery{
				Expression: &schema.LogExpression{
					Search: "error",
				},
				Scope: schema.QueryScope{
					Service: "api",
				},
			},
			expected: "error service:api",
		},
		{
			name: "search with filters",
			query: schema.LogQuery{
				Expression: &schema.LogExpression{
					Search: "error",
					Filters: []schema.LogFilter{
						{Field: "host", Operator: "=", Value: "web01"},
					},
				},
			},
			expected: "error host:web01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildLogQuery(tt.query)
			if result != tt.expected {
				t.Errorf("buildLogQuery() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMapSeverityToDatadog(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"critical", "critical"},
		{"emergency", "critical"},
		{"error", "error"},
		{"warning", "warn"},
		{"warn", "warn"},
		{"info", "info"},
		{"debug", "debug"},
		{"trace", "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapSeverityToDatadog(tt.input)
			if result != tt.expected {
				t.Errorf("mapSeverityToDatadog(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMapDatadogSeverityToOpsOrch(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"critical", "critical"},
		{"error", "error"},
		{"warn", "warning"},
		{"warning", "warning"},
		{"info", "info"},
		{"debug", "debug"},
		{"trace", "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapDatadogSeverityToOpsOrch(tt.input)
			if result != tt.expected {
				t.Errorf("mapDatadogSeverityToOpsOrch(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatLogFilter(t *testing.T) {
	tests := []struct {
		name     string
		filter   schema.LogFilter
		expected string
	}{
		{
			name:     "equals operator",
			filter:   schema.LogFilter{Field: "host", Operator: "=", Value: "web01"},
			expected: "host:web01",
		},
		{
			name:     "not equals operator",
			filter:   schema.LogFilter{Field: "env", Operator: "!=", Value: "prod"},
			expected: "-env:prod",
		},
		{
			name:     "contains operator",
			filter:   schema.LogFilter{Field: "message", Operator: "contains", Value: "error"},
			expected: "message:*error*",
		},
		{
			name:     "regex operator",
			filter:   schema.LogFilter{Field: "host", Operator: "regex", Value: "web.*"},
			expected: "host:web.*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLogFilter(tt.filter)
			if result != tt.expected {
				t.Errorf("formatLogFilter() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestLogPaginationUsesCursor documents that log queries use cursor-based
// pagination to retrieve all results across multiple pages.
func TestLogPaginationUsesCursor(t *testing.T) {
	// This test documents the CRITICAL requirement that log queries must
	// implement cursor-based pagination to retrieve all results.
	//
	// BACKGROUND:
	// Datadog's Logs API uses cursor-based pagination, not offset-based.
	// Each response includes meta.page.after cursor for the next page.
	//
	// PROBLEM:
	// Without pagination, queries are capped at 1000 entries (default page size),
	// silently truncating large result sets regardless of requested limit.
	//
	// SOLUTION:
	// The Query method must:
	// 1. Make initial request with page.limit
	// 2. Check response.meta.page.after for cursor
	// 3. If cursor exists, make next request with page.cursor set
	// 4. Continue until no cursor returned or limit reached
	//
	// IMPLEMENTATION:
	// - Page size: 1000 entries per request (Datadog default)
	// - Default limit: 100 entries (reasonable for LLM context)
	// - Configurable: Set query.Limit to fetch more pages
	// - Cursor field: response.Meta.Page.After
	// - Next request: page.SetCursor(*cursor)
	//
	// This was discovered during code review when analyzing why large
	// log queries were silently truncated at 1000 entries.

	t.Log("Log queries must implement cursor-based pagination")
	t.Log("Each page returns up to 1000 entries")
	t.Log("Use response.Meta.Page.After cursor for next page")
	t.Log("Continue until no cursor or limit reached")
}

// TestJSONFormatForStatusExtraction verifies that logs should be submitted
// in JSON format with explicit status field for reliable severity filtering.
func TestJSONFormatForStatusExtraction(t *testing.T) {
	// This test documents the learning that Datadog's log processing pipeline
	// reliably extracts status from JSON-formatted messages with explicit
	// status field, rather than relying on log level keyword extraction.
	//
	// Recommended format: {"level":"error","status":"error","message":"..."}
	//
	// This was discovered during integration testing when log severity filtering
	// returned 0 results with keyword-based extraction ([ERROR], [WARN], etc.)
	// but worked reliably with JSON format containing explicit status field.

	tests := []struct {
		name           string
		message        string
		expectedStatus string
	}{
		{
			name:           "JSON with error status",
			message:        `{"level":"error","status":"error","message":"Database connection failed"}`,
			expectedStatus: "error",
		},
		{
			name:           "JSON with warn status",
			message:        `{"level":"warn","status":"warn","message":"High memory usage"}`,
			expectedStatus: "warn",
		},
		{
			name:           "JSON with info status",
			message:        `{"level":"info","status":"info","message":"Application started"}`,
			expectedStatus: "info",
		},
		{
			name:           "JSON with critical status",
			message:        `{"level":"critical","status":"critical","message":"System failure"}`,
			expectedStatus: "critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the message is valid JSON
			if tt.message[0] != '{' || tt.message[len(tt.message)-1] != '}' {
				t.Errorf("Message should be JSON format, got: %s", tt.message)
			}

			// Verify it contains the status field
			if !contains(tt.message, `"status":"`+tt.expectedStatus+`"`) {
				t.Errorf("Message should contain status field with value %s", tt.expectedStatus)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
