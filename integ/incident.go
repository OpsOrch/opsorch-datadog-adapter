//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/opsorch/opsorch-core/schema"
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

	fmt.Println("=== Datadog Incident Provider Integration Test ===")
	fmt.Printf("Site: %s\n\n", site)

	// Create provider
	cfg := map[string]any{
		"apiKey": apiKey,
		"appKey": appKey,
		"site":   site,
		"source": "datadog-integ-test",
	}

	provider, err := incident.New(cfg)
	if err != nil {
		fmt.Printf("Failed to create provider: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Test 1: Query all incidents
	fmt.Println("Test 1: Query all incidents")
	query := schema.IncidentQuery{
		Limit: 10,
	}

	incidents, err := provider.Query(ctx, query)
	if err != nil {
		fmt.Printf("  ❌ Query failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query succeeded: returned %d incidents\n", len(incidents))
		if len(incidents) > 0 {
			fmt.Printf("    First incident: %s (Status: %s, Severity: %s)\n",
				incidents[0].Title, incidents[0].Status, incidents[0].Severity)
			if incidents[0].URL != "" {
				fmt.Printf("    URL: %s\n", incidents[0].URL)
			}
			if incidents[0].Metadata != nil {
				fmt.Printf("    Source: %v\n", incidents[0].Metadata["source"])
				fmt.Printf("    Public ID: %v\n", incidents[0].Metadata["public_id"])
			}
		}
	}
	fmt.Println()

	// Test 2: Query incidents by status
	fmt.Println("Test 2: Query active incidents")
	queryByStatus := schema.IncidentQuery{
		Statuses: []string{"open", "investigating"},
		Limit:    10,
	}

	incidents, err = provider.Query(ctx, queryByStatus)
	if err != nil {
		fmt.Printf("  ❌ Query by status failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query by status succeeded: returned %d active incidents\n", len(incidents))
		for i, inc := range incidents {
			if i >= 3 {
				break
			}
			fmt.Printf("    [%s] %s\n", inc.Status, inc.Title)
		}
	}
	fmt.Println()

	// Test 3: Query incidents by severity
	fmt.Println("Test 3: Query critical incidents")
	queryBySeverity := schema.IncidentQuery{
		Severities: []string{"critical", "high"},
		Limit:      10,
	}

	incidents, err = provider.Query(ctx, queryBySeverity)
	if err != nil {
		fmt.Printf("  ❌ Query by severity failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query by severity succeeded: returned %d critical/high incidents\n", len(incidents))
	}
	fmt.Println()

	// Test 3a: Verify built-in attributes are populated (Status and Severity)
	fmt.Println("Test 3a: Verify incident normalization reads built-in attributes")
	allIncidents, err := provider.Query(ctx, schema.IncidentQuery{Limit: 5})
	if err != nil {
		fmt.Printf("  ❌ Query failed: %v\n", err)
	} else {
		hasStatus := 0
		hasSeverity := 0
		for _, inc := range allIncidents {
			if inc.Status != "" {
				hasStatus++
			}
			if inc.Severity != "" {
				hasSeverity++
			}
		}
		fmt.Printf("  ✓ Checked %d incidents\n", len(allIncidents))
		fmt.Printf("    Incidents with Status: %d/%d\n", hasStatus, len(allIncidents))
		fmt.Printf("    Incidents with Severity: %d/%d\n", hasSeverity, len(allIncidents))
		if hasStatus == len(allIncidents) && hasSeverity == len(allIncidents) {
			fmt.Println("    ✓ All incidents have Status and Severity (built-in attributes working)")
		} else {
			fmt.Println("    ⚠ Some incidents missing Status or Severity")
		}
	}
	fmt.Println()

	// Test 3b: Verify pagination works (fetch more than default page size)
	fmt.Println("Test 3b: Verify pagination fetches all incidents")
	// Query without limit to test pagination
	allIncidentsNoPagination, err := provider.Query(ctx, schema.IncidentQuery{})
	if err != nil {
		fmt.Printf("  ❌ Query failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Fetched %d total incidents (pagination working)\n", len(allIncidentsNoPagination))
		if len(allIncidentsNoPagination) > 100 {
			fmt.Println("    ✓ Pagination successfully fetched more than one page (>100 incidents)")
		} else {
			fmt.Printf("    Note: Only %d incidents in account (pagination would work with >100)\n", len(allIncidentsNoPagination))
		}
	}
	fmt.Println()

	// Test 4: Get specific incident (if we have one)
	var testIncidentID string
	if len(incidents) > 0 {
		fmt.Println("Test 4: Get specific incident by ID")
		testIncidentID = incidents[0].ID

		singleIncident, err := provider.Get(ctx, testIncidentID)
		if err != nil {
			fmt.Printf("  ❌ Get incident failed: %v\n", err)
		} else {
			fmt.Printf("  ✓ Get incident succeeded\n")
			fmt.Printf("    ID: %s\n", singleIncident.ID)
			fmt.Printf("    Title: %s\n", singleIncident.Title)
			fmt.Printf("    Status: %s\n", singleIncident.Status)
			fmt.Printf("    Severity: %s\n", singleIncident.Severity)
			if singleIncident.URL != "" {
				fmt.Printf("    URL: %s\n", singleIncident.URL)
			}
			if singleIncident.Service != "" {
				fmt.Printf("    Service: %s\n", singleIncident.Service)
			}
		}
		fmt.Println()
	}

	// Test 5: Create a test incident
	fmt.Println("Test 5: Create a test incident")
	createInput := schema.CreateIncidentInput{
		Title:    fmt.Sprintf("Integration Test Incident - %s", time.Now().Format("2006-01-02 15:04:05")),
		Severity: "low",
		Status:   "open",
	}

	createdIncident, err := provider.Create(ctx, createInput)
	if err != nil {
		fmt.Printf("  ❌ Create incident failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Create incident succeeded\n")
		fmt.Printf("    ID: %s\n", createdIncident.ID)
		fmt.Printf("    Title: %s\n", createdIncident.Title)
		fmt.Printf("    Status: %s\n", createdIncident.Status)
		fmt.Printf("    Severity: %s\n", createdIncident.Severity)
		testIncidentID = createdIncident.ID
	}
	fmt.Println()

	// Test 6: Update the incident (if we created one)
	if testIncidentID != "" {
		fmt.Println("Test 6: Update incident")
		newTitle := fmt.Sprintf("Updated Integration Test Incident - %s", time.Now().Format("2006-01-02 15:04:05"))
		newStatus := "investigating"

		updateInput := schema.UpdateIncidentInput{
			Title:  &newTitle,
			Status: &newStatus,
		}

		updatedIncident, err := provider.Update(ctx, testIncidentID, updateInput)
		if err != nil {
			fmt.Printf("  ❌ Update incident failed: %v\n", err)
		} else {
			fmt.Printf("  ✓ Update incident succeeded\n")
			fmt.Printf("    Title: %s\n", updatedIncident.Title)
			fmt.Printf("    Status: %s\n", updatedIncident.Status)
		}
		fmt.Println()

		// Test 7: Get timeline
		fmt.Println("Test 7: Get incident timeline")
		timeline, err := provider.GetTimeline(ctx, testIncidentID)
		if err != nil {
			fmt.Printf("  ❌ Get timeline failed: %v\n", err)
		} else {
			fmt.Printf("  ✓ Get timeline succeeded: %d entries\n", len(timeline))
			for i, entry := range timeline {
				if i >= 3 {
					break
				}
				fmt.Printf("    [%s] %s: %s\n", entry.At.Format("15:04:05"), entry.Kind, entry.Body[:min(50, len(entry.Body))])
			}
		}
		fmt.Println()

		// Test 8: Append to timeline
		fmt.Println("Test 8: Append to incident timeline")
		appendInput := schema.TimelineAppendInput{
			At:   time.Now(),
			Kind: "note",
			Body: "Integration test note added via API",
		}

		err = provider.AppendTimeline(ctx, testIncidentID, appendInput)
		if err != nil {
			fmt.Printf("  ❌ Append timeline failed: %v\n", err)
		} else {
			fmt.Printf("  ✓ Append timeline succeeded\n")
		}
		fmt.Println()

		// Test 9: Resolve the incident
		fmt.Println("Test 9: Resolve incident")
		resolvedStatus := "resolved"
		resolveInput := schema.UpdateIncidentInput{
			Status: &resolvedStatus,
		}

		_, err = provider.Update(ctx, testIncidentID, resolveInput)
		if err != nil {
			fmt.Printf("  ❌ Resolve incident failed: %v\n", err)
		} else {
			fmt.Printf("  ✓ Resolve incident succeeded\n")
		}
		fmt.Println()
	}

	fmt.Println("=== Integration Test Complete ===")
	if testIncidentID != "" {
		fmt.Printf("\nNote: Test incident created with ID: %s\n", testIncidentID)
		fmt.Println("You may want to clean this up in the Datadog UI if needed.")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
