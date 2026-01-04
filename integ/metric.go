//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-datadog-adapter/metric"
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

	fmt.Println("=== Datadog Metric Provider Integration Test ===")
	fmt.Printf("Site: %s\n\n", site)

	// Create provider
	cfg := map[string]any{
		"apiKey": apiKey,
		"appKey": appKey,
		"site":   site,
		"source": "datadog-integ-test",
	}

	provider, err := metric.New(cfg)
	if err != nil {
		fmt.Printf("Failed to create provider: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Test 1: Query metrics
	fmt.Println("Test 1: Query system.cpu.user metric")
	end := time.Now()
	start := end.Add(-1 * time.Hour)

	query := schema.MetricQuery{
		Expression: &schema.MetricExpression{
			MetricName:  "system.cpu.user",
			Aggregation: "avg",
		},
		Start: start,
		End:   end,
		Step:  60,
	}

	series, err := provider.Query(ctx, query)
	if err != nil {
		fmt.Printf("  ❌ Query failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query succeeded: returned %d series\n", len(series))
		if len(series) > 0 {
			fmt.Printf("    First series: %s with %d points\n", series[0].Name, len(series[0].Points))
			if series[0].URL != "" {
				fmt.Printf("    URL: %s\n", series[0].URL)
			}
			if series[0].Metadata != nil {
				fmt.Printf("    Source: %v\n", series[0].Metadata["source"])
			}
		}
	}
	fmt.Println()

	// Test 2: Query with filters
	fmt.Println("Test 2: Query with tag filters")
	queryWithFilters := schema.MetricQuery{
		Expression: &schema.MetricExpression{
			MetricName:  "system.cpu.user",
			Aggregation: "avg",
			Filters: []schema.MetricFilter{
				{Label: "host", Operator: "=", Value: "*"},
			},
		},
		Start: start,
		End:   end,
		Step:  60,
	}

	series, err = provider.Query(ctx, queryWithFilters)
	if err != nil {
		fmt.Printf("  ❌ Query with filters failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query with filters succeeded: returned %d series\n", len(series))
	}
	fmt.Println()

	// Test 3: Query with scope
	fmt.Println("Test 3: Query with scope (environment)")
	queryWithScope := schema.MetricQuery{
		Expression: &schema.MetricExpression{
			MetricName:  "system.cpu.user",
			Aggregation: "avg",
		},
		Scope: schema.QueryScope{
			Environment: "prod",
		},
		Start: start,
		End:   end,
		Step:  60,
	}

	series, err = provider.Query(ctx, queryWithScope)
	if err != nil {
		fmt.Printf("  ❌ Query with scope failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query with scope succeeded: returned %d series\n", len(series))
	}
	fmt.Println()

	// Test 4: Describe metrics
	fmt.Println("Test 4: Describe available metrics")
	descriptors, err := provider.Describe(ctx, schema.QueryScope{})
	if err != nil {
		fmt.Printf("  ❌ Describe failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Describe succeeded: found %d metrics\n", len(descriptors))
		if len(descriptors) > 0 {
			fmt.Printf("    Sample metrics: ")
			for i := 0; i < 5 && i < len(descriptors); i++ {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(descriptors[i].Name)
				if i == 0 && descriptors[i].URL != "" {
					fmt.Printf(" (URL: %s)", descriptors[i].URL)
				}
			}
			fmt.Println()
		}

		// Verify pagination is working (should have more than one page worth of metrics)
		if len(descriptors) > 1000 {
			fmt.Println("    ✓ Pagination working: fetched >1000 metrics across multiple pages")
		} else {
			fmt.Printf("    Note: Account has %d metrics (pagination would work with >1000)\n", len(descriptors))
		}
	}
	fmt.Println()

	// Test 5: Query with groupBy
	fmt.Println("Test 5: Query with groupBy")
	queryWithGroupBy := schema.MetricQuery{
		Expression: &schema.MetricExpression{
			MetricName:  "system.cpu.user",
			Aggregation: "avg",
			GroupBy:     []string{"host"},
		},
		Start: start,
		End:   end,
		Step:  60,
	}

	series, err = provider.Query(ctx, queryWithGroupBy)
	if err != nil {
		fmt.Printf("  ❌ Query with groupBy failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query with groupBy succeeded: returned %d series\n", len(series))
		// Verify that series have labels (which come from groupBy)
		if len(series) > 0 {
			hasLabels := false
			for _, s := range series {
				if len(s.Labels) > 0 {
					hasLabels = true
					break
				}
			}
			if hasLabels {
				fmt.Println("    ✓ Series have labels (groupBy working)")
			} else {
				fmt.Println("    Note: No labels found (may not have data with host tags)")
			}
		}
	}
	fmt.Println()

	// Test 5a: Query with both groupBy and rollup (Step)
	fmt.Println("Test 5a: Query with groupBy and rollup")
	queryWithBoth := schema.MetricQuery{
		Expression: &schema.MetricExpression{
			MetricName:  "system.cpu.user",
			Aggregation: "avg",
			GroupBy:     []string{"host"},
		},
		Start: start,
		End:   end,
		Step:  300, // 5 minute rollup
	}

	series, err = provider.Query(ctx, queryWithBoth)
	if err != nil {
		fmt.Printf("  ❌ Query with groupBy and rollup failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query with groupBy and rollup succeeded: returned %d series\n", len(series))
		if len(series) > 0 && len(series[0].Points) > 0 {
			fmt.Printf("    First series has %d points (rollup working)\n", len(series[0].Points))
		}
	}
	fmt.Println()

	// Test 5b: Verify Describe pagination (cursor-based)
	fmt.Println("Test 5b: Verify Describe pagination")
	allDescriptors, err := provider.Describe(ctx, schema.QueryScope{})
	if err != nil {
		fmt.Printf("  ❌ Describe failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Describe succeeded: found %d total metrics\n", len(allDescriptors))
		if len(allDescriptors) > 1000 {
			fmt.Println("    ✓ Pagination successfully fetched multiple pages (>1000 metrics)")
		} else {
			fmt.Printf("    Note: %d metrics found (pagination would work with >1000)\n", len(allDescriptors))
		}
	}
	fmt.Println()

	fmt.Println("=== Integration Test Complete ===")
}
