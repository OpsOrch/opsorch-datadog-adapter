# OpsOrch Datadog Adapter

This adapter integrates OpsOrch with Datadog's observability platform using the official [Datadog Go SDK v2](https://github.com/DataDog/datadog-api-client-go), providing five capabilities:

1. **Metric Adapter**: Query time-series metrics via Datadog Metrics API (v2)
2. **Log Adapter**: Search and retrieve logs via Datadog Logs API (v2)
3. **Alert Adapter**: Query Datadog monitors via Monitors API (v1)
4. **Incident Adapter**: Manage Datadog incidents via Incidents API (v2)
5. **Service Adapter**: Query services from Datadog Service Catalog API (v2)

## Dependencies

This adapter uses the official Datadog Go SDK:
- `github.com/DataDog/datadog-api-client-go/v2` - Official Datadog API client with full type safety and automatic retries

## Configuration

All adapters share the same base configuration:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `apiKey` | string | Yes | Datadog API key |
| `appKey` | string | Yes | Datadog Application key |
| `site` | string | No | Datadog site (default: `datadoghq.com`) |
| `source` | string | No | Source identifier for metadata (default: `datadog`) |

### Supported Sites

- `datadoghq.com` (US1, default)
- `us3.datadoghq.com` (US3)
- `us5.datadoghq.com` (US5)
- `datadoghq.eu` (EU1)
- `ap1.datadoghq.com` (AP1)

### Example Configuration

```json
{
  "apiKey": "your-api-key",
  "appKey": "your-application-key",
  "site": "datadoghq.com",
  "source": "datadog"
}
```

## Building

```bash
go mod download  # download dependencies including Datadog SDK
make test        # run unit tests
make build       # build all packages
make plugin      # builds plugin binaries in ./bin/
make integ       # run integration tests (requires credentials)
```

## SDK Features

The adapter leverages the following Datadog SDK features:

- **Type Safety**: All API requests and responses use strongly-typed SDK models
- **Authentication**: Automatic API key and application key injection via SDK context
- **Retry Logic**: Built-in exponential backoff and retry handling for transient failures
- **Error Handling**: Structured error types with detailed error messages
- **Site Configuration**: Automatic server URL configuration based on Datadog site

## Field Mapping

### Log Adapter

#### Query ↔ Datadog Mapping

| OpsOrch Field | Datadog Behavior |
|---------------|------------------|
| `expression.search` | Literal search string appended to the Datadog query |
| `expression.severityIn` | Converted to `status:(...)` filter using `mapSeverityToDatadog` |
| `expression.filters` | Rendered into Datadog clauses (`field:value`, negative or wildcard) |
| `scope.service/team/environment` | Adds `service:<name>`, `team:<name>`, `env:<name>` filters |
| `metadata` (string values) | Added as `key:value` clauses |

#### Pagination

- **API**: Uses `meta.page.after` cursor from responses
- **Request**: Sends `page[cursor]` from the previous response and `page[limit]`
- **Defaults**: 1 000 logs per request, 10 000 total unless `query.Limit` overrides it
- **Loop**: Continues until cursor is empty, limit reached, or the API returns no data

#### Normalization

- `LogEntry.Message` ← `attributes.message`
- `LogEntry.Severity` ← Datadog status mapped back to OpsOrch levels
- `LogEntry.Service` ← `attributes.service`
- `LogEntry.Labels` ← Flattened tags
- `LogEntry.Fields` ← Raw Datadog `attributes.attributes`
- `LogEntry.Metadata` includes `source`, `log_id`, `host`, timestamp, and tag array

### Incident Field Mapping

The incident adapter maps between OpsOrch's normalized schema and Datadog's incident model:

#### Built-in Attributes

| OpsOrch Field | Datadog Field | Type | Notes |
|---------------|---------------|------|-------|
| `ID` | `id` | string | Incident UUID |
| `Title` | `attributes.title` | string | Incident title |
| `Status` | `attributes.state` | string | Mapped: active→open, stable→investigating, resolved→resolved |
| `Severity` | `attributes.severity` | string | Mapped: SEV-1→critical, SEV-2→high, SEV-3→medium, SEV-4→low, SEV-5→info |
| `Service` | `attributes.fields.service` or `attributes.fields.services` | string | Extracted from custom fields (first value if multiple) |
| `CreatedAt` | `attributes.created` | time | ISO 8601 timestamp |
| `UpdatedAt` | `attributes.modified` | time | ISO 8601 timestamp |

#### Metadata Fields

| OpsOrch Metadata | Datadog Field | Type | Notes |
|------------------|---------------|------|-------|
| `source` | N/A | string | Always set to configured source (default: "datadog") |
| `customer_impacted` | `attributes.customer_impacted` | bool | Whether customers were impacted |
| `customer_impact_scope` | `attributes.customer_impact_scope` | string | **Free-form string** describing impact (e.g., "EU customers", "Checkout unavailable") |
| `public_id` | `attributes.public_id` | string | Human-readable incident ID |
| `detected` | `attributes.detected` | time | When incident was detected |
| `resolved` | `attributes.resolved` | time | When incident was resolved |

#### Custom Fields

All Datadog custom fields are copied to the `Fields` map. Custom fields can be:
- **Single-value**: Stored as `string`
- **Multi-value**: Stored as `[]string` when multiple values exist

Common custom fields:
- `team`: Team responsible (string or []string)
- `environment`: Environment affected (string or []string)
- `env`: Short form of environment (string or []string)

#### Customer Impact Handling

When creating or updating incidents with `customer_impacted=true`:

1. **Required**: Both `customer_impacted` and `customer_impact_scope` must be set together
2. **Scope Format**: `customer_impact_scope` is a **free-form string** (not an enum)
   - ✅ Valid: "EU customers affected", "Checkout service unavailable", "all", "some", "none"
   - ✅ Default: "unknown" (when scope not provided)
3. **API Requirement**: Datadog returns 400 error if `customer_impacted=true` without scope

Example:
```json
{
  "title": "Payment service outage",
  "metadata": {
    "customer_impacted": true,
    "customer_impact_scope": "EU customers unable to complete checkout"
  }
}
```

#### Scope Filtering

The incident adapter supports client-side filtering by scope:

- **Service**: Matches `incident.Service` field
- **Team**: Matches `incident.Fields["team"]` or `incident.Metadata["team"]`
  - Supports both single values (string) and multiple values ([]string)
  - If multiple teams, matches if ANY team equals the query
- **Environment**: Matches `incident.Fields["environment"]`, `incident.Fields["env"]`, or `incident.Metadata["environment"]`
  - Supports both single values (string) and multiple values ([]string)
  - If multiple environments, matches if ANY environment equals the query

**Multi-Value Field Handling**: When Datadog returns multiple values for a custom field (e.g., an incident belongs to multiple teams), the adapter:
1. Stores them as `[]string` in the `Fields` map
2. Filters match if ANY value in the array equals the query
3. This ensures incidents with multiple teams/environments can be found by any of their values

Example:
```go
// Incident with multiple teams
incident.Fields["team"] = []string{"platform", "infrastructure"}

// Query for either team will match
query1 := IncidentQuery{Scope: QueryScope{Team: "platform"}}        // ✅ Matches
query2 := IncidentQuery{Scope: QueryScope{Team: "infrastructure"}}  // ✅ Matches
query3 := IncidentQuery{Scope: QueryScope{Team: "security"}}        // ❌ No match
```

#### Known Limitations

1. **Severity Cannot Be Set**: Datadog Incidents API v2 does not support setting severity via Create or Update
   - All incidents default to "medium" (SEV-3)
   - Severity must be changed manually in Datadog UI
   - This is a Datadog API limitation, not an adapter issue

2. **State/Status Mapping**: Limited to three states (active, stable, resolved)
   - More granular OpsOrch statuses are mapped to these three

3. **Custom Fields**: Service, team, environment are stored as custom fields in Datadog
   - Not available as built-in attributes in the API
   - Extracted during normalization

4. **Timeline API**: Uses Todos API as timeline entries
   - Datadog SDK v2 doesn't expose direct timeline API
   - `GetTimeline()` → `ListIncidentTodos()`
   - `AppendTimeline()` → `CreateIncidentTodo()` with completed status

### Metric Adapter

| OpsOrch Concept | Datadog Mapping |
|-----------------|-----------------|
| `MetricQuery.Expression.MetricName` | Metric selector (`metric{tags}`) |
| `MetricQuery.Expression.Filters` | Tag filters (`label:value`, `!label:value`) |
| `MetricQuery.Scope` | Additional tag filters (`service:<name>`, `team:<name>`, `env:<name>`) |
| `MetricQuery.Expression.Aggregation` | Prefixed aggregation (`avg:`, `sum:`, etc.) |
| `MetricQuery.Expression.GroupBy` | `... by {label1,label2}` clause |
| `MetricQuery.Step` | Appended rollup (`.rollup(avg, <seconds>)`) |
| Raw Datadog query | Provide `query.Metadata["query"]` to bypass formatter |

**Describe Pagination**: Uses `/api/v2/metrics` with `filter[tags_configured]=service:<name>` when scoped. The adapter follows `meta.pagination.next_cursor` so all pages are returned.

### Alert Adapter

- Status mapping: OpsOrch statuses map to monitor `overall_state` via `MapOpsOrchStatusToDatadog`/`MapDatadogStatusToOpsOrch`.
- Severity mapping: Derived from Datadog priority (`P1..P5`) or inferred from state when priority is absent.
- Scope filters: OpsOrch scope fields are converted to monitor tag filters (e.g., `service:<name>`, `team:<name>`, `env:<name>`).
- Metadata: Monitor type, query, tags, thresholds, and raw monitor ID are preserved under `alert.Metadata` / `alert.Fields`.

### Service Adapter

- Calls the Service Definition API with paging (`page[size]`, `page[number]`).
- Uses the Datadog service `id` as both OpsOrch `Service.ID` and `Service.Name`.
- Extracts tags, team, and other metadata from whichever schema version (v2, v2.1, v2.2) the service uses and places them in `Service.Tags`.
- Stores schema origin, ingestion source, and the raw schema payload under `Service.Metadata` for troubleshooting.

## Using as Plugins

Build the plugin binaries and configure OpsOrch Core:

```bash
# Metric Plugin
OPSORCH_METRIC_PLUGIN=/path/to/bin/metricplugin
OPSORCH_METRIC_CONFIG='{"apiKey":"...","appKey":"..."}'

# Log Plugin
OPSORCH_LOG_PLUGIN=/path/to/bin/logplugin
OPSORCH_LOG_CONFIG='{"apiKey":"...","appKey":"..."}'

# Alert Plugin
OPSORCH_ALERT_PLUGIN=/path/to/bin/alertplugin
OPSORCH_ALERT_CONFIG='{"apiKey":"...","appKey":"..."}'

# Incident Plugin
OPSORCH_INCIDENT_PLUGIN=/path/to/bin/incidentplugin
OPSORCH_INCIDENT_CONFIG='{"apiKey":"...","appKey":"..."}'

# Service Plugin
OPSORCH_SERVICE_PLUGIN=/path/to/bin/serviceplugin
OPSORCH_SERVICE_CONFIG='{"apiKey":"...","appKey":"..."}'
```

## Using In-Process

Import the adapters for side effects to register them:

```go
import (
    _ "github.com/opsorch/opsorch-datadog-adapter/metric"
    _ "github.com/opsorch/opsorch-datadog-adapter/log"
    _ "github.com/opsorch/opsorch-datadog-adapter/alert"
    _ "github.com/opsorch/opsorch-datadog-adapter/incident"
    _ "github.com/opsorch/opsorch-datadog-adapter/service"
)
```

Then configure via environment variables:

```bash
OPSORCH_METRIC_PROVIDER=datadog OPSORCH_METRIC_CONFIG='{"apiKey":"...","appKey":"..."}'
```

## Testing

### Unit Tests

```bash
make test
```

### Integration Tests

Integration tests run against a real Datadog account. To seed test data (monitors, metrics, logs, incidents):

```bash
# Set environment variables
export DATADOG_API_KEY="your-api-key"
export DATADOG_APP_KEY="your-app-key"
export DATADOG_SITE="datadoghq.com"  # optional

# Seed test data (creates monitors, metrics, logs, and incidents)
make seed

# Wait 1-2 minutes for metrics and logs to be indexed

# Run all integration tests (should return data after seeding)
make integ

# Or run individual provider tests
make integ-metric
make integ-log
make integ-alert
make integ-incident
make integ-service
```

**Note**: After seeding, integration tests should return data for most queries:
- ✅ Metrics: `system.cpu.user` with env:test and env:prod tags (36 data points total)
- ✅ Logs: 7 log entries with log level keywords ([ERROR], [WARN], etc.) for severity filtering
- ✅ Alerts: 5 test monitors with service scope tags (using `monitor_tags` parameter)
- ⚠️ Incidents: 5 incidents (severity cannot be set via Create API - Datadog limitation)
- ✅ Services: 3 service definitions with team and environment tags (using v2.2 schema)

The seeding script creates:
- **Monitors**: 5 monitors with different severities, tags, and service scopes
- **Metrics**: Custom metrics with both env:test and env:prod tags (36 data points over last hour)
  - `system.cpu.user` with env:test (12 points)
  - `system.cpu.user` with env:prod (12 points)
  - `opsorch.test.cpu.usage` with env:test (12 points)
- **Logs**: 7 log entries with JSON-formatted messages containing explicit status field
  - Uses JSON format: `{"level":"error","status":"error","message":"..."}`
  - Datadog parses JSON and extracts the status field
  - 4 ERROR logs, 1 WARN log, 1 INFO log, 1 CRITICAL log
- **Incidents**: 5 incidents with timeline entries (severity defaults to "medium" due to API limitation)
- **Services**: 3 service definitions with team and environment tags (using v2.2 schema)

Known API Limitations:
- **Incident Severity**: Cannot be set via Create or Update API - requires complex custom field types not exposed in SDK
- **Log Severity**: Uses JSON-formatted messages with explicit `status` field for reliable severity filtering
- **Monitor Service Scope**: Uses `monitor_tags` parameter for filtering (not `tags`)

**Important**: After running `make seed`, wait 1-2 minutes for Datadog to index logs before running `make integ`

## License

Apache 2.0
