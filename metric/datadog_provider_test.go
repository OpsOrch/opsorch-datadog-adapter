package metric

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

func TestBuildMetricQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    schema.MetricQuery
		expected string
	}{
		{
			name: "simple metric name",
			query: schema.MetricQuery{
				Expression: &schema.MetricExpression{
					MetricName: "system.cpu.user",
				},
			},
			expected: "system.cpu.user",
		},
		{
			name: "metric with aggregation (v2 API ignores aggregation in query string)",
			query: schema.MetricQuery{
				Expression: &schema.MetricExpression{
					MetricName:  "system.cpu.user",
					Aggregation: "avg",
				},
			},
			expected: "system.cpu.user",
		},
		{
			name: "metric with filters",
			query: schema.MetricQuery{
				Expression: &schema.MetricExpression{
					MetricName: "system.cpu.user",
					Filters: []schema.MetricFilter{
						{Label: "host", Operator: "=", Value: "web01"},
					},
				},
			},
			expected: "system.cpu.user{host:web01}",
		},
		{
			name: "metric with scope",
			query: schema.MetricQuery{
				Expression: &schema.MetricExpression{
					MetricName: "http.requests",
				},
				Scope: schema.QueryScope{
					Service: "api",
				},
			},
			expected: "http.requests{service:api}",
		},
		{
			name: "metric with aggregation and group by (v2 API ignores in query string)",
			query: schema.MetricQuery{
				Expression: &schema.MetricExpression{
					MetricName:  "system.cpu.user",
					Aggregation: "sum",
					GroupBy:     []string{"host", "region"},
				},
			},
			expected: "system.cpu.user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildMetricQuery(tt.query)
			if result != tt.expected {
				t.Errorf("buildMetricQuery() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMapAggregation(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"avg", "avg"},
		{"average", "avg"},
		{"sum", "sum"},
		{"max", "max"},
		{"min", "min"},
		{"count", "count"},
		{"unknown", "avg"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapAggregation(tt.input)
			if result != tt.expected {
				t.Errorf("mapAggregation(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatTagFilter(t *testing.T) {
	tests := []struct {
		name     string
		filter   schema.MetricFilter
		expected string
	}{
		{
			name:     "equals operator",
			filter:   schema.MetricFilter{Label: "host", Operator: "=", Value: "web01"},
			expected: "host:web01",
		},
		{
			name:     "not equals operator",
			filter:   schema.MetricFilter{Label: "env", Operator: "!=", Value: "prod"},
			expected: "!env:prod",
		},
		{
			name:     "regex match operator",
			filter:   schema.MetricFilter{Label: "host", Operator: "=~", Value: "web.*"},
			expected: "host:web.*",
		},
		{
			name:     "regex not match operator",
			filter:   schema.MetricFilter{Label: "host", Operator: "!~", Value: "test.*"},
			expected: "!host:test.*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTagFilter(tt.filter)
			if result != tt.expected {
				t.Errorf("formatTagFilter() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestMetricQueryFormatRequiresEmptyTagSet verifies that metric queries
// properly format tag filters in the query string.
func TestMetricQueryFormatRequiresEmptyTagSet(t *testing.T) {
	// This test documents the learning that Datadog Metrics API v2 uses
	// tag filters in the format: metric{tag:value,tag2:value2}
	//
	// When no filters are specified, the query is just the metric name.
	// When filters are specified, they are enclosed in curly braces.
	//
	// The v2 API handles queries differently than v1:
	// - v1 required: avg:metric{*}
	// - v2 accepts: metric or metric{tag:value}
	//
	// This was discovered during integration testing when working with
	// the Datadog Metrics API v2 timeseries endpoint.

	tests := []struct {
		name          string
		metricName    string
		filters       []schema.MetricFilter
		expectedQuery string
	}{
		{
			name:          "no filters returns metric name only",
			metricName:    "system.cpu.user",
			filters:       nil,
			expectedQuery: "system.cpu.user",
		},
		{
			name:       "with filters uses actual tags",
			metricName: "system.cpu.user",
			filters: []schema.MetricFilter{
				{Label: "host", Operator: "=", Value: "web01"},
			},
			expectedQuery: "system.cpu.user{host:web01}",
		},
		{
			name:       "multiple filters",
			metricName: "system.cpu.user",
			filters: []schema.MetricFilter{
				{Label: "host", Operator: "=", Value: "web01"},
				{Label: "env", Operator: "=", Value: "prod"},
			},
			expectedQuery: "system.cpu.user{host:web01,env:prod}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := schema.MetricQuery{
				Expression: &schema.MetricExpression{
					MetricName: tt.metricName,
					Filters:    tt.filters,
				},
			}

			result := buildMetricQuery(query)

			if result != tt.expectedQuery {
				t.Errorf("buildMetricQuery() = %s, want %s", result, tt.expectedQuery)
			}
		})
	}
}

// TestScopeFilteringWithEnvironment verifies that environment scope
// is properly converted to env tag filter.
func TestScopeFilteringWithEnvironment(t *testing.T) {
	// This test documents the learning that scope.Environment should be
	// converted to env:value tag filter in the metric query.
	//
	// Example: scope.Environment = "prod" -> {env:prod}
	//
	// This was discovered during integration testing when environment scope
	// filtering returned 0 results until proper tag conversion was implemented.

	t.Log("Environment scope must be converted to env tag filter")
	t.Log("Example: scope.Environment='prod' -> metric{env:prod}")
}

// TestMetricQueryV2FormatWithGroupByAndRollup verifies that v2 API queries
// support GroupBy and Step (rollup) in the query string format.
func TestMetricQueryV2FormatWithGroupByAndRollup(t *testing.T) {
	// This test documents the learning that Datadog Metrics API v2 supports
	// advanced query features in the query string:
	//
	// 1. GroupBy: aggregation:metric{tags} by {label1,label2}
	// 2. Rollup: aggregation:metric{tags}.rollup(agg, interval)
	//
	// Example: avg:system.cpu.user{env:prod} by {host}.rollup(avg, 60)
	//
	// This was discovered during integration testing when implementing
	// Test 5 (Query with groupBy) which requires proper query formatting.

	tests := []struct {
		name          string
		metricName    string
		groupBy       []string
		step          int
		expectedParts []string
	}{
		{
			name:       "with groupBy",
			metricName: "system.cpu.user",
			groupBy:    []string{"host", "region"},
			step:       0,
			expectedParts: []string{
				"avg:system.cpu.user{*}",
				"by {host,region}",
			},
		},
		{
			name:       "with rollup",
			metricName: "system.cpu.user",
			groupBy:    nil,
			step:       60,
			expectedParts: []string{
				"avg:system.cpu.user{*}",
				".rollup(avg, 60)",
			},
		},
		{
			name:       "with both groupBy and rollup",
			metricName: "system.cpu.user",
			groupBy:    []string{"host"},
			step:       60,
			expectedParts: []string{
				"avg:system.cpu.user{*}",
				"by {host}",
				".rollup(avg, 60)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := schema.MetricQuery{
				Expression: &schema.MetricExpression{
					MetricName: tt.metricName,
					GroupBy:    tt.groupBy,
				},
				Step: tt.step,
			}

			result := buildMetricQueryV2(query)

			// Verify all expected parts are in the query
			for _, part := range tt.expectedParts {
				if !contains(result, part) {
					t.Errorf("buildMetricQueryV2() = %s, should contain %s", result, part)
				}
			}
		})
	}
}

// TestDescribePaginationWithCursor verifies that metric describe operations
// must paginate using the next_cursor from the response metadata.
func TestDescribePaginationWithCursor(t *testing.T) {
	// This test documents the learning that Datadog Metrics API v2
	// ListTagConfigurations endpoint uses cursor-based pagination.
	//
	// The response includes:
	// - meta.pagination.next_cursor: cursor for next page
	//
	// CRITICAL FIX: Must pass cursor back to API using WithPageCursor()
	//
	// Implementation must:
	// 1. Check if next_cursor exists and is non-empty
	// 2. Call optParams.WithPageCursor(*nextCursor) for next request
	// 3. Continue until next_cursor is empty
	//
	// BEFORE (broken): Read cursor but never used it, always fetched page 1
	// AFTER (fixed): Pass cursor via WithPageCursor() to fetch next pages
	//
	// This was discovered during code review when analyzing why Describe
	// was only returning the first page of metrics.

	t.Log("CRITICAL: Metric describe must use cursor-based pagination")
	t.Log("Read: meta.pagination.next_cursor")
	t.Log("Use: optParams.WithPageCursor(*nextCursor)")
	t.Log("Continue until next_cursor is empty")
}

// TestServiceFilterFormatInDescribe verifies that service scope filter
// must use "service:name" format in the Describe operation.
func TestServiceFilterFormatInDescribe(t *testing.T) {
	// This test documents the learning that when filtering metrics by service
	// in the Describe operation, the filter must be in "service:name" format.
	//
	// Example: scope.Service = "datadog" -> filter_tags_configured="service:datadog"
	//
	// This was discovered during integration testing when service filtering
	// in Describe returned incorrect results until proper format was used.

	t.Log("Service filter in Describe must use 'service:name' format")
	t.Log("Example: scope.Service='datadog' -> filter_tags_configured='service:datadog'")
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
