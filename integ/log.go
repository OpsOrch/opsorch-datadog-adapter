//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-datadog-adapter/log"
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

	fmt.Println("=== Datadog Log Provider Integration Test ===")
	fmt.Printf("Site: %s\n\n", site)

	// Create provider
	cfg := map[string]any{
		"apiKey": apiKey,
		"appKey": appKey,
		"site":   site,
		"source": "datadog-integ-test",
	}

	provider, err := log.New(cfg)
	if err != nil {
		fmt.Printf("Failed to create provider: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Test 1: Query all logs
	fmt.Println("Test 1: Query all logs (last 15 minutes)")
	end := time.Now()
	start := end.Add(-15 * time.Minute)

	query := schema.LogQuery{
		Expression: &schema.LogExpression{
			Search: "*",
		},
		Start: start,
		End:   end,
		Limit: 10,
	}

	entries, err := provider.Query(ctx, query)
	if err != nil {
		fmt.Printf("  ❌ Query failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query succeeded: returned %d log entries\n", len(entries))
		if len(entries) > 0 {
			fmt.Printf("    First entry: %s\n", entries[0].Message[:min(50, len(entries[0].Message))])
			if entries[0].Metadata != nil {
				fmt.Printf("    Source: %v\n", entries[0].Metadata["source"])
			}
		}
	}
	fmt.Println()

	// Test 2: Query with severity filter
	fmt.Println("Test 2: Query error logs")
	queryWithSeverity := schema.LogQuery{
		Expression: &schema.LogExpression{
			Search:     "*",
			SeverityIn: []string{"error", "critical"},
		},
		Start: start,
		End:   end,
		Limit: 10,
	}

	entries, err = provider.Query(ctx, queryWithSeverity)
	if err != nil {
		fmt.Printf("  ❌ Query with severity failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query with severity succeeded: returned %d error logs\n", len(entries))
		for i, entry := range entries {
			if i >= 3 {
				break
			}
			fmt.Printf("    [%s] %s\n", entry.Severity, entry.Message[:min(60, len(entry.Message))])
		}
	}
	fmt.Println()

	// Test 3: Query with service filter
	fmt.Println("Test 3: Query logs with service scope")
	queryWithService := schema.LogQuery{
		Expression: &schema.LogExpression{
			Search: "*",
		},
		Scope: schema.QueryScope{
			Service: "datadog",
		},
		Start: start,
		End:   end,
		Limit: 10,
	}

	entries, err = provider.Query(ctx, queryWithService)
	if err != nil {
		fmt.Printf("  ❌ Query with service failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query with service succeeded: returned %d logs\n", len(entries))
	}
	fmt.Println()

	// Test 4: Query with environment filter
	fmt.Println("Test 4: Query logs with environment scope")
	queryWithEnv := schema.LogQuery{
		Expression: &schema.LogExpression{
			Search: "*",
		},
		Scope: schema.QueryScope{
			Environment: "prod",
		},
		Start: start,
		End:   end,
		Limit: 10,
	}

	entries, err = provider.Query(ctx, queryWithEnv)
	if err != nil {
		fmt.Printf("  ❌ Query with environment failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query with environment succeeded: returned %d logs\n", len(entries))
	}
	fmt.Println()

	// Test 5: Query with field filters
	fmt.Println("Test 5: Query logs with field filters")
	queryWithFilters := schema.LogQuery{
		Expression: &schema.LogExpression{
			Search: "*",
			Filters: []schema.LogFilter{
				{Field: "status", Operator: "=", Value: "error"},
			},
		},
		Start: start,
		End:   end,
		Limit: 10,
	}

	entries, err = provider.Query(ctx, queryWithFilters)
	if err != nil {
		fmt.Printf("  ❌ Query with filters failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query with filters succeeded: returned %d logs\n", len(entries))
	}
	fmt.Println()

	fmt.Println("=== Integration Test Complete ===")
}
