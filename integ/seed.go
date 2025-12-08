//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-datadog-adapter/common"
	"github.com/opsorch/opsorch-datadog-adapter/incident"
)

func main() {
	// Get credentials from environment
	apiKey := os.Getenv("DATADOG_API_KEY")
	appKey := os.Getenv("DATADOG_APP_KEY")
	site := os.Getenv("DATADOG_SITE")

	if apiKey == "" {
		fmt.Println("Error: DATADOG_API_KEY environment variable is required")
		os.Exit(1)
	}
	if appKey == "" {
		fmt.Println("Error: DATADOG_APP_KEY environment variable is required")
		os.Exit(1)
	}

	if site == "" {
		site = "datadoghq.com"
	}

	fmt.Println("=== Datadog Integration Test Seeding ===")
	fmt.Printf("Site: %s\n\n", site)

	cfg := map[string]any{
		"apiKey": apiKey,
		"appKey": appKey,
		"site":   site,
		"source": "datadog-integ-test-seed",
	}

	ctx := context.Background()

	// Seed monitors (alerts)
	fmt.Println("Seeding monitors (alerts)...")
	if err := seedMonitors(ctx, cfg); err != nil {
		fmt.Printf("  ❌ Failed to seed monitors: %v\n", err)
	} else {
		fmt.Println("  ✓ Monitors seeded successfully")
	}
	fmt.Println()

	// Seed custom metrics
	fmt.Println("Seeding custom metrics...")
	if err := seedMetrics(ctx, cfg); err != nil {
		fmt.Printf("  ❌ Failed to seed metrics: %v\n", err)
	} else {
		fmt.Println("  ✓ Metrics seeded successfully")
	}
	fmt.Println()

	// Seed logs
	fmt.Println("Seeding logs...")
	if err := seedLogs(ctx, cfg); err != nil {
		fmt.Printf("  ❌ Failed to seed logs: %v\n", err)
	} else {
		fmt.Println("  ✓ Logs seeded successfully")
	}
	fmt.Println()

	// Seed incidents
	fmt.Println("Seeding incidents...")
	if err := seedIncidents(ctx, cfg); err != nil {
		fmt.Printf("  ❌ Failed to seed incidents: %v\n", err)
	} else {
		fmt.Println("  ✓ Incidents seeded successfully")
	}
	fmt.Println()

	// Seed service definitions
	fmt.Println("Seeding service definitions...")
	if err := seedServices(ctx, cfg); err != nil {
		fmt.Printf("  ❌ Failed to seed services: %v\n", err)
	} else {
		fmt.Println("  ✓ Services seeded successfully")
	}
	fmt.Println()

	fmt.Println("=== Seeding Complete ===")
	fmt.Println("\nNote: It may take a few minutes for metrics and logs to appear in Datadog.")
	fmt.Println("Monitors will be visible immediately in the Datadog UI.")
}

func seedIncidents(ctx context.Context, cfg map[string]any) error {
	provider, err := incident.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create incident provider: %w", err)
	}

	// Create test incidents with different severities and statuses
	testIncidents := []struct {
		title    string
		severity string
		status   string
	}{
		{"Test Incident - Critical Database Outage", "critical", "open"},
		{"Test Incident - High API Latency", "high", "open"},
		{"Test Incident - Medium Service Degradation", "medium", "open"},
		{"Test Incident - Low Warning Alert", "low", "resolved"},
		{"Test Incident - Info Maintenance Window", "low", "resolved"},
	}

	for _, tc := range testIncidents {
		input := schema.CreateIncidentInput{
			Title:    fmt.Sprintf("%s - %s", tc.title, time.Now().Format("2006-01-02")),
			Severity: tc.severity,
			Status:   tc.status,
		}

		inc, err := provider.Create(ctx, input)
		if err != nil {
			return fmt.Errorf("failed to create incident '%s': %w", tc.title, err)
		}

		// Update to set severity (Create doesn't support it directly)
		updateInput := schema.UpdateIncidentInput{
			// Severity and status need to be set via update
		}
		inc, err = provider.Update(ctx, inc.ID, updateInput)
		if err != nil {
			fmt.Printf("    ⚠ Warning: Failed to update incident severity: %v\n", err)
		}

		fmt.Printf("    Created: %s (ID: %s, Severity: %s)\n", inc.Title, inc.ID, tc.severity)

		// Add a timeline entry
		timelineEntry := schema.TimelineAppendInput{
			At:   time.Now(),
			Kind: "note",
			Body: fmt.Sprintf("Test incident with %s severity created by seeding script", tc.severity),
		}
		if err := provider.AppendTimeline(ctx, inc.ID, timelineEntry); err != nil {
			fmt.Printf("    ⚠ Warning: Failed to add timeline entry: %v\n", err)
		}

		// Small delay to avoid rate limiting
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

func seedMonitors(ctx context.Context, cfg map[string]any) error {
	parsedCfg := common.ParseConfig(cfg)
	apiClient := common.NewAPIClient(parsedCfg)
	apiCtx := common.NewAPIContext(parsedCfg)
	monitorsApi := datadogV1.NewMonitorsApi(apiClient)

	// Create test monitors with different types, severities, and scopes
	// to match integration test queries
	// Note: Using simple metric queries that don't require actual data
	testMonitors := []struct {
		name     string
		query    string
		message  string
		tags     []string
		priority *int64
	}{
		{
			name:     "Test Monitor - High CPU Usage",
			query:    "avg(last_5m):system.cpu.user{*} > 90",
			message:  "CPU usage is above 90% - Test monitor created by seeding script",
			tags:     []string{"env:test", "team:platform"},
			priority: datadog.PtrInt64(2),
		},
		{
			name:     "Test Monitor - Memory Usage",
			query:    "avg(last_5m):system.mem.pct_usable{*} < 0.1",
			message:  "Memory usage is high - Test monitor created by seeding script",
			tags:     []string{"env:test", "team:platform"},
			priority: datadog.PtrInt64(3),
		},
		{
			name:     "Test Monitor - Disk Space",
			query:    "avg(last_5m):system.disk.in_use{*} > 0.85",
			message:  "Disk usage is above 85% - Test monitor created by seeding script",
			tags:     []string{"env:test", "team:infrastructure"},
			priority: datadog.PtrInt64(4),
		},
		{
			name:     "Test Monitor - Service Scope",
			query:    "avg(last_5m):system.load.1{*} > 5",
			message:  "Load average is high - Test monitor for service scope",
			tags:     []string{"env:test", "service:datadog", "team:platform"},
			priority: datadog.PtrInt64(3),
		},
		{
			name:     "Test Monitor - CPU Search",
			query:    "avg(last_5m):system.cpu.idle{*} < 10",
			message:  "CPU idle is low - Test monitor for search term 'cpu'",
			tags:     []string{"env:test", "team:platform"},
			priority: datadog.PtrInt64(3),
		},
	}

	for _, tm := range testMonitors {
		monitor := datadogV1.NewMonitor(tm.query, datadogV1.MONITORTYPE_METRIC_ALERT)
		monitor.SetName(tm.name)
		monitor.SetMessage(tm.message)
		monitor.SetTags(tm.tags)
		if tm.priority != nil {
			monitor.SetPriority(*tm.priority)
		}

		created, _, err := monitorsApi.CreateMonitor(apiCtx, *monitor)
		if err != nil {
			return fmt.Errorf("failed to create monitor '%s': %w", tm.name, err)
		}

		fmt.Printf("    Created: %s (ID: %d)\n", tm.name, created.GetId())
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("    Note: Monitors are immediately visible in queries")

	return nil
}

func seedMetrics(ctx context.Context, cfg map[string]any) error {
	parsedCfg := common.ParseConfig(cfg)
	apiClient := common.NewAPIClient(parsedCfg)
	apiCtx := common.NewAPIContext(parsedCfg)
	metricsApi := datadogV2.NewMetricsApi(apiClient)

	// Submit custom metrics - create multiple data points over the last hour
	// to ensure integration tests find data
	now := time.Now()
	series := []datadogV2.MetricSeries{}

	// Create data points for the last hour (every 5 minutes)
	for i := 0; i < 12; i++ {
		timestamp := now.Add(-time.Duration(i*5) * time.Minute).Unix()

		// Metrics with env:test for general queries
		series = append(series, datadogV2.MetricSeries{
			Metric: "system.cpu.user",
			Type:   datadogV2.METRICINTAKETYPE_GAUGE.Ptr(),
			Points: []datadogV2.MetricPoint{
				{
					Timestamp: datadog.PtrInt64(timestamp),
					Value:     datadog.PtrFloat64(45.5 + float64(i)*2.3),
				},
			},
			Tags: []string{"env:test", "host:test-host-1"},
		})

		// Metrics with env:prod for scope testing (Test 3)
		series = append(series, datadogV2.MetricSeries{
			Metric: "system.cpu.user",
			Type:   datadogV2.METRICINTAKETYPE_GAUGE.Ptr(),
			Points: []datadogV2.MetricPoint{
				{
					Timestamp: datadog.PtrInt64(timestamp),
					Value:     datadog.PtrFloat64(55.5 + float64(i)*1.8),
				},
			},
			Tags: []string{"env:prod", "host:prod-host-1"},
		})

		series = append(series, datadogV2.MetricSeries{
			Metric: "opsorch.test.cpu.usage",
			Type:   datadogV2.METRICINTAKETYPE_GAUGE.Ptr(),
			Points: []datadogV2.MetricPoint{
				{
					Timestamp: datadog.PtrInt64(timestamp),
					Value:     datadog.PtrFloat64(50.0 + float64(i)*1.5),
				},
			},
			Tags: []string{"env:test", "service:opsorch-test", "team:platform"},
		})
	}

	body := datadogV2.NewMetricPayload(series)
	_, _, err := metricsApi.SubmitMetrics(apiCtx, *body)
	if err != nil {
		return fmt.Errorf("failed to submit metrics: %w", err)
	}

	fmt.Printf("    Submitted %d metric data points\n", len(series))
	fmt.Println("      - system.cpu.user with env:test (12 points)")
	fmt.Println("      - system.cpu.user with env:prod (12 points)")
	fmt.Println("      - opsorch.test.cpu.usage (12 points)")
	fmt.Println("    Note: Metrics may take 1-2 minutes to appear in queries")

	return nil
}

func seedLogs(ctx context.Context, cfg map[string]any) error {
	parsedCfg := common.ParseConfig(cfg)
	apiClient := common.NewAPIClient(parsedCfg)
	apiCtx := common.NewAPIContext(parsedCfg)
	logsApi := datadogV2.NewLogsApi(apiClient)

	// Submit multiple test logs with different severities and services
	// Use JSON-formatted messages with explicit status field
	testLogs := []datadogV2.HTTPLogItem{
		// General logs with different severities
		{
			Ddsource: datadog.PtrString("opsorch-test"),
			Ddtags:   datadog.PtrString("env:test,service:opsorch-test,team:platform"),
			Hostname: datadog.PtrString("test-host-1"),
			Message:  `{"level":"info","status":"info","message":"Test log entry - Application started successfully"}`,
			Service:  datadog.PtrString("opsorch-test"),
		},
		{
			Ddsource: datadog.PtrString("opsorch-test"),
			Ddtags:   datadog.PtrString("env:test,service:opsorch-test,team:platform"),
			Hostname: datadog.PtrString("test-host-1"),
			Message:  `{"level":"warn","status":"warn","message":"Test log entry - High memory usage detected"}`,
			Service:  datadog.PtrString("opsorch-test"),
		},
		{
			Ddsource: datadog.PtrString("opsorch-test"),
			Ddtags:   datadog.PtrString("env:test,service:opsorch-test,team:platform"),
			Hostname: datadog.PtrString("test-host-2"),
			Message:  `{"level":"error","status":"error","message":"Test log entry - Failed to connect to database"}`,
			Service:  datadog.PtrString("opsorch-test"),
		},
		{
			Ddsource: datadog.PtrString("opsorch-test"),
			Ddtags:   datadog.PtrString("env:test,service:opsorch-test,team:platform"),
			Hostname: datadog.PtrString("test-host-2"),
			Message:  `{"level":"error","status":"error","message":"Test log entry - Connection timeout"}`,
			Service:  datadog.PtrString("opsorch-test"),
		},
		{
			Ddsource: datadog.PtrString("opsorch-test"),
			Ddtags:   datadog.PtrString("env:test,service:opsorch-test,team:platform"),
			Hostname: datadog.PtrString("test-host-3"),
			Message:  `{"level":"critical","status":"critical","message":"Test log entry - System failure"}`,
			Service:  datadog.PtrString("opsorch-test"),
		},
		// Logs for service scope test
		{
			Ddsource: datadog.PtrString("datadog"),
			Ddtags:   datadog.PtrString("env:test,service:datadog,team:platform"),
			Hostname: datadog.PtrString("test-host-1"),
			Message:  `{"level":"error","status":"error","message":"Test log for service:datadog filter"}`,
			Service:  datadog.PtrString("datadog"),
		},
		// Logs for environment scope test
		{
			Ddsource: datadog.PtrString("opsorch-test"),
			Ddtags:   datadog.PtrString("env:prod,service:opsorch-test,team:platform"),
			Hostname: datadog.PtrString("prod-host-1"),
			Message:  `{"level":"error","status":"error","message":"Test log for env:prod filter"}`,
			Service:  datadog.PtrString("opsorch-test"),
		},
	}

	body := testLogs
	_, _, err := logsApi.SubmitLog(apiCtx, body, *datadogV2.NewSubmitLogOptionalParameters())
	if err != nil {
		return fmt.Errorf("failed to submit logs: %w", err)
	}

	fmt.Printf("    Submitted %d log entries\n", len(testLogs))
	fmt.Println("    Note: Logs may take 1-2 minutes to appear in queries")
	fmt.Println("    Note: Using JSON format with explicit status field for reliable severity filtering")

	return nil
}

func seedServices(ctx context.Context, cfg map[string]any) error {
	parsedCfg := common.ParseConfig(cfg)
	apiClient := common.NewAPIClient(parsedCfg)
	apiCtx := common.NewAPIContext(parsedCfg)
	serviceDefApi := datadogV2.NewServiceDefinitionApi(apiClient)

	// Create test service definitions with team information
	// Using v2.2 schema which supports team field
	testServices := []struct {
		name string
		team string
		tags []string
		env  string
	}{
		{
			name: "opsorch-test-service",
			team: "platform",
			tags: []string{"env:test", "service:opsorch-test"},
			env:  "test",
		},
		{
			name: "opsorch-prod-service",
			team: "platform",
			tags: []string{"env:prod", "service:opsorch-prod"},
			env:  "prod",
		},
		{
			name: "datadog-test-service",
			team: "infrastructure",
			tags: []string{"env:test", "service:datadog"},
			env:  "test",
		},
	}

	for _, ts := range testServices {
		// Create a v2.2 service definition with required fields
		serviceDef := datadogV2.NewServiceDefinitionV2Dot2(
			ts.name,
			datadogV2.SERVICEDEFINITIONV2DOT2VERSION_V2_2,
		)

		// Set optional fields
		serviceDef.SetTeam(ts.team)
		serviceDef.SetTags(ts.tags)
		serviceDef.SetApplication("opsorch")
		serviceDef.SetTier("tier1")
		serviceDef.SetDescription(fmt.Sprintf("Test service for integration tests - team: %s, env: %s", ts.team, ts.env))

		// Create the request body using the helper function
		body := datadogV2.ServiceDefinitionV2Dot2AsServiceDefinitionsCreateRequest(serviceDef)

		// Create or update the service definition
		_, httpResp, err := serviceDefApi.CreateOrUpdateServiceDefinitions(apiCtx, body)
		if err != nil {
			return fmt.Errorf("failed to create service '%s': %w", ts.name, err)
		}
		defer httpResp.Body.Close()

		fmt.Printf("    Created: %s (team: %s, env: %s)\n", ts.name, ts.team, ts.env)
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("    Note: Service definitions are immediately visible in queries")

	return nil
}
