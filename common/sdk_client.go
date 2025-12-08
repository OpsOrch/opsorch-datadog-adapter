package common

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
)

// NewAPIContext creates a configured Datadog API context with authentication.
// The context includes API keys for authentication and server variables for site configuration.
func NewAPIContext(cfg Config) context.Context {
	ctx := context.WithValue(
		context.Background(),
		datadog.ContextAPIKeys,
		map[string]datadog.APIKey{
			"apiKeyAuth": {Key: cfg.APIKey},
			"appKeyAuth": {Key: cfg.AppKey},
		},
	)

	// Configure server URL based on site
	// The SDK uses server variables to determine the API endpoint
	if cfg.Site != "" && cfg.Site != DefaultSite {
		ctx = context.WithValue(
			ctx,
			datadog.ContextServerVariables,
			map[string]string{"site": cfg.Site},
		)
	}

	return ctx
}

// NewAPIClient creates a configured Datadog API client.
// The client is configured with the appropriate server URL based on the site setting.
func NewAPIClient(cfg Config) *datadog.APIClient {
	config := datadog.NewConfiguration()

	// Enable unstable operations (required for Incidents API v2 and other beta features)
	config.SetUnstableOperationEnabled("v2.ListIncidents", true)
	config.SetUnstableOperationEnabled("v2.GetIncident", true)
	config.SetUnstableOperationEnabled("v2.CreateIncident", true)
	config.SetUnstableOperationEnabled("v2.UpdateIncident", true)
	config.SetUnstableOperationEnabled("v2.DeleteIncident", true)
	config.SetUnstableOperationEnabled("v2.SearchIncidents", true)
	config.SetUnstableOperationEnabled("v2.ListIncidentTodos", true)
	config.SetUnstableOperationEnabled("v2.CreateIncidentTodo", true)
	config.SetUnstableOperationEnabled("v2.UpdateIncidentTodo", true)
	config.SetUnstableOperationEnabled("v2.DeleteIncidentTodo", true)

	// Set server based on site
	// For non-default sites, we need to configure the server URL explicitly
	if cfg.Site != "" && cfg.Site != DefaultSite {
		if baseURL, ok := ValidSites[cfg.Site]; ok {
			config.Servers = datadog.ServerConfigurations{
				{
					URL:         baseURL,
					Description: "Datadog " + cfg.Site,
				},
			}
		}
	}

	return datadog.NewAPIClient(config)
}
