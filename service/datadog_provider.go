package service

import (
	"context"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/opsorch/opsorch-core/schema"
	coreservice "github.com/opsorch/opsorch-core/service"
	"github.com/opsorch/opsorch-datadog-adapter/common"
)

// ProviderName is the registry key for the Datadog service adapter.
const ProviderName = "datadog"

// AdapterVersion and RequiresCore express compatibility.
const (
	AdapterVersion = "0.1.0"
	RequiresCore   = ">=0.1.0"
)

// DatadogProvider implements the service.Provider interface for Datadog.
type DatadogProvider struct {
	client *datadog.APIClient
	config common.Config
}

// New constructs the provider from decrypted config.
func New(cfg map[string]any) (coreservice.Provider, error) {
	parsedCfg := common.ParseConfig(cfg)
	if err := common.ValidateConfig(parsedCfg); err != nil {
		return nil, err
	}

	client := common.NewAPIClient(parsedCfg)
	return &DatadogProvider{
		client: client,
		config: parsedCfg,
	}, nil
}

func init() {
	_ = coreservice.RegisterProvider(ProviderName, New)
}

// Query returns services from the Datadog Service Catalog.
func (p *DatadogProvider) Query(ctx context.Context, query schema.ServiceQuery) ([]schema.Service, error) {
	apiCtx := common.NewAPIContext(p.config)
	api := datadogV2.NewServiceDefinitionApi(p.client)

	// Build optional parameters
	pageSize := int64(100) // Default page size
	if query.Limit > 0 && query.Limit < 100 {
		pageSize = int64(query.Limit)
	}

	// Collect all services across pages
	allServices := make([]schema.Service, 0)
	pageNumber := int64(0)
	retryCfg := common.DefaultRetryConfig()

	for {
		optionalParams := datadogV2.NewListServiceDefinitionsOptionalParameters()
		optionalParams.WithPageSize(pageSize)
		optionalParams.WithPageNumber(pageNumber)

		// Call SDK with retry logic for rate limits
		var resp datadogV2.ServiceDefinitionsListResponse

		err := common.RetryWithBackoff(ctx, retryCfg, func() error {
			r, httpResp, err := api.ListServiceDefinitions(apiCtx, *optionalParams)
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

		// Normalize and add services from this page
		pageServices := normalizeServiceListResponse(resp, p.config.Source)
		allServices = append(allServices, pageServices...)

		// Check if there are more pages
		data := resp.GetData()
		if len(data) < int(pageSize) {
			// Last page (partial or empty)
			break
		}

		// Check if we've reached the requested limit
		if query.Limit > 0 && len(allServices) >= query.Limit {
			break
		}

		pageNumber++
	}

	// Apply client-side filters (Datadog API doesn't support filtering by name, tags, etc.)
	services := applyFilters(allServices, query)

	return services, nil
}

// normalizeServiceListResponse converts Datadog SDK services to OpsOrch services.
func normalizeServiceListResponse(resp datadogV2.ServiceDefinitionsListResponse, source string) []schema.Service {
	data := resp.GetData()
	services := make([]schema.Service, 0, len(data))
	for _, svcDef := range data {
		services = append(services, normalizeService(svcDef, source))
	}
	return services
}

// normalizeService converts a single Datadog SDK service to an OpsOrch service.
func normalizeService(svcDef datadogV2.ServiceDefinitionData, source string) schema.Service {
	id := svcDef.GetId()

	// Use ID as name (service definitions use ID as the service name)
	service := schema.Service{
		ID:   id,
		Name: id,
		Tags: make(map[string]string),
		Metadata: map[string]any{
			"source": source,
		},
	}

	// Get attributes if present
	if attrs, ok := svcDef.GetAttributesOk(); ok {
		// Extract tags from schema if available
		if schemaObj, ok := attrs.GetSchemaOk(); ok {
			// Schema is a union type, try to extract tags from the actual instance
			if actualSchema := schemaObj.GetActualInstance(); actualSchema != nil {
				service.Metadata["schema"] = actualSchema

				// Try to extract tags from the schema (best effort)
				extractTagsFromSchema(actualSchema, &service)
			}
		}

		// Add meta information to metadata
		if meta, ok := attrs.GetMetaOk(); ok {
			if origin, ok := meta.GetOriginOk(); ok && *origin != "" {
				service.Metadata["origin"] = *origin
			}
			if ingestionSource, ok := meta.GetIngestionSourceOk(); ok && *ingestionSource != "" {
				service.Metadata["ingestion_source"] = *ingestionSource
			}
		}
	}

	return service
}

// extractTagsFromSchema attempts to extract tags from the service definition schema.
// This is a best-effort function since the schema is a complex union type.
func extractTagsFromSchema(schema interface{}, service *schema.Service) {
	// Try to extract tags from different schema versions
	switch s := schema.(type) {
	case *datadogV2.ServiceDefinitionV2Dot2:
		if tags, ok := s.GetTagsOk(); ok && tags != nil {
			for _, tag := range *tags {
				parts := strings.SplitN(tag, ":", 2)
				if len(parts) == 2 {
					service.Tags[parts[0]] = parts[1]
				}
			}
		}
		// Extract team
		if team, ok := s.GetTeamOk(); ok && team != nil {
			service.Tags["team"] = *team
		}
	case *datadogV2.ServiceDefinitionV2Dot1:
		if tags, ok := s.GetTagsOk(); ok && tags != nil {
			for _, tag := range *tags {
				parts := strings.SplitN(tag, ":", 2)
				if len(parts) == 2 {
					service.Tags[parts[0]] = parts[1]
				}
			}
		}
		// Extract team
		if team, ok := s.GetTeamOk(); ok && team != nil {
			service.Tags["team"] = *team
		}
	case *datadogV2.ServiceDefinitionV2:
		if tags, ok := s.GetTagsOk(); ok && tags != nil {
			for _, tag := range *tags {
				parts := strings.SplitN(tag, ":", 2)
				if len(parts) == 2 {
					service.Tags[parts[0]] = parts[1]
				}
			}
		}
		// Extract team
		if team, ok := s.GetTeamOk(); ok && team != nil {
			service.Tags["team"] = *team
		}
	}
}

// applyFilters applies client-side filters to the service list.
func applyFilters(services []schema.Service, query schema.ServiceQuery) []schema.Service {
	filtered := make([]schema.Service, 0, len(services))

	for _, svc := range services {
		// Filter by IDs
		if len(query.IDs) > 0 {
			found := false
			for _, id := range query.IDs {
				if svc.ID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by name (substring match)
		if query.Name != "" {
			if !containsIgnoreCase(svc.Name, query.Name) {
				continue
			}
		}

		// Filter by tags
		if len(query.Tags) > 0 {
			matchAll := true
			for key, value := range query.Tags {
				if svcValue, ok := svc.Tags[key]; !ok || svcValue != value {
					matchAll = false
					break
				}
			}
			if !matchAll {
				continue
			}
		}

		// Filter by scope
		if query.Scope.Team != "" {
			if teamTag, ok := svc.Tags["team"]; !ok || teamTag != query.Scope.Team {
				continue
			}
		}

		if query.Scope.Environment != "" {
			if envTag, ok := svc.Tags["env"]; !ok || envTag != query.Scope.Environment {
				continue
			}
		}

		filtered = append(filtered, svc)
	}

	// Apply limit
	if query.Limit > 0 && len(filtered) > query.Limit {
		filtered = filtered[:query.Limit]
	}

	return filtered
}

// Helper functions

func findFirstColon(s string) int {
	for i, c := range s {
		if c == ':' {
			return i
		}
	}
	return -1
}

func containsIgnoreCase(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return contains(s, substr)
}

func toLower(s string) string {
	result := make([]rune, len(s))
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			result[i] = r + 32
		} else {
			result[i] = r
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
