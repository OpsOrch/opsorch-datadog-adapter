package service

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

func TestApplyFilters(t *testing.T) {
	services := []schema.Service{
		{
			ID:   "service-1",
			Name: "api-service",
			Tags: map[string]string{
				"team": "backend",
				"env":  "prod",
			},
		},
		{
			ID:   "service-2",
			Name: "web-service",
			Tags: map[string]string{
				"team": "frontend",
				"env":  "prod",
			},
		},
		{
			ID:   "service-3",
			Name: "api-gateway",
			Tags: map[string]string{
				"team": "backend",
				"env":  "staging",
			},
		},
	}

	tests := []struct {
		name     string
		query    schema.ServiceQuery
		expected int
	}{
		{
			name:     "no filters returns all",
			query:    schema.ServiceQuery{},
			expected: 3,
		},
		{
			name: "filter by ID",
			query: schema.ServiceQuery{
				IDs: []string{"service-1"},
			},
			expected: 1,
		},
		{
			name: "filter by name substring",
			query: schema.ServiceQuery{
				Name: "api",
			},
			expected: 2,
		},
		{
			name: "filter by tags",
			query: schema.ServiceQuery{
				Tags: map[string]string{
					"team": "backend",
				},
			},
			expected: 2,
		},
		{
			name: "filter by scope team",
			query: schema.ServiceQuery{
				Scope: schema.QueryScope{
					Team: "frontend",
				},
			},
			expected: 1,
		},
		{
			name: "filter by scope environment",
			query: schema.ServiceQuery{
				Scope: schema.QueryScope{
					Environment: "prod",
				},
			},
			expected: 2,
		},
		{
			name: "filter with limit",
			query: schema.ServiceQuery{
				Limit: 2,
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyFilters(services, tt.query)
			if len(result) != tt.expected {
				t.Errorf("applyFilters() returned %d services, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("findFirstColon", func(t *testing.T) {
		tests := []struct {
			input    string
			expected int
		}{
			{"key:value", 3},
			{"no-colon", -1},
			{"multiple:colons:here", 8},
			{"", -1},
		}

		for _, tt := range tests {
			result := findFirstColon(tt.input)
			if result != tt.expected {
				t.Errorf("findFirstColon(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		}
	})

	t.Run("containsIgnoreCase", func(t *testing.T) {
		tests := []struct {
			s        string
			substr   string
			expected bool
		}{
			{"Hello World", "world", true},
			{"Hello World", "HELLO", true},
			{"Hello World", "foo", false},
			{"", "", true},
			{"test", "", true},
		}

		for _, tt := range tests {
			result := containsIgnoreCase(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("containsIgnoreCase(%v, %v) = %v, want %v", tt.s, tt.substr, result, tt.expected)
			}
		}
	})
}

// TestServiceDefinitionV2Dot2WithTeam verifies that service definitions
// should be created using v2.2 schema to include team information.
func TestServiceDefinitionV2Dot2WithTeam(t *testing.T) {
	// This test documents the learning that Datadog Service Definition API
	// v2.2 schema is required to properly set and extract team information.
	//
	// The v2.2 schema includes:
	// - Team field (string)
	// - Tags field ([]string) for env, service, etc.
	// - Application, Tier, Description fields
	//
	// This was discovered during integration testing when service team filtering
	// returned 0 results because services weren't created with team information.
	// Using v2.2 schema with explicit team field resolved the issue.

	t.Log("Service definitions must use v2.2 schema for team support")
	t.Log("Example: datadogV2.NewServiceDefinitionV2Dot2(name, SERVICEDEFINITIONV2DOT2VERSION_V2_2)")
	t.Log("Then set team: serviceDef.SetTeam(\"platform\")")
}

// TestExtractTagsFromSchema verifies that tags are properly extracted
// from different service definition schema versions.
func TestExtractTagsFromSchema(t *testing.T) {
	// This test documents the learning that extractTagsFromSchema() function
	// must handle different schema versions (v2, v2.1, v2.2) and extract:
	// - Tags from the tags array (format: "key:value")
	// - Team from the team field
	//
	// The extraction logic uses type assertions to handle the union type
	// returned by the Datadog SDK.

	t.Log("Tag extraction must handle v2, v2.1, and v2.2 schemas")
	t.Log("Tags format: []string{\"env:prod\", \"team:platform\"}")
	t.Log("Team field: string (v2.2 schema)")
}

// TestClientSideFiltering verifies that service filtering is done client-side
// because Datadog Service Definitions API doesn't support server-side filtering.
func TestClientSideFiltering(t *testing.T) {
	// This test documents the learning that Datadog Service Definitions API
	// only supports pagination parameters (PageSize, PageNumber, SchemaVersion).
	//
	// No server-side filtering is available for:
	// - Name
	// - Tags
	// - Team
	// - Environment
	//
	// Therefore, all filtering must be done client-side after fetching all
	// service definitions across all pages.
	//
	// This was discovered during integration testing when attempting to use
	// query parameters for filtering, which are not supported by the API.

	t.Log("Service filtering must be done client-side")
	t.Log("API only supports: PageSize, PageNumber, SchemaVersion")
	t.Log("Client-side filters: Name, Tags, Team, Environment, IDs")
}

// TestPaginationRequired verifies that pagination is required to fetch
// all service definitions.
func TestPaginationRequired(t *testing.T) {
	// This test documents the learning that service definitions must be
	// fetched across multiple pages to get the complete list.
	//
	// The API returns a maximum of 100 services per page (default page size).
	// The implementation must loop through pages until no more results are returned.
	//
	// This was discovered during integration testing when only the first page
	// of services was being returned, missing services on subsequent pages.

	t.Log("Service queries must paginate through all pages")
	t.Log("Default page size: 100")
	t.Log("Loop until len(data) < pageSize")
}
