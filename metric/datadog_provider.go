package metric

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"

	coremetric "github.com/opsorch/opsorch-core/metric"
	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-datadog-adapter/common"
)

// DatadogProvider implements the metric.Provider interface using the Datadog SDK.
type DatadogProvider struct {
	apiClient *datadog.APIClient
	config    common.Config
}

// New constructs the provider from decrypted config using the SDK.
func New(cfg map[string]any) (coremetric.Provider, error) {
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

// Query executes a metric query against Datadog and returns normalized results.
func (p *DatadogProvider) Query(ctx context.Context, query schema.MetricQuery) ([]schema.MetricSeries, error) {
	// Create API context with authentication
	apiCtx := common.NewAPIContext(p.config)

	// Build the Datadog query string with proper v2 format
	ddQuery := buildMetricQueryV2(query)

	// Build SDK request
	metricsQuery := datadogV2.NewMetricsTimeseriesQuery(datadogV2.METRICSDATASOURCE_METRICS, ddQuery)
	metricsQuery.SetName("query1")

	timeseriesQuery := datadogV2.MetricsTimeseriesQueryAsTimeseriesQuery(metricsQuery)

	attributes := datadogV2.NewTimeseriesFormulaRequestAttributes(
		query.Start.UnixMilli(),
		[]datadogV2.TimeseriesQuery{timeseriesQuery},
		query.End.UnixMilli(),
	)

	formulaRequest := datadogV2.NewTimeseriesFormulaRequest(
		*attributes,
		datadogV2.TIMESERIESFORMULAREQUESTTYPE_TIMESERIES_REQUEST,
	)

	body := datadogV2.NewTimeseriesFormulaQueryRequest(*formulaRequest)

	// Call SDK with retry logic for rate limits
	var resp datadogV2.TimeseriesFormulaQueryResponse
	retryCfg := common.DefaultRetryConfig()

	err := common.RetryWithBackoff(ctx, retryCfg, func() error {
		metricsApi := datadogV2.NewMetricsApi(p.apiClient)
		r, httpResp, err := metricsApi.QueryTimeseriesData(apiCtx, *body)
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

	// Transform SDK response to OpsOrch schema
	return normalizeSDKMetricResponse(resp, ddQuery, p.config.Source, p.config.Site), nil
}

// Describe lists available metrics from Datadog.
func (p *DatadogProvider) Describe(ctx context.Context, scope schema.QueryScope) ([]schema.MetricDescriptor, error) {
	// Create API context with authentication
	apiCtx := common.NewAPIContext(p.config)

	allDescriptors := make([]schema.MetricDescriptor, 0)
	var nextCursor *string = nil
	retryCfg := common.DefaultRetryConfig()

	for {
		// Build SDK optional parameters for this page
		optParams := datadogV2.NewListTagConfigurationsOptionalParameters()
		optParams.WithWindowSeconds(3600) // Look at last hour of data

		if scope.Service != "" {
			optParams.WithFilterConfigured(true)
			optParams.WithFilterTagsConfigured(fmt.Sprintf("service:%s", scope.Service))
		}

		if nextCursor != nil {
			optParams.WithPageCursor(*nextCursor)
		}

		// Call SDK with retry logic for rate limits
		var resp datadogV2.MetricsAndMetricTagConfigurationsResponse

		err := common.RetryWithBackoff(ctx, retryCfg, func() error {
			metricsApi := datadogV2.NewMetricsApi(p.apiClient)
			r, httpResp, err := metricsApi.ListTagConfigurations(apiCtx, *optParams)
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

		// Transform and add metrics from this page
		pageDescriptors := normalizeSDKMetricsListResponse(resp, p.config.Source, p.config.Site)
		allDescriptors = append(allDescriptors, pageDescriptors...)

		// Check if there are more pages
		if resp.Meta == nil || resp.Meta.Pagination == nil {
			break
		}

		pagination := resp.Meta.Pagination
		if nextCursorPtr := pagination.NextCursor.Get(); nextCursorPtr == nil || *nextCursorPtr == "" {
			// No more pages
			break
		} else {
			nextCursor = nextCursorPtr
		}

		// Safety limit to prevent infinite loops
		if len(allDescriptors) >= 100000 {
			break
		}
	}

	return allDescriptors, nil
}

// normalizeSDKMetricResponse converts SDK timeseries response to OpsOrch schema.
func normalizeSDKMetricResponse(resp datadogV2.TimeseriesFormulaQueryResponse, query string, source string, site string) []schema.MetricSeries {
	if resp.Data == nil || resp.Data.Attributes == nil {
		return []schema.MetricSeries{}
	}

	attrs := resp.Data.Attributes
	sdkSeries := attrs.Series
	times := attrs.Times
	values := attrs.Values

	series := make([]schema.MetricSeries, 0, len(sdkSeries))

	for i, s := range sdkSeries {
		ms := schema.MetricSeries{
			Name:   getMetricName(s, i),
			Labels: make(map[string]any),
			Points: make([]schema.MetricPoint, 0),
			Metadata: map[string]any{
				"source": source,
			},
		}

		// Generate URL
		// Format: https://app.{site}/metric/explorer?exp_metric={encoded_menu_query}
		if site == "" {
			site = "datadoghq.com"
		}
		// Use the query string passed in
		ms.URL = fmt.Sprintf("https://app.%s/metric/explorer?exp_metric=%s", site, url.QueryEscape(query))

		// Parse group tags into labels
		for _, tag := range s.GroupTags {
			parts := strings.SplitN(tag, ":", 2)
			if len(parts) == 2 {
				ms.Labels[parts[0]] = parts[1]
			}
		}

		// Extract service from labels if present
		if svc, ok := ms.Labels["service"].(string); ok {
			ms.Service = svc
		}

		// Add unit info if available
		if len(s.Unit) > 0 && s.Unit[0].Name != nil {
			ms.Metadata["unit"] = *s.Unit[0].Name
		}

		// Convert times and values to MetricPoints
		// Values is a 2D array where each row corresponds to a series
		if i < len(values) && len(times) > 0 {
			seriesValues := values[i]
			for j, timestamp := range times {
				if j < len(seriesValues) && seriesValues[j] != nil {
					ms.Points = append(ms.Points, schema.MetricPoint{
						Timestamp: time.UnixMilli(timestamp),
						Value:     *seriesValues[j],
					})
				}
			}
		}

		series = append(series, ms)
	}

	return series
}

// getMetricName extracts the metric name from SDK series data.
func getMetricName(s datadogV2.TimeseriesResponseSeries, index int) string {
	// Try to get from query metadata if available
	if s.QueryIndex != nil {
		return fmt.Sprintf("query%d", *s.QueryIndex)
	}
	return fmt.Sprintf("series%d", index)
}

// normalizeSDKMetricsListResponse converts SDK metrics list to descriptors.
func normalizeSDKMetricsListResponse(resp datadogV2.MetricsAndMetricTagConfigurationsResponse, source string, site string) []schema.MetricDescriptor {
	descriptors := make([]schema.MetricDescriptor, 0, len(resp.Data))

	for _, metricConfig := range resp.Data {
		// Extract metric name from the union type
		if metricConfig.Metric != nil && metricConfig.Metric.Id != nil {
			desc := schema.MetricDescriptor{
				Name: *metricConfig.Metric.Id,
				Type: "unknown", // SDK doesn't provide type in list endpoint
				Metadata: map[string]any{
					"source": source,
				},
			}

			// Generate URL
			// Format: https://app.{site}/metric/summary?metric={metric_name}
			if site == "" {
				site = "datadoghq.com"
			}
			desc.URL = fmt.Sprintf("https://app.%s/metric/summary?metric=%s", site, url.QueryEscape(desc.Name))

			descriptors = append(descriptors, desc)
		}
	}

	return descriptors
}

// buildMetricQueryV2 constructs a Datadog v2 API query string from a MetricQuery.
// v2 API format: avg:metric{tag:value} by {label}.rollup(avg, 60)
func buildMetricQueryV2(query schema.MetricQuery) string {
	// If raw query is provided in metadata, use it
	if raw, ok := query.Metadata["query"].(string); ok && raw != "" {
		return raw
	}

	if query.Expression == nil {
		return ""
	}

	// Start with the metric name
	metricName := query.Expression.MetricName

	// Build tag filters
	var tags []string

	// Add expression filters
	for _, f := range query.Expression.Filters {
		tags = append(tags, formatTagFilter(f))
	}

	// Add scope filters
	if query.Scope.Service != "" {
		tags = append(tags, fmt.Sprintf("service:%s", query.Scope.Service))
	}
	if query.Scope.Team != "" {
		tags = append(tags, fmt.Sprintf("team:%s", query.Scope.Team))
	}
	if query.Scope.Environment != "" {
		tags = append(tags, fmt.Sprintf("env:%s", query.Scope.Environment))
	}

	// v2 API requires aggregation prefix
	agg := "avg" // default
	if query.Expression.Aggregation != "" {
		agg = mapAggregation(query.Expression.Aggregation)
	}

	// Build the metric selector with tags
	// v2 API requires curly braces even for empty tag sets
	selector := metricName
	if len(tags) > 0 {
		selector = fmt.Sprintf("%s{%s}", metricName, strings.Join(tags, ","))
	} else {
		// Empty tag set required by v2 API
		selector = fmt.Sprintf("%s{*}", metricName)
	}

	// Format: aggregation:metric{tags}
	selector = fmt.Sprintf("%s:%s", agg, selector)

	// Format: aggregation:metric{tags} by {label1,label2}
	if len(query.Expression.GroupBy) > 0 {
		selector = fmt.Sprintf("%s by {%s}", selector, strings.Join(query.Expression.GroupBy, ","))
	}

	// Format: aggregation:metric{tags}.rollup(agg, interval)
	if query.Step > 0 {
		// Convert step to seconds (Datadog expects seconds)
		stepSeconds := int(query.Step)
		selector = fmt.Sprintf("%s.rollup(%s, %d)", selector, agg, stepSeconds)
	}

	return selector
}

// buildMetricQuery constructs a Datadog query string from a MetricQuery.
// For v2 Metrics API timeseries queries, use simple metric name with filters
func buildMetricQuery(query schema.MetricQuery) string {
	// If raw query is provided in metadata, use it
	if raw, ok := query.Metadata["query"].(string); ok && raw != "" {
		return raw
	}

	if query.Expression == nil {
		return ""
	}

	// Start with the metric name
	metricName := query.Expression.MetricName

	// Build tag filters
	var tags []string

	// Add expression filters
	for _, f := range query.Expression.Filters {
		tags = append(tags, formatTagFilter(f))
	}

	// Add scope filters
	if query.Scope.Service != "" {
		tags = append(tags, fmt.Sprintf("service:%s", query.Scope.Service))
	}
	if query.Scope.Team != "" {
		tags = append(tags, fmt.Sprintf("team:%s", query.Scope.Team))
	}
	if query.Scope.Environment != "" {
		tags = append(tags, fmt.Sprintf("env:%s", query.Scope.Environment))
	}

	// For v2 API, just use metric name with filters
	// The aggregation and groupBy are handled by the SDK request structure
	selector := metricName
	if len(tags) > 0 {
		selector = fmt.Sprintf("%s{%s}", metricName, strings.Join(tags, ","))
	}

	return selector
}

// formatTagFilter formats a MetricFilter as a Datadog tag filter.
func formatTagFilter(f schema.MetricFilter) string {
	switch f.Operator {
	case "=":
		return fmt.Sprintf("%s:%s", f.Label, f.Value)
	case "!=":
		return fmt.Sprintf("!%s:%s", f.Label, f.Value)
	case "=~":
		// Datadog uses glob patterns, not regex
		return fmt.Sprintf("%s:%s", f.Label, f.Value)
	case "!~":
		return fmt.Sprintf("!%s:%s", f.Label, f.Value)
	default:
		return fmt.Sprintf("%s:%s", f.Label, f.Value)
	}
}

// mapAggregation maps OpsOrch aggregation to Datadog aggregation.
func mapAggregation(agg string) string {
	switch strings.ToLower(agg) {
	case "avg", "average":
		return "avg"
	case "sum":
		return "sum"
	case "max":
		return "max"
	case "min":
		return "min"
	case "count":
		return "count"
	default:
		return "avg"
	}
}
