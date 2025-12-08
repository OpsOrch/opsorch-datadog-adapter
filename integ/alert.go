//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-datadog-adapter/alert"
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

	fmt.Println("=== Datadog Alert Provider Integration Test ===")
	fmt.Printf("Site: %s\n\n", site)

	// Create provider
	cfg := map[string]any{
		"apiKey": apiKey,
		"appKey": appKey,
		"site":   site,
		"source": "datadog-integ-test",
	}

	provider, err := alert.New(cfg)
	if err != nil {
		fmt.Printf("Failed to create provider: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Test 1: Query all monitors
	fmt.Println("Test 1: Query all monitors")
	query := schema.AlertQuery{
		Limit: 10,
	}

	alerts, err := provider.Query(ctx, query)
	if err != nil {
		fmt.Printf("  ❌ Query failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query succeeded: returned %d monitors\n", len(alerts))
		if len(alerts) > 0 {
			fmt.Printf("    First monitor: %s (Status: %s, Severity: %s)\n",
				alerts[0].Title, alerts[0].Status, alerts[0].Severity)
			if alerts[0].Metadata != nil {
				fmt.Printf("    Source: %v\n", alerts[0].Metadata["source"])
				fmt.Printf("    Monitor ID: %v\n", alerts[0].Metadata["monitor_id"])
			}
		}
	}
	fmt.Println()

	// Test 2: Query monitors by status
	fmt.Println("Test 2: Query firing monitors")
	queryByStatus := schema.AlertQuery{
		Statuses: []string{"firing"},
		Limit:    10,
	}

	alerts, err = provider.Query(ctx, queryByStatus)
	if err != nil {
		fmt.Printf("  ❌ Query by status failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query by status succeeded: returned %d firing monitors\n", len(alerts))
		for i, a := range alerts {
			if i >= 3 {
				break
			}
			fmt.Printf("    [%s] %s\n", a.Status, a.Title)
		}
	}
	fmt.Println()

	// Test 3: Query monitors by severity
	fmt.Println("Test 3: Query critical monitors")
	queryBySeverity := schema.AlertQuery{
		Severities: []string{"critical", "high"},
		Limit:      10,
	}

	alerts, err = provider.Query(ctx, queryBySeverity)
	if err != nil {
		fmt.Printf("  ❌ Query by severity failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query by severity succeeded: returned %d critical/high monitors\n", len(alerts))
	}
	fmt.Println()

	// Test 4: Query monitors with service scope
	fmt.Println("Test 4: Query monitors with service scope")
	queryWithService := schema.AlertQuery{
		Scope: schema.QueryScope{
			Service: "datadog",
		},
		Limit: 10,
	}

	alerts, err = provider.Query(ctx, queryWithService)
	if err != nil {
		fmt.Printf("  ❌ Query with service failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query with service succeeded: returned %d monitors\n", len(alerts))
	}
	fmt.Println()

	// Test 5: Get specific monitor (if we have one)
	if len(alerts) > 0 {
		fmt.Println("Test 5: Get specific monitor by ID")
		monitorID := alerts[0].ID

		singleAlert, err := provider.Get(ctx, monitorID)
		if err != nil {
			fmt.Printf("  ❌ Get monitor failed: %v\n", err)
		} else {
			fmt.Printf("  ✓ Get monitor succeeded\n")
			fmt.Printf("    ID: %s\n", singleAlert.ID)
			fmt.Printf("    Title: %s\n", singleAlert.Title)
			fmt.Printf("    Status: %s\n", singleAlert.Status)
			fmt.Printf("    Severity: %s\n", singleAlert.Severity)
			if singleAlert.Service != "" {
				fmt.Printf("    Service: %s\n", singleAlert.Service)
			}
		}
		fmt.Println()
	}

	// Test 6: Query with search term
	fmt.Println("Test 6: Query monitors with search term")
	queryWithSearch := schema.AlertQuery{
		Query: "cpu",
		Limit: 10,
	}

	alerts, err = provider.Query(ctx, queryWithSearch)
	if err != nil {
		fmt.Printf("  ❌ Query with search failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query with search succeeded: returned %d monitors matching 'cpu'\n", len(alerts))
	}
	fmt.Println()

	fmt.Println("=== Integration Test Complete ===")
}
