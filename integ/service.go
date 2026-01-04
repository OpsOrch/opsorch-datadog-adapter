//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/opsorch/opsorch-core/schema"
	"github.com/opsorch/opsorch-datadog-adapter/service"
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

	fmt.Println("=== Datadog Service Provider Integration Test ===")
	fmt.Printf("Site: %s\n\n", site)

	// Create provider
	cfg := map[string]any{
		"apiKey": apiKey,
		"appKey": appKey,
		"site":   site,
		"source": "datadog-integ-test",
	}

	provider, err := service.New(cfg)
	if err != nil {
		fmt.Printf("Failed to create provider: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Test 1: Query all services
	fmt.Println("Test 1: Query all services")
	query := schema.ServiceQuery{
		Limit: 10,
	}

	services, err := provider.Query(ctx, query)
	if err != nil {
		fmt.Printf("  ❌ Query failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query succeeded: returned %d services\n", len(services))
		if len(services) > 0 {
			fmt.Printf("    First service: %s (ID: %s)\n", services[0].Name, services[0].ID)
			if services[0].URL != "" {
				fmt.Printf("    URL: %s\n", services[0].URL)
			}
			if services[0].Metadata != nil {
				fmt.Printf("    Source: %v\n", services[0].Metadata["source"])
			}
			if len(services[0].Tags) > 0 {
				fmt.Printf("    Tags: ")
				count := 0
				for k, v := range services[0].Tags {
					if count > 0 {
						fmt.Print(", ")
					}
					fmt.Printf("%s:%s", k, v)
					count++
					if count >= 3 {
						break
					}
				}
				fmt.Println()
			}
		}
	}
	fmt.Println()

	// Test 2: Query services by name
	if len(services) > 0 {
		fmt.Println("Test 2: Query services by name")
		// Use part of the first service name
		searchName := services[0].Name
		if len(searchName) > 3 {
			searchName = searchName[:3]
		}

		queryByName := schema.ServiceQuery{
			Name:  searchName,
			Limit: 10,
		}

		results, err := provider.Query(ctx, queryByName)
		if err != nil {
			fmt.Printf("  ❌ Query by name failed: %v\n", err)
		} else {
			fmt.Printf("  ✓ Query by name succeeded: returned %d services matching '%s'\n", len(results), searchName)
			for i, svc := range results {
				if i >= 3 {
					break
				}
				fmt.Printf("    - %s\n", svc.Name)
			}
		}
		fmt.Println()
	}

	// Test 3: Query services by tags
	fmt.Println("Test 3: Query services with env:prod tag")
	queryByTags := schema.ServiceQuery{
		Tags: map[string]string{
			"env": "prod",
		},
		Limit: 10,
	}

	results, err := provider.Query(ctx, queryByTags)
	if err != nil {
		fmt.Printf("  ❌ Query by tags failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query by tags succeeded: returned %d services with env:prod\n", len(results))
	}
	fmt.Println()

	// Test 4: Query services by team scope
	fmt.Println("Test 4: Query services with team scope")
	queryByTeam := schema.ServiceQuery{
		Scope: schema.QueryScope{
			Team: "platform",
		},
		Limit: 10,
	}

	results, err = provider.Query(ctx, queryByTeam)
	if err != nil {
		fmt.Printf("  ❌ Query by team failed: %v\n", err)
	} else {
		fmt.Printf("  ✓ Query by team succeeded: returned %d services for team 'platform'\n", len(results))
	}
	fmt.Println()

	// Test 5: Query specific service by ID
	if len(services) > 0 {
		fmt.Println("Test 5: Query specific service by ID")
		serviceID := services[0].ID

		queryByID := schema.ServiceQuery{
			IDs: []string{serviceID},
		}

		results, err := provider.Query(ctx, queryByID)
		if err != nil {
			fmt.Printf("  ❌ Query by ID failed: %v\n", err)
		} else {
			fmt.Printf("  ✓ Query by ID succeeded: returned %d service(s)\n", len(results))
			if len(results) > 0 {
				svc := results[0]
				fmt.Printf("    ID: %s\n", svc.ID)
				fmt.Printf("    Name: %s\n", svc.Name)
				if svc.URL != "" {
					fmt.Printf("    URL: %s\n", svc.URL)
				}
				if team, ok := svc.Tags["team"]; ok {
					fmt.Printf("    Team: %s\n", team)
				}
				if contacts, ok := svc.Metadata["contacts"]; ok {
					fmt.Printf("    Contacts: %v\n", contacts)
				}
				if links, ok := svc.Metadata["links"]; ok {
					fmt.Printf("    Links: %v\n", links)
				}
			}
		}
		fmt.Println()
	}

	fmt.Println("=== Integration Test Complete ===")
}
