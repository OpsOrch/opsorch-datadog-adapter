package incident

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"

	coreincident "github.com/opsorch/opsorch-core/incident"
	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-datadog-adapter/common"
)

// ProviderName is the registry key for the Datadog incident adapter.
const ProviderName = "datadog"

// AdapterVersion and RequiresCore express compatibility.
const (
	AdapterVersion = "0.1.0"
	RequiresCore   = ">=0.1.0"
)

// DatadogProvider implements the incident.Provider interface using the Datadog SDK.
type DatadogProvider struct {
	apiClient *datadog.APIClient
	config    common.Config
}

// New constructs the provider from decrypted config using the SDK.
func New(cfg map[string]any) (coreincident.Provider, error) {
	parsedCfg := common.ParseConfig(cfg)
	if err := common.ValidateConfig(parsedCfg); err != nil {
		return nil, err
	}

	apiClient := common.NewAPIClient(parsedCfg)
	return &DatadogProvider{
		apiClient: apiClient,
		config:    parsedCfg,
	}, nil
}

func init() {
	_ = coreincident.RegisterProvider(ProviderName, New)
}

// Query returns incidents matching the IncidentQuery criteria.
func (p *DatadogProvider) Query(ctx context.Context, query schema.IncidentQuery) ([]schema.Incident, error) {
	// Create API context with authentication
	apiCtx := common.NewAPIContext(p.config)

	// Note: Datadog v2 incidents API doesn't support filtering by query, status, severity, or scope
	// We'll need to filter results client-side, so fetch all incidents across pages

	allIncidents := make([]schema.Incident, 0)
	pageSize := int64(100) // Fetch 100 per page for efficiency
	offset := int64(0)
	retryCfg := common.DefaultRetryConfig()

	for {
		// Build SDK optional parameters for this page
		optParams := datadogV2.NewListIncidentsOptionalParameters()
		optParams.WithPageSize(pageSize)
		optParams.WithPageOffset(offset)

		// Call SDK with retry logic for rate limits
		var resp datadogV2.IncidentsResponse

		err := common.RetryWithBackoff(ctx, retryCfg, func() error {
			incidentsApi := datadogV2.NewIncidentsApi(p.apiClient)
			r, httpResp, err := incidentsApi.ListIncidents(apiCtx, *optParams)
			if err != nil {
				return common.HandleSDKError(err, p.config)
			}
			defer httpResp.Body.Close()
			resp = r
			return nil
		})

		if err != nil {
			return nil, err
		}

		// Transform and add incidents from this page
		pageIncidents := normalizeSDKIncidentListResponse(resp, p.config.Source)
		allIncidents = append(allIncidents, pageIncidents...)

		// Check if there are more pages
		if len(resp.Data) < int(pageSize) {
			// Last page (partial or empty)
			break
		}

		// Check if we've fetched enough for filtering
		// Continue fetching if we need more for client-side filtering
		offset += pageSize

		// Safety limit to prevent infinite loops
		if offset >= 10000 {
			break
		}
	}

	// Apply client-side filters
	incidents := applyIncidentFilters(allIncidents, query)

	return incidents, nil
}

// Get returns a specific incident by ID.
func (p *DatadogProvider) Get(ctx context.Context, id string) (schema.Incident, error) {
	// Create API context with authentication
	apiCtx := common.NewAPIContext(p.config)

	// Call SDK
	incidentsApi := datadogV2.NewIncidentsApi(p.apiClient)
	resp, httpResp, err := incidentsApi.GetIncident(apiCtx, id)
	if err != nil {
		return schema.Incident{}, common.HandleSDKError(err, p.config)
	}
	defer httpResp.Body.Close()

	return normalizeSDKIncident(resp.Data, p.config.Source), nil
}

// Create creates a new incident in Datadog.
func (p *DatadogProvider) Create(ctx context.Context, in schema.CreateIncidentInput) (schema.Incident, error) {
	// Create API context with authentication
	apiCtx := common.NewAPIContext(p.config)

	// Build SDK request
	// Note: Datadog requires CustomerImpactScope when CustomerImpacted is true
	// Default to false if not provided in metadata
	customerImpacted := false
	if in.Metadata != nil {
		if ci, ok := in.Metadata["customer_impacted"].(bool); ok {
			customerImpacted = ci
		}
	}

	attrs := datadogV2.NewIncidentCreateAttributes(
		customerImpacted,
		in.Title,
	)

	// When customer_impacted is true, we must also set CustomerImpactScope
	if customerImpacted {
		// Extract scope from metadata or use default
		// CustomerImpactScope is a free-form string describing the impact
		scope := "unknown" // Default to "unknown" if not provided

		if in.Metadata != nil {
			if scopeStr, ok := in.Metadata["customer_impact_scope"].(string); ok && scopeStr != "" {
				// Use the provided scope string as-is (trim whitespace)
				scope = strings.TrimSpace(scopeStr)
			}
		}

		// Set the scope atomically with customer_impacted
		attrs.SetCustomerImpactScope(scope)
	}

	// Note: Datadog Incidents API v2 has limitations:
	// - State and Severity cannot be set via Create API (requires complex custom field types)
	// - They can only be set via Update API or Datadog UI
	// - Service, Description, and other fields are stored as custom fields
	//
	// For now, we create incidents with basic attributes only.
	// Callers should use Update() immediately after Create() to set additional fields.

	data := datadogV2.NewIncidentCreateData(*attrs, datadogV2.INCIDENTTYPE_INCIDENTS)
	body := datadogV2.NewIncidentCreateRequest(*data)

	// Call SDK
	incidentsApi := datadogV2.NewIncidentsApi(p.apiClient)
	resp, httpResp, err := incidentsApi.CreateIncident(apiCtx, *body)
	if err != nil {
		return schema.Incident{}, common.HandleSDKError(err, p.config)
	}
	defer httpResp.Body.Close()

	return normalizeSDKIncident(resp.Data, p.config.Source), nil
}

// Update modifies an incident in Datadog.
func (p *DatadogProvider) Update(ctx context.Context, id string, in schema.UpdateIncidentInput) (schema.Incident, error) {
	// Create API context with authentication
	apiCtx := common.NewAPIContext(p.config)

	// Build SDK request
	attrs := datadogV2.NewIncidentUpdateAttributes()

	if in.Title != nil {
		attrs.SetTitle(*in.Title)
	}

	// Note: Datadog Incidents API v2 has limitations:
	// - State and Severity cannot be set via simple Update API
	// - They require complex custom field types not exposed in the SDK
	// - Service, Description, and other fields are stored as custom fields
	//
	// The SDK's IncidentUpdateAttributes only supports:
	// - Title
	// - CustomerImpacted
	// - CustomerImpactScope
	// - CustomerImpactStart/End
	// - Detected
	// - NotificationHandles
	//
	// For now, we update incidents with supported attributes only.
	// Status, Severity, Service, and custom fields cannot be updated via this API.

	// Handle customer impact with required scope
	// Datadog API requires CustomerImpactScope when CustomerImpacted is true
	if in.Metadata != nil {
		if ci, ok := in.Metadata["customer_impacted"].(bool); ok {
			attrs.SetCustomerImpacted(ci)

			// When customer_impacted is true, we must also set CustomerImpactScope
			if ci {
				// Extract scope from metadata or use default
				// CustomerImpactScope is a free-form string describing the impact
				scope := "unknown" // Default to "unknown" if not provided

				if scopeStr, ok := in.Metadata["customer_impact_scope"].(string); ok && scopeStr != "" {
					// Use the provided scope string as-is (trim whitespace)
					scope = strings.TrimSpace(scopeStr)
				}

				// Set the scope atomically with customer_impacted
				attrs.SetCustomerImpactScope(scope)
			}
			// When customer_impacted is false, we don't set scope (not required by API)
		}
	}

	data := datadogV2.NewIncidentUpdateData(id, datadogV2.INCIDENTTYPE_INCIDENTS)
	data.SetAttributes(*attrs)
	body := datadogV2.NewIncidentUpdateRequest(*data)

	// Call SDK
	incidentsApi := datadogV2.NewIncidentsApi(p.apiClient)
	resp, httpResp, err := incidentsApi.UpdateIncident(apiCtx, id, *body)
	if err != nil {
		return schema.Incident{}, common.HandleSDKError(err, p.config)
	}
	defer httpResp.Body.Close()

	return normalizeSDKIncident(resp.Data, p.config.Source), nil
}

// GetTimeline returns the timeline entries for an incident.
// Note: Datadog SDK v2 doesn't expose timeline API directly, so we use todos as timeline entries.
func (p *DatadogProvider) GetTimeline(ctx context.Context, id string) ([]schema.TimelineEntry, error) {
	// Create API context with authentication
	apiCtx := common.NewAPIContext(p.config)

	// Call SDK to get todos (which serve as timeline entries)
	incidentsApi := datadogV2.NewIncidentsApi(p.apiClient)
	resp, httpResp, err := incidentsApi.ListIncidentTodos(apiCtx, id)
	if err != nil {
		return nil, common.HandleSDKError(err, p.config)
	}
	defer httpResp.Body.Close()

	// Transform SDK response
	return normalizeSDKTodosToTimeline(resp, id), nil
}

// AppendTimeline adds a timeline entry to an incident.
// Note: Datadog SDK v2 doesn't expose timeline API directly, so we create a todo as a timeline entry.
func (p *DatadogProvider) AppendTimeline(ctx context.Context, id string, entry schema.TimelineAppendInput) error {
	// Create API context with authentication
	apiCtx := common.NewAPIContext(p.config)

	// Build SDK request - create a todo with the timeline entry content
	// Todos require assignees (empty array is fine) and content
	assignees := []datadogV2.IncidentTodoAssignee{}
	attrs := datadogV2.NewIncidentTodoAttributes(assignees, entry.Body)

	// Mark as completed immediately since this is a timeline note, not an actual todo
	completedStr := entry.At.Format("2006-01-02T15:04:05Z")
	attrs.SetCompleted(completedStr)

	data := datadogV2.NewIncidentTodoCreateData(
		*attrs,
		datadogV2.INCIDENTTODOTYPE_INCIDENT_TODOS,
	)

	body := datadogV2.NewIncidentTodoCreateRequest(*data)

	// Call SDK
	incidentsApi := datadogV2.NewIncidentsApi(p.apiClient)
	_, httpResp, err := incidentsApi.CreateIncidentTodo(apiCtx, id, *body)
	if err != nil {
		return common.HandleSDKError(err, p.config)
	}
	defer httpResp.Body.Close()

	return nil
}

// Status mapping functions

// MapOpsOrchStatusToDatadog converts OpsOrch status to Datadog incident state.
func MapOpsOrchStatusToDatadog(status string) string {
	switch strings.ToLower(status) {
	case "open", "triggered":
		return "active"
	case "investigating", "acknowledged":
		return "stable"
	case "resolved", "closed":
		return "resolved"
	default:
		return "active"
	}
}

// MapDatadogStatusToOpsOrch converts Datadog incident state to OpsOrch status.
func MapDatadogStatusToOpsOrch(state string) string {
	switch strings.ToLower(state) {
	case "active":
		return "open"
	case "stable":
		return "investigating"
	case "resolved":
		return "resolved"
	default:
		return strings.ToLower(state)
	}
}

// Severity mapping functions

// MapOpsOrchSeverityToDatadog converts OpsOrch severity to Datadog severity.
func MapOpsOrchSeverityToDatadog(severity string) string {
	switch strings.ToLower(severity) {
	case "critical", "p1", "sev1":
		return "SEV-1"
	case "high", "p2", "sev2":
		return "SEV-2"
	case "medium", "p3", "sev3":
		return "SEV-3"
	case "low", "p4", "sev4":
		return "SEV-4"
	case "info", "p5", "sev5":
		return "SEV-5"
	default:
		return "SEV-3" // Default to medium
	}
}

// MapDatadogSeverityToOpsOrch converts Datadog severity to OpsOrch severity.
func MapDatadogSeverityToOpsOrch(severity string) string {
	switch strings.ToUpper(severity) {
	case "SEV-1":
		return "critical"
	case "SEV-2":
		return "high"
	case "SEV-3":
		return "medium"
	case "SEV-4":
		return "low"
	case "SEV-5":
		return "info"
	default:
		return "medium" // Default to medium
	}
}

// normalizeSDKIncidentListResponse converts SDK incidents to OpsOrch incidents.
func normalizeSDKIncidentListResponse(resp datadogV2.IncidentsResponse, source string) []schema.Incident {
	incidents := make([]schema.Incident, 0, len(resp.Data))
	for _, data := range resp.Data {
		incidents = append(incidents, normalizeSDKIncident(data, source))
	}
	return incidents
}

// normalizeSDKIncident converts a single SDK incident to an OpsOrch incident.
func normalizeSDKIncident(data datadogV2.IncidentResponseData, source string) schema.Incident {
	incident := schema.Incident{
		Fields: make(map[string]any),
		Metadata: map[string]any{
			"source": source,
		},
	}

	// Set ID
	incident.ID = data.Id

	if data.Attributes == nil {
		return incident
	}

	attrs := data.Attributes

	// Set title
	incident.Title = attrs.Title

	// CRITICAL FIX: Read built-in State and Severity attributes first
	// These are the actual incident state/severity, not custom fields
	if attrs.State.IsSet() {
		state := attrs.State.Get()
		if state != nil {
			incident.Status = MapDatadogStatusToOpsOrch(*state)
		}
	}

	if attrs.Severity != nil {
		incident.Severity = MapDatadogSeverityToOpsOrch(string(*attrs.Severity))
	}

	// Extract service from fields (services are stored as custom fields in Datadog)
	if len(attrs.Fields) > 0 {
		// Look for service in custom fields
		if serviceField, ok := attrs.Fields["service"]; ok {
			if serviceField.IncidentFieldAttributesMultipleValue != nil {
				values := serviceField.IncidentFieldAttributesMultipleValue.Value.Get()
				if values != nil && len(*values) > 0 {
					incident.Service = (*values)[0]
				}
			} else if serviceField.IncidentFieldAttributesSingleValue != nil {
				// Value is NullableString
				if serviceField.IncidentFieldAttributesSingleValue.Value.IsSet() {
					value := serviceField.IncidentFieldAttributesSingleValue.Value.Get()
					if value != nil {
						incident.Service = *value
					}
				}
			}
		}

		// Also check for services field (plural)
		if servicesField, ok := attrs.Fields["services"]; ok {
			if servicesField.IncidentFieldAttributesMultipleValue != nil {
				values := servicesField.IncidentFieldAttributesMultipleValue.Value.Get()
				if values != nil && len(*values) > 0 {
					incident.Service = (*values)[0]
				}
			}
		}

		// Copy all custom fields to Fields map
		for key, field := range attrs.Fields {
			if field.IncidentFieldAttributesMultipleValue != nil {
				values := field.IncidentFieldAttributesMultipleValue.Value.Get()
				if values != nil && len(*values) > 0 {
					if len(*values) == 1 {
						incident.Fields[key] = (*values)[0]
					} else {
						incident.Fields[key] = *values
					}
				}
			} else if field.IncidentFieldAttributesSingleValue != nil {
				value := field.IncidentFieldAttributesSingleValue.Value.Get()
				if value != nil {
					// CRITICAL FIX: Dereference the pointer to store actual value
					// This allows applyIncidentFilters to type-assert to string
					incident.Fields[key] = *value
				}
			}
		}
	}

	// Set customer impacted
	if attrs.CustomerImpacted != nil {
		incident.Metadata["customer_impacted"] = *attrs.CustomerImpacted
	}

	// Set public ID
	if attrs.PublicId != nil {
		incident.Metadata["public_id"] = *attrs.PublicId
	}

	// Parse timestamps
	if attrs.Created != nil {
		incident.CreatedAt = *attrs.Created
	}
	if attrs.Modified != nil {
		incident.UpdatedAt = *attrs.Modified
	}
	if attrs.Detected.IsSet() {
		detected := attrs.Detected.Get()
		if detected != nil {
			incident.Metadata["detected"] = *detected
		}
	}
	if attrs.Resolved.IsSet() {
		resolved := attrs.Resolved.Get()
		if resolved != nil {
			incident.Metadata["resolved"] = *resolved
		}
	}

	// Default severity if not set
	if incident.Severity == "" {
		incident.Severity = "medium"
	}

	// Default status if not set
	if incident.Status == "" {
		incident.Status = "open"
	}

	return incident
}

// applyIncidentFilters applies all client-side filters to incidents.
func applyIncidentFilters(incidents []schema.Incident, query schema.IncidentQuery) []schema.Incident {
	filtered := make([]schema.Incident, 0, len(incidents))

	for _, inc := range incidents {
		// Filter by query (search in title)
		if query.Query != "" {
			if !strings.Contains(strings.ToLower(inc.Title), strings.ToLower(query.Query)) {
				continue
			}
		}

		// Filter by status (inc.Status is already normalized to OpsOrch values)
		if len(query.Statuses) > 0 {
			found := false
			for _, status := range query.Statuses {
				if strings.EqualFold(inc.Status, status) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by severity (inc.Severity is already normalized to OpsOrch values)
		if len(query.Severities) > 0 {
			found := false
			for _, severity := range query.Severities {
				if strings.EqualFold(inc.Severity, severity) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by scope - service
		if query.Scope.Service != "" {
			if inc.Service != query.Scope.Service {
				continue
			}
		}

		// Filter by scope - team
		if query.Scope.Team != "" {
			// Check if team is in metadata or fields
			teamFound := false

			// Check metadata (string)
			if team, ok := inc.Metadata["team"].(string); ok && team == query.Scope.Team {
				teamFound = true
			}

			// Check fields (can be string or []string)
			if !teamFound {
				if team, ok := inc.Fields["team"].(string); ok && team == query.Scope.Team {
					teamFound = true
				} else if teams, ok := inc.Fields["team"].([]string); ok {
					// Check if any team in the array matches
					for _, team := range teams {
						if team == query.Scope.Team {
							teamFound = true
							break
						}
					}
				}
			}

			if !teamFound {
				continue
			}
		}

		// Filter by scope - environment
		if query.Scope.Environment != "" {
			// Check if environment is in metadata or fields
			envFound := false

			// Check metadata (string)
			if env, ok := inc.Metadata["environment"].(string); ok && env == query.Scope.Environment {
				envFound = true
			}

			// Check fields (can be string or []string)
			if !envFound {
				// Check "environment" field
				if env, ok := inc.Fields["environment"].(string); ok && env == query.Scope.Environment {
					envFound = true
				} else if envs, ok := inc.Fields["environment"].([]string); ok {
					// Check if any environment in the array matches
					for _, env := range envs {
						if env == query.Scope.Environment {
							envFound = true
							break
						}
					}
				}

				// Also check "env" field (shorthand)
				if !envFound {
					if env, ok := inc.Fields["env"].(string); ok && env == query.Scope.Environment {
						envFound = true
					} else if envs, ok := inc.Fields["env"].([]string); ok {
						// Check if any env in the array matches
						for _, env := range envs {
							if env == query.Scope.Environment {
								envFound = true
								break
							}
						}
					}
				}
			}

			if !envFound {
				continue
			}
		}

		filtered = append(filtered, inc)
	}

	// Apply limit
	if query.Limit > 0 && len(filtered) > query.Limit {
		filtered = filtered[:query.Limit]
	}

	return filtered
}

// normalizeSDKTodosToTimeline converts SDK todos to OpsOrch timeline entries.
// Note: Since Datadog SDK v2 doesn't expose timeline API directly, we use todos as timeline entries.
func normalizeSDKTodosToTimeline(resp datadogV2.IncidentTodoListResponse, incidentID string) []schema.TimelineEntry {
	entries := make([]schema.TimelineEntry, 0, len(resp.Data))

	for _, todo := range resp.Data {
		if todo.Attributes == nil {
			continue
		}

		attrs := todo.Attributes
		entry := schema.TimelineEntry{
			ID:         todo.Id,
			IncidentID: incidentID,
			Kind:       "todo",
			Metadata:   make(map[string]any),
		}

		// Set body/content
		entry.Body = attrs.Content

		// Set timestamp - use completed time if available, otherwise use due date
		// Note: Completed and DueDate are strings in ISO format, not time.Time
		if completed := attrs.Completed.Get(); completed != nil && *completed != "" {
			// Parse the completed timestamp
			if t, err := parseDatadogTimestamp(*completed); err == nil {
				entry.At = t
			}
		} else if dueDate := attrs.DueDate.Get(); dueDate != nil && *dueDate != "" {
			// Parse the due date
			if t, err := parseDatadogTimestamp(*dueDate); err == nil {
				entry.At = t
			}
		}

		// Add assignees to metadata
		if len(attrs.Assignees) > 0 {
			entry.Metadata["assignees"] = attrs.Assignees
		}

		// Add completed status
		if completed := attrs.Completed.Get(); completed != nil && *completed != "" {
			entry.Metadata["completed"] = true
		} else {
			entry.Metadata["completed"] = false
		}

		entries = append(entries, entry)
	}

	return entries
}

// parseDatadogTimestamp parses a Datadog timestamp string to time.Time
func parseDatadogTimestamp(ts string) (time.Time, error) {
	// Try RFC3339 format first
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t, nil
	}
	// Try other common formats
	if t, err := time.Parse("2006-01-02T15:04:05Z", ts); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", ts)
}
