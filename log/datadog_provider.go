package log

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"

	corelog "github.com/opsorch/opsorch-core/log"
	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-datadog-adapter/common"
)

// ProviderName is the registry key for the Datadog log adapter.
const ProviderName = "datadog"

// AdapterVersion and RequiresCore express compatibility.
const (
	AdapterVersion = "0.1.0"
	RequiresCore   = ">=0.1.0"
)

// DatadogProvider implements the log.Provider interface using the Datadog SDK.
type DatadogProvider struct {
	apiClient *datadog.APIClient
	config    common.Config
}

// New constructs the provider from decrypted config using the SDK.
func New(cfg map[string]any) (corelog.Provider, error) {
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
	_ = corelog.RegisterProvider(ProviderName, New)
}

// Query searches logs in Datadog and returns normalized results.
func (p *DatadogProvider) Query(ctx context.Context, query schema.LogQuery) ([]schema.LogEntry, error) {
	// Create API context with authentication
	apiCtx := common.NewAPIContext(p.config)

	// Build the Datadog log query
	ddQuery := buildLogQuery(query)

	// Determine page size and total limit
	pageSize := int32(1000) // Datadog default page size
	totalLimit := query.Limit
	if totalLimit <= 0 {
		totalLimit = 100 // Default total limit (reasonable for LLM context)
	}

	// Collect all logs across pages
	allLogs := make([]schema.LogEntry, 0)
	var cursor *string
	retryCfg := common.DefaultRetryConfig()

	for {
		// Build SDK request for this page
		filter := datadogV2.NewLogsQueryFilter()
		filter.SetQuery(ddQuery)
		filter.SetFrom(query.Start.Format(time.RFC3339))
		filter.SetTo(query.End.Format(time.RFC3339))

		page := datadogV2.NewLogsListRequestPage()

		// Set page size (limit per request)
		currentPageSize := pageSize
		remaining := int32(totalLimit - len(allLogs))
		if remaining < pageSize {
			currentPageSize = remaining
		}
		page.SetLimit(currentPageSize)

		// Set cursor for pagination (if not first page)
		if cursor != nil {
			page.SetCursor(*cursor)
		}

		body := datadogV2.NewLogsListRequest()
		body.SetFilter(*filter)
		body.SetPage(*page)
		body.SetSort(datadogV2.LOGSSORT_TIMESTAMP_ASCENDING)

		// Call SDK with retry logic for rate limits
		var resp datadogV2.LogsListResponse

		err := common.RetryWithBackoff(ctx, retryCfg, func() error {
			logsApi := datadogV2.NewLogsApi(p.apiClient)
			optParams := datadogV2.NewListLogsOptionalParameters().WithBody(*body)
			r, httpResp, err := logsApi.ListLogs(apiCtx, *optParams)
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

		// Transform and add logs from this page
		pageLogs := normalizeSDKLogResponse(resp, p.config.Source)
		allLogs = append(allLogs, pageLogs...)

		// Check if we've reached the limit
		if len(allLogs) >= totalLimit {
			// Trim to exact limit if we exceeded it
			if len(allLogs) > totalLimit {
				allLogs = allLogs[:totalLimit]
			}
			break
		}

		// Check if there are more pages (cursor-based pagination)
		if resp.Meta != nil && resp.Meta.Page != nil && resp.Meta.Page.After != nil && *resp.Meta.Page.After != "" {
			cursor = resp.Meta.Page.After
		} else {
			// No more pages
			break
		}

		// Safety check: if this page returned no logs, stop
		if len(pageLogs) == 0 {
			break
		}
	}

	return allLogs, nil
}

// buildLogQuery constructs a Datadog log query string from a LogQuery.
func buildLogQuery(query schema.LogQuery) string {
	var parts []string

	if query.Expression != nil {
		// Add search term
		if query.Expression.Search != "" {
			parts = append(parts, query.Expression.Search)
		}

		// Add severity filter
		if len(query.Expression.SeverityIn) > 0 {
			severities := make([]string, 0, len(query.Expression.SeverityIn))
			for _, sev := range query.Expression.SeverityIn {
				ddStatus := mapSeverityToDatadog(sev)
				severities = append(severities, ddStatus)
			}
			parts = append(parts, fmt.Sprintf("status:(%s)", strings.Join(severities, " OR ")))
		}

		// Add expression filters
		for _, f := range query.Expression.Filters {
			parts = append(parts, formatLogFilter(f))
		}
	}

	// Add scope filters
	if query.Scope.Service != "" {
		parts = append(parts, fmt.Sprintf("service:%s", query.Scope.Service))
	}
	if query.Scope.Team != "" {
		parts = append(parts, fmt.Sprintf("team:%s", query.Scope.Team))
	}
	if query.Scope.Environment != "" {
		parts = append(parts, fmt.Sprintf("env:%s", query.Scope.Environment))
	}

	// Add metadata filters
	for key, value := range query.Metadata {
		if strVal, ok := value.(string); ok {
			parts = append(parts, fmt.Sprintf("%s:%s", key, strVal))
		}
	}

	if len(parts) == 0 {
		return "*" // Match all logs
	}

	return strings.Join(parts, " ")
}

// formatLogFilter formats a LogFilter as a Datadog query clause.
func formatLogFilter(f schema.LogFilter) string {
	switch f.Operator {
	case "=":
		return fmt.Sprintf("%s:%s", f.Field, f.Value)
	case "!=":
		return fmt.Sprintf("-%s:%s", f.Field, f.Value)
	case "contains":
		return fmt.Sprintf("%s:*%s*", f.Field, f.Value)
	case "regex":
		// Datadog doesn't support regex in the same way, use wildcard
		return fmt.Sprintf("%s:%s", f.Field, f.Value)
	default:
		return fmt.Sprintf("%s:%s", f.Field, f.Value)
	}
}

// mapSeverityToDatadog maps OpsOrch severity to Datadog status.
func mapSeverityToDatadog(severity string) string {
	switch strings.ToLower(severity) {
	case "critical", "emergency", "alert":
		return "critical"
	case "error":
		return "error"
	case "warning", "warn":
		return "warn"
	case "info", "notice":
		return "info"
	case "debug", "trace":
		return "debug"
	default:
		return severity
	}
}

// mapDatadogSeverityToOpsOrch maps Datadog status to OpsOrch severity.
func mapDatadogSeverityToOpsOrch(status string) string {
	switch strings.ToLower(status) {
	case "critical", "emergency", "alert":
		return "critical"
	case "error":
		return "error"
	case "warn", "warning":
		return "warning"
	case "info", "notice":
		return "info"
	case "debug", "trace":
		return "debug"
	default:
		return status
	}
}

// normalizeSDKLogResponse converts SDK log response to OpsOrch schema.
func normalizeSDKLogResponse(resp datadogV2.LogsListResponse, source string) []schema.LogEntry {
	entries := make([]schema.LogEntry, 0, len(resp.Data))

	for _, event := range resp.Data {
		if event.Attributes == nil {
			continue
		}

		attrs := event.Attributes
		entry := schema.LogEntry{
			Labels: make(map[string]string),
			Fields: make(map[string]any),
			Metadata: map[string]any{
				"source": source,
			},
		}

		// Set message
		if attrs.Message != nil {
			entry.Message = *attrs.Message
		}

		// Set status/severity
		if attrs.Status != nil {
			entry.Severity = mapDatadogSeverityToOpsOrch(*attrs.Status)
		}

		// Set service
		if attrs.Service != nil {
			entry.Service = *attrs.Service
		}

		// Set log ID
		if event.Id != nil {
			entry.Metadata["log_id"] = *event.Id
		}

		// Set host
		if attrs.Host != nil {
			entry.Metadata["host"] = *attrs.Host
		}

		// Set timestamp
		if attrs.Timestamp != nil {
			entry.Timestamp = *attrs.Timestamp
		}

		// Parse tags into labels
		if len(attrs.Tags) > 0 {
			entry.Metadata["tags"] = attrs.Tags
			for _, tag := range attrs.Tags {
				parts := strings.SplitN(tag, ":", 2)
				if len(parts) == 2 {
					entry.Labels[parts[0]] = parts[1]
				}
			}
		}

		// Copy attributes to fields
		for k, v := range attrs.Attributes {
			entry.Fields[k] = v
		}

		entries = append(entries, entry)
	}

	return entries
}
