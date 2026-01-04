package alert

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"

	corealert "github.com/opsorch/opsorch-core/alert"
	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-datadog-adapter/common"
)

// ProviderName is the registry key for the Datadog alert adapter.
const ProviderName = "datadog"

// AdapterVersion and RequiresCore express compatibility.
const (
	AdapterVersion = "0.1.0"
	RequiresCore   = ">=0.1.0"
)

// DatadogProvider implements the alert.Provider interface using the Datadog SDK.
type DatadogProvider struct {
	apiClient *datadog.APIClient
	config    common.Config
}

// New constructs the provider from decrypted config using the SDK.
func New(cfg map[string]any) (corealert.Provider, error) {
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
	_ = corealert.RegisterProvider(ProviderName, New)
}

// Query returns monitors matching the AlertQuery criteria.
func (p *DatadogProvider) Query(ctx context.Context, query schema.AlertQuery) ([]schema.Alert, error) {
	// Create API context with authentication
	apiCtx := common.NewAPIContext(p.config)

	// Build SDK optional parameters
	optParams := datadogV1.NewListMonitorsOptionalParameters()

	// Add search query
	if query.Query != "" {
		optParams.WithName(query.Query)
	}

	// Add status filter using GroupStates (not monitor_tags)
	if len(query.Statuses) > 0 {
		ddStates := make([]string, 0, len(query.Statuses))
		for _, status := range query.Statuses {
			ddState := MapOpsOrchStatusToDatadog(status)
			if ddState != "" {
				ddStates = append(ddStates, ddState)
			}
		}
		if len(ddStates) > 0 {
			optParams.WithGroupStates(strings.Join(ddStates, ","))
		}
	}

	// Add scope filters as monitor tags
	var tagFilters []string
	if query.Scope.Service != "" {
		tagFilters = append(tagFilters, fmt.Sprintf("service:%s", query.Scope.Service))
	}
	if query.Scope.Team != "" {
		tagFilters = append(tagFilters, fmt.Sprintf("team:%s", query.Scope.Team))
	}
	if query.Scope.Environment != "" {
		tagFilters = append(tagFilters, fmt.Sprintf("env:%s", query.Scope.Environment))
	}
	if len(tagFilters) > 0 {
		optParams.WithMonitorTags(strings.Join(tagFilters, ","))
	}

	// Add page size
	if query.Limit > 0 {
		optParams.WithPageSize(int32(query.Limit))
	}

	// Call SDK with retry logic for rate limits
	var monitors []datadogV1.Monitor
	retryCfg := common.DefaultRetryConfig()

	err := common.RetryWithBackoff(ctx, retryCfg, func() error {
		monitorsApi := datadogV1.NewMonitorsApi(p.apiClient)
		m, httpResp, err := monitorsApi.ListMonitors(apiCtx, *optParams)
		if err != nil {
			return common.HandleSDKError(err, p.config)
		}
		defer httpResp.Body.Close()
		monitors = m
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Transform SDK response
	alerts := normalizeSDKMonitorResponse(monitors, p.config.Source, p.config.Site)

	// Filter by severity if specified (Datadog doesn't have native severity filter)
	if len(query.Severities) > 0 {
		alerts = filterBySeverity(alerts, query.Severities)
	}

	return alerts, nil
}

// Get returns a specific monitor by ID.
func (p *DatadogProvider) Get(ctx context.Context, id string) (schema.Alert, error) {
	// Create API context with authentication
	apiCtx := common.NewAPIContext(p.config)

	// Parse ID to int64
	monitorID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return schema.Alert{}, fmt.Errorf("invalid monitor ID: %w", err)
	}

	// Call SDK
	monitorsApi := datadogV1.NewMonitorsApi(p.apiClient)
	monitor, httpResp, err := monitorsApi.GetMonitor(apiCtx, monitorID)
	if err != nil {
		return schema.Alert{}, common.HandleSDKError(err, p.config)
	}
	defer httpResp.Body.Close()

	return normalizeSDKMonitor(monitor, p.config.Source, p.config.Site), nil
}

// Status mapping functions

// MapOpsOrchStatusToDatadog converts OpsOrch status to Datadog monitor state.
func MapOpsOrchStatusToDatadog(status string) string {
	switch strings.ToLower(status) {
	case "firing", "alert":
		return "Alert"
	case "warning", "warn":
		return "Warn"
	case "ok", "resolved":
		return "OK"
	case "unknown", "no data", "nodata":
		return "No Data"
	case "skipped":
		return "Skipped"
	default:
		return ""
	}
}

// MapDatadogStatusToOpsOrch converts Datadog monitor state to OpsOrch status.
func MapDatadogStatusToOpsOrch(state string) string {
	switch strings.ToLower(state) {
	case "alert":
		return "firing"
	case "warn":
		return "warning"
	case "ok":
		return "ok"
	case "no data", "nodata":
		return "unknown"
	case "skipped":
		return "unknown"
	default:
		return strings.ToLower(state)
	}
}

// Severity mapping functions

// MapOpsOrchSeverityToPriority converts OpsOrch severity to Datadog priority.
func MapOpsOrchSeverityToPriority(severity string) string {
	switch strings.ToLower(severity) {
	case "critical", "p1":
		return "P1"
	case "high", "p2":
		return "P2"
	case "medium", "p3":
		return "P3"
	case "low", "p4":
		return "P4"
	case "info", "p5":
		return "P5"
	default:
		return "P3" // Default to medium
	}
}

// MapPriorityToOpsOrchSeverity converts Datadog priority to OpsOrch severity.
func MapPriorityToOpsOrchSeverity(priority string) string {
	switch strings.ToUpper(priority) {
	case "P1":
		return "critical"
	case "P2":
		return "high"
	case "P3":
		return "medium"
	case "P4":
		return "low"
	case "P5":
		return "info"
	default:
		return "medium" // Default to medium
	}
}

// normalizeSDKMonitorResponse converts SDK monitors to OpsOrch alerts.
func normalizeSDKMonitorResponse(monitors []datadogV1.Monitor, source string, site string) []schema.Alert {
	alerts := make([]schema.Alert, 0, len(monitors))
	for _, m := range monitors {
		alerts = append(alerts, normalizeSDKMonitor(m, source, site))
	}
	return alerts
}

// normalizeSDKMonitor converts a single SDK monitor to an OpsOrch alert.
func normalizeSDKMonitor(m datadogV1.Monitor, source string, site string) schema.Alert {
	alert := schema.Alert{
		Fields: make(map[string]any),
		Metadata: map[string]any{
			"source": source,
		},
	}

	// Set ID and URL
	if m.Id != nil {
		alert.ID = strconv.FormatInt(*m.Id, 10)
		alert.Metadata["monitor_id"] = *m.Id

		// Generate URL
		// Format: https://app.{site}/monitors/{monitor_id}
		if site == "" {
			site = "datadoghq.com"
		}
		alert.URL = fmt.Sprintf("https://app.%s/monitors/%d", site, *m.Id)
	}

	// Set title
	if m.Name != nil {
		alert.Title = *m.Name
	}

	// Set description
	if m.Message != nil {
		alert.Description = *m.Message
	}

	// Set status
	if m.OverallState != nil {
		alert.Status = MapDatadogStatusToOpsOrch(string(*m.OverallState))
	}

	// Set severity
	alert.Severity = extractSDKSeverity(m)

	// Set type
	alert.Metadata["type"] = string(m.Type)

	// Set query
	alert.Metadata["query"] = m.Query

	// Parse timestamps
	if m.Created != nil {
		alert.CreatedAt = *m.Created
	}
	if m.Modified != nil {
		alert.UpdatedAt = *m.Modified
	}

	// Parse tags
	if len(m.Tags) > 0 {
		alert.Metadata["tags"] = m.Tags
		for _, tag := range m.Tags {
			parts := strings.SplitN(tag, ":", 2)
			if len(parts) == 2 {
				if parts[0] == "service" {
					alert.Service = parts[1]
				}
				alert.Fields[parts[0]] = parts[1]
			}
		}
	}

	// Add thresholds to fields if present
	if m.Options != nil && m.Options.Thresholds != nil {
		alert.Fields["thresholds"] = m.Options.Thresholds
	}

	return alert
}

// extractSDKSeverity determines severity from SDK monitor priority or tags.
func extractSDKSeverity(m datadogV1.Monitor) string {
	// Check priority field first
	if m.Priority.IsSet() {
		priority := m.Priority.Get()
		if priority != nil {
			return MapPriorityToOpsOrchSeverity(fmt.Sprintf("P%d", *priority))
		}
	}

	// Check tags for priority
	for _, tag := range m.Tags {
		parts := strings.SplitN(tag, ":", 2)
		if len(parts) == 2 && parts[0] == "priority" {
			return MapPriorityToOpsOrchSeverity(parts[1])
		}
	}

	// Default based on monitor state
	if m.OverallState != nil {
		switch strings.ToLower(string(*m.OverallState)) {
		case "alert":
			return "high"
		case "warn":
			return "medium"
		default:
			return "info"
		}
	}

	return "info"
}

// filterBySeverity filters alerts to only include specified severities.
func filterBySeverity(alerts []schema.Alert, severities []string) []schema.Alert {
	sevSet := make(map[string]bool)
	for _, s := range severities {
		sevSet[strings.ToLower(s)] = true
		// Also add mapped priority
		priority := MapOpsOrchSeverityToPriority(s)
		sevSet[strings.ToLower(priority)] = true
	}

	filtered := make([]schema.Alert, 0)
	for _, a := range alerts {
		if sevSet[strings.ToLower(a.Severity)] {
			filtered = append(filtered, a)
		}
	}
	return filtered
}
