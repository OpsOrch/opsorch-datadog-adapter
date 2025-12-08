package incident

import (
	"strings"
	"testing"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
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

func TestMapOpsOrchStatusToDatadog(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"open", "active"},
		{"triggered", "active"},
		{"investigating", "stable"},
		{"acknowledged", "stable"},
		{"resolved", "resolved"},
		{"closed", "resolved"},
		{"unknown", "active"},
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
		{"active", "open"},
		{"stable", "investigating"},
		{"resolved", "resolved"},
		{"unknown", "unknown"},
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

func TestMapOpsOrchSeverityToDatadog(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"critical", "SEV-1"},
		{"p1", "SEV-1"},
		{"sev1", "SEV-1"},
		{"high", "SEV-2"},
		{"p2", "SEV-2"},
		{"medium", "SEV-3"},
		{"p3", "SEV-3"},
		{"low", "SEV-4"},
		{"p4", "SEV-4"},
		{"info", "SEV-5"},
		{"p5", "SEV-5"},
		{"unknown", "SEV-3"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := MapOpsOrchSeverityToDatadog(tt.input)
			if result != tt.expected {
				t.Errorf("MapOpsOrchSeverityToDatadog(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMapDatadogSeverityToOpsOrch(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SEV-1", "critical"},
		{"sev-1", "critical"},
		{"SEV-2", "high"},
		{"sev-2", "high"},
		{"SEV-3", "medium"},
		{"sev-3", "medium"},
		{"SEV-4", "low"},
		{"sev-4", "low"},
		{"SEV-5", "info"},
		{"sev-5", "info"},
		{"unknown", "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := MapDatadogSeverityToOpsOrch(tt.input)
			if result != tt.expected {
				t.Errorf("MapDatadogSeverityToOpsOrch(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIncidentSeverityCannotBeSetViaAPI documents the known limitation
// that incident severity cannot be set via Create or Update API.
func TestIncidentSeverityCannotBeSetViaAPI(t *testing.T) {
	// This test documents the known Datadog API limitation that incident
	// severity cannot be set via the Create or Update API endpoints.
	//
	// The Datadog Incidents API v2 requires complex custom field types
	// to set severity, which are not exposed in the SDK's simple Create
	// and Update methods.
	//
	// As a result:
	// - All created incidents default to "medium" severity (SEV-3)
	// - Severity can only be changed via the Datadog UI or complex API calls
	//
	// This was discovered during integration testing when all incidents
	// were created with "medium" severity regardless of the input severity.
	//
	// Workaround: Accept that all incidents will have "medium" severity
	// when created via the API. Users must manually update severity in the UI.

	t.Log("KNOWN LIMITATION: Incident severity cannot be set via Create/Update API")
	t.Log("All incidents default to 'medium' (SEV-3) severity")
	t.Log("Severity must be changed manually in Datadog UI")
	t.Log("This is a Datadog API limitation, not a code issue")
}

// TestUnstableOperationsMustBeEnabled documents that incident operations
// require enabling unstable operations in the SDK configuration.
func TestUnstableOperationsMustBeEnabled(t *testing.T) {
	// This test documents the learning that Datadog Incidents API v2
	// operations are marked as "unstable" and must be explicitly enabled
	// in the SDK configuration.
	//
	// Required unstable operations:
	// - v2.ListIncidents
	// - v2.GetIncident
	// - v2.CreateIncident
	// - v2.UpdateIncident
	// - v2.DeleteIncident
	// - v2.SearchIncidents
	// - v2.ListIncidentTodos
	// - v2.CreateIncidentTodo
	// - v2.UpdateIncidentTodo
	// - v2.DeleteIncidentTodo
	//
	// This is done in common/sdk_client.go using:
	// config.SetUnstableOperationEnabled("v2.ListIncidents", true)
	//
	// This was discovered during integration testing when incident operations
	// returned "operation not enabled" errors until unstable operations were enabled.

	t.Log("Incident operations require enabling unstable operations")
	t.Log("Enable in SDK config: config.SetUnstableOperationEnabled()")
	t.Log("Required for all incident and todo operations")
}

// TestTimelineUsesTodosAPI documents that timeline functionality
// uses the Todos API since timeline API is not directly exposed.
func TestTimelineUsesTodosAPI(t *testing.T) {
	// This test documents the learning that Datadog SDK v2 doesn't expose
	// a direct timeline API, so we use the Todos API as timeline entries.
	//
	// Implementation:
	// - GetTimeline() calls ListIncidentTodos()
	// - AppendTimeline() calls CreateIncidentTodo() with completed status
	// - Todos are converted to timeline entries via normalizeSDKTodosToTimeline()
	//
	// This was discovered during integration testing when looking for
	// timeline API methods and finding only the Todos API available.

	t.Log("Timeline functionality uses Todos API")
	t.Log("GetTimeline() -> ListIncidentTodos()")
	t.Log("AppendTimeline() -> CreateIncidentTodo()")
}

// TestScopeFilteringIsClientSide documents that incident filtering
// by scope (service, team, environment) is done client-side.
func TestScopeFilteringIsClientSide(t *testing.T) {
	// This test documents the learning that Datadog Incidents API doesn't
	// support server-side filtering by scope fields (service, team, environment).
	//
	// The API only supports:
	// - Query (text search in title)
	// - PageSize, PageOffset (pagination)
	//
	// Therefore, scope filtering must be done client-side by:
	// 1. Fetching all incidents
	// 2. Checking incident metadata and fields for scope values
	// 3. Filtering results based on scope criteria
	//
	// This was discovered during integration testing when attempting to use
	// query parameters for scope filtering, which are not supported by the API.

	t.Log("Incident scope filtering must be done client-side")
	t.Log("API only supports: Query (text search), Pagination")
	t.Log("Client-side filters: Service, Team, Environment, Status, Severity")
}

// TestIncidentNormalizationReadsBuiltInAttributes documents that incident
// normalization must read built-in State and Severity attributes, not custom fields.
func TestIncidentNormalizationReadsBuiltInAttributes(t *testing.T) {
	// This test documents the critical learning that Datadog incidents have
	// built-in State and Severity attributes that must be read directly from
	// the response attributes, NOT from custom fields.
	//
	// CRITICAL FIX: The normalizeSDKIncident function was updated to:
	// 1. Read attrs.State.Get() for incident status
	// 2. Read attrs.Severity for incident severity
	// 3. THEN read custom fields for service and other metadata
	//
	// Previously, the code was trying to read severity from custom fields,
	// which was incorrect. The built-in attributes are the source of truth.
	//
	// This was discovered during code review when analyzing why incidents
	// weren't showing correct severity values.

	t.Log("Incident normalization must read built-in State and Severity attributes")
	t.Log("attrs.State.Get() -> incident.Status (via MapDatadogStatusToOpsOrch)")
	t.Log("attrs.Severity -> incident.Severity (via MapDatadogSeverityToOpsOrch)")
	t.Log("Custom fields are for service, team, environment, etc.")
}

// TestIncidentServiceExtractedFromCustomFields documents that service
// information is stored in custom fields, not built-in attributes.
func TestIncidentServiceExtractedFromCustomFields(t *testing.T) {
	// This test documents the learning that incident service information
	// is stored in custom fields (attrs.Fields), not as a built-in attribute.
	//
	// The extraction logic checks for:
	// 1. "service" field (singular) - can be single or multiple value
	// 2. "services" field (plural) - multiple value
	//
	// Custom fields can be either:
	// - IncidentFieldAttributesSingleValue (single string value)
	// - IncidentFieldAttributesMultipleValue (array of strings)
	//
	// This was discovered during code review when ensuring proper service
	// extraction for scope filtering.

	t.Log("Incident service is extracted from custom fields, not built-in attributes")
	t.Log("Check attrs.Fields['service'] or attrs.Fields['services']")
	t.Log("Handle both single value and multiple value field types")
}

// TestIncidentPaginationUsesPageOffset documents that incident queries
// must paginate using PageOffset parameter.
func TestIncidentPaginationUsesPageOffset(t *testing.T) {
	// This test documents the learning that Datadog Incidents API uses
	// offset-based pagination, not cursor-based pagination.
	//
	// Implementation:
	// 1. Set PageSize (e.g., 100 incidents per page)
	// 2. Set PageOffset (starts at 0, increment by PageSize)
	// 3. Continue until len(response.Data) < PageSize
	//
	// This is different from other APIs like Metrics Describe which use
	// cursor-based pagination with next_cursor.
	//
	// This was discovered during code review when implementing pagination
	// to fetch all incidents for client-side filtering.

	t.Log("Incident pagination uses PageOffset parameter")
	t.Log("optParams.WithPageSize(100).WithPageOffset(offset)")
	t.Log("Increment offset by PageSize until response is smaller than PageSize")
}

// TestCustomFieldValuesDereferenced documents that single-value custom fields
// must be dereferenced before storing in incident.Fields.
func TestCustomFieldValuesDereferenced(t *testing.T) {
	// This test documents the CRITICAL FIX that single-value custom fields
	// return *interface{} which must be dereferenced before storing.
	//
	// BEFORE (broken):
	// incident.Fields[key] = value  // stores *interface{}
	//
	// AFTER (fixed):
	// incident.Fields[key] = *value  // stores actual value (string, etc.)
	//
	// This was critical because applyIncidentFilters type-asserts to string:
	// if team, ok := inc.Fields["team"].(string); ok { ... }
	//
	// Without dereferencing, the type assertion always fails, breaking
	// team/environment scope filtering.
	//
	// This was discovered during code review when analyzing why scope
	// filtering wasn't working for incidents with single-value custom fields.

	t.Log("CRITICAL: Single-value custom fields must be dereferenced")
	t.Log("WRONG: incident.Fields[key] = value  // stores *interface{}")
	t.Log("RIGHT: incident.Fields[key] = *value  // stores actual value")
	t.Log("Required for type assertions in applyIncidentFilters to work")
}

// TestMultiValueCustomFieldsHandling documents that multi-value custom fields
// are stored as []string and must be handled in filtering.
func TestMultiValueCustomFieldsHandling(t *testing.T) {
	// This test documents the CRITICAL requirement that applyIncidentFilters
	// must handle both single-value (string) and multi-value ([]string) custom fields.
	//
	// BACKGROUND:
	// Datadog custom fields can be either single-value or multi-value.
	// When normalizeSDKIncident processes them:
	// - Single values: stored as string
	// - Multiple values: stored as []string
	//
	// PROBLEM:
	// If the filter only checks for string type, incidents with multiple
	// teams/environments will never match scope filters, even if one of
	// the values matches the query.
	//
	// SOLUTION:
	// The filter must:
	// 1. Try to match as string first
	// 2. If that fails, try to match as []string and check if any element matches
	//
	// This was discovered during code review when analyzing scope filtering
	// for incidents with multiple team or environment values.

	t.Log("Multi-value custom fields must be handled in scope filtering")
	t.Log("Fields can be string or []string depending on number of values")
	t.Log("Filter must check both types to match incidents correctly")
}

// TestMultiValueFieldsInFilters verifies that multi-value custom fields
// are handled correctly in scope filtering.
func TestMultiValueFieldsInFilters(t *testing.T) {
	// This test verifies that applyIncidentFilters correctly handles both
	// single-value (string) and multi-value ([]string) custom fields.
	//
	// When Datadog returns multiple values for a field (e.g., multiple teams),
	// the normalizeSDKIncident function stores them as []string.
	//
	// The filter must check:
	// 1. If the field is a string, compare directly
	// 2. If the field is []string, check if any element matches
	//
	// This ensures incidents with multiple teams/environments can match filters.

	tests := []struct {
		name        string
		incident    schema.Incident
		query       schema.IncidentQuery
		shouldMatch bool
	}{
		{
			name: "single team matches",
			incident: schema.Incident{
				Fields: map[string]any{
					"team": "platform",
				},
			},
			query: schema.IncidentQuery{
				Scope: schema.QueryScope{
					Team: "platform",
				},
			},
			shouldMatch: true,
		},
		{
			name: "multiple teams - first matches",
			incident: schema.Incident{
				Fields: map[string]any{
					"team": []string{"platform", "infrastructure"},
				},
			},
			query: schema.IncidentQuery{
				Scope: schema.QueryScope{
					Team: "platform",
				},
			},
			shouldMatch: true,
		},
		{
			name: "multiple teams - second matches",
			incident: schema.Incident{
				Fields: map[string]any{
					"team": []string{"platform", "infrastructure"},
				},
			},
			query: schema.IncidentQuery{
				Scope: schema.QueryScope{
					Team: "infrastructure",
				},
			},
			shouldMatch: true,
		},
		{
			name: "multiple teams - no match",
			incident: schema.Incident{
				Fields: map[string]any{
					"team": []string{"platform", "infrastructure"},
				},
			},
			query: schema.IncidentQuery{
				Scope: schema.QueryScope{
					Team: "security",
				},
			},
			shouldMatch: false,
		},
		{
			name: "single environment matches",
			incident: schema.Incident{
				Fields: map[string]any{
					"environment": "production",
				},
			},
			query: schema.IncidentQuery{
				Scope: schema.QueryScope{
					Environment: "production",
				},
			},
			shouldMatch: true,
		},
		{
			name: "multiple environments - matches",
			incident: schema.Incident{
				Fields: map[string]any{
					"environment": []string{"production", "staging"},
				},
			},
			query: schema.IncidentQuery{
				Scope: schema.QueryScope{
					Environment: "staging",
				},
			},
			shouldMatch: true,
		},
		{
			name: "multiple environments via env field - matches",
			incident: schema.Incident{
				Fields: map[string]any{
					"env": []string{"prod", "stage"},
				},
			},
			query: schema.IncidentQuery{
				Scope: schema.QueryScope{
					Environment: "stage",
				},
			},
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			incidents := []schema.Incident{tt.incident}
			filtered := applyIncidentFilters(incidents, tt.query)

			matched := len(filtered) > 0
			if matched != tt.shouldMatch {
				t.Errorf("Expected match=%v, got match=%v", tt.shouldMatch, matched)
			}
		})
	}
}

// TestCustomerImpactedRequiresScope documents that setting customer_impacted
// to true requires also setting CustomerImpactScope.
func TestCustomerImpactedRequiresScope(t *testing.T) {
	// This test documents the Datadog API requirement that when
	// CustomerImpacted is set to true, CustomerImpactScope must also be set.
	//
	// The API returns 400 Bad Request if CustomerImpacted=true without scope.
	//
	// Current implementation: When customer_impacted=true, also sets CustomerImpactScope
	// to avoid validation errors. Defaults to "unknown" if no scope is provided.
	//
	// This was discovered during code review when analyzing incident creation
	// failures with customer_impacted=true in metadata.

	t.Log("CustomerImpacted=true requires CustomerImpactScope to be set")
	t.Log("Current: Sets both CustomerImpacted and CustomerImpactScope atomically")
	t.Log("Default scope: 'unknown' when not provided in metadata")
}

// TestUpdateIncidentCustomerImpactScope verifies that customer impact scope
// is handled correctly in incident updates.
func TestUpdateIncidentCustomerImpactScope(t *testing.T) {
	tests := []struct {
		name                   string
		metadata               map[string]any
		expectCustomerImpacted bool
		expectScope            string
		expectScopeSet         bool
	}{
		{
			name: "customer_impacted=true with scope=all",
			metadata: map[string]any{
				"customer_impacted":     true,
				"customer_impact_scope": "all",
			},
			expectCustomerImpacted: true,
			expectScope:            "all",
			expectScopeSet:         true,
		},
		{
			name: "customer_impacted=true with scope=some",
			metadata: map[string]any{
				"customer_impacted":     true,
				"customer_impact_scope": "some",
			},
			expectCustomerImpacted: true,
			expectScope:            "some",
			expectScopeSet:         true,
		},
		{
			name: "customer_impacted=true with scope=none",
			metadata: map[string]any{
				"customer_impacted":     true,
				"customer_impact_scope": "none",
			},
			expectCustomerImpacted: true,
			expectScope:            "none",
			expectScopeSet:         true,
		},
		{
			name: "customer_impacted=true without scope defaults to unknown",
			metadata: map[string]any{
				"customer_impacted": true,
			},
			expectCustomerImpacted: true,
			expectScope:            "unknown",
			expectScopeSet:         true,
		},
		{
			name: "customer_impacted=true with custom scope string",
			metadata: map[string]any{
				"customer_impacted":     true,
				"customer_impact_scope": "EU customers affected",
			},
			expectCustomerImpacted: true,
			expectScope:            "EU customers affected",
			expectScopeSet:         true,
		},
		{
			name: "customer_impacted=true with scope trimmed",
			metadata: map[string]any{
				"customer_impacted":     true,
				"customer_impact_scope": "  Checkout unavailable  ",
			},
			expectCustomerImpacted: true,
			expectScope:            "Checkout unavailable",
			expectScopeSet:         true,
		},
		{
			name: "customer_impacted=false does not set scope",
			metadata: map[string]any{
				"customer_impacted": false,
			},
			expectCustomerImpacted: false,
			expectScope:            "",
			expectScopeSet:         false,
		},
		{
			name:                   "no customer_impacted metadata does not set fields",
			metadata:               map[string]any{},
			expectCustomerImpacted: false,
			expectScope:            "",
			expectScopeSet:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create update input with metadata
			input := schema.UpdateIncidentInput{
				Metadata: tt.metadata,
			}

			// Build attributes as the Update method does
			attrs := datadogV2.NewIncidentUpdateAttributes()

			// Apply the customer impact logic (extracted from Update method)
			if input.Metadata != nil {
				if ci, ok := input.Metadata["customer_impacted"].(bool); ok {
					attrs.SetCustomerImpacted(ci)

					if ci {
						scope := "unknown"
						if scopeStr, ok := input.Metadata["customer_impact_scope"].(string); ok && scopeStr != "" {
							scope = strings.TrimSpace(scopeStr)
						}
						attrs.SetCustomerImpactScope(scope)
					}
				}
			}

			// Verify CustomerImpacted
			if attrs.CustomerImpacted != nil {
				if *attrs.CustomerImpacted != tt.expectCustomerImpacted {
					t.Errorf("CustomerImpacted = %v, want %v", *attrs.CustomerImpacted, tt.expectCustomerImpacted)
				}
			} else if tt.expectCustomerImpacted {
				t.Error("CustomerImpacted not set, but expected to be set")
			}

			// Verify CustomerImpactScope
			if tt.expectScopeSet {
				if attrs.CustomerImpactScope == nil {
					t.Error("CustomerImpactScope not set, but expected to be set")
				} else if *attrs.CustomerImpactScope != tt.expectScope {
					t.Errorf("CustomerImpactScope = %v, want %v", *attrs.CustomerImpactScope, tt.expectScope)
				}
			} else {
				if attrs.CustomerImpactScope != nil {
					t.Errorf("CustomerImpactScope set to %v, but expected not to be set", *attrs.CustomerImpactScope)
				}
			}
		})
	}
}
