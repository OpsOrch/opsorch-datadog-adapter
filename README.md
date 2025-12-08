# OpsOrch Datadog Adapter

[![Version](https://img.shields.io/github/v/release/opsorch/opsorch-datadog-adapter)](https://github.com/opsorch/opsorch-datadog-adapter/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/opsorch/opsorch-datadog-adapter)](https://github.com/opsorch/opsorch-datadog-adapter/blob/main/go.mod)
[![License](https://img.shields.io/github/license/opsorch/opsorch-datadog-adapter)](https://github.com/opsorch/opsorch-datadog-adapter/blob/main/LICENSE)
[![CI](https://github.com/opsorch/opsorch-datadog-adapter/workflows/CI/badge.svg)](https://github.com/opsorch/opsorch-datadog-adapter/actions)

This adapter integrates OpsOrch with Datadog's observability platform using the official [Datadog Go SDK v2](https://github.com/DataDog/datadog-api-client-go), enabling metric queries, log searches, alert monitoring, incident management, and service discovery.

## Capabilities

This adapter provides five capabilities:

1. **Metric Adapter**: Query time-series metrics via Datadog Metrics API (v2)
2. **Log Adapter**: Search and retrieve logs via Datadog Logs API (v2)
3. **Alert Adapter**: Query Datadog monitors via Monitors API (v1)
4. **Incident Adapter**: Manage Datadog incidents via Incidents API (v2)
5. **Service Adapter**: Query services from Datadog Service Catalog API (v2)

## Features

### Metrics
- **Metric Query**: Execute Datadog metric queries via structured expressions or raw query strings
- **Metric Discovery**: List all available metrics with pagination support
- **QueryScope Support**: Automatically map service/team/environment to Datadog tags
- **Aggregation**: Support for aggregation functions (avg, sum, max, min, count)
- **Filtering**: Tag-based filtering with label operators
- **Rollup Functions**: Automatic rollup based on query step size

### Logs
- **Log Search**: Search logs using Datadog query syntax with full-text search
- **Severity Filtering**: Filter by log severity levels (error, warn, info, debug)
- **Structured Filters**: Field-level filters with operators (equality, negation, wildcards)
- **Scope Filtering**: Filter by service, team, environment metadata
- **Pagination**: Automatic cursor-based pagination with a 100-log default limit (override via `limit`)
- **Result Normalization**: Returns standardized OpsOrch LogEntry objects

### Alerts
- **Alert Query**: Fetch Datadog monitors with status and severity filtering
- **Alert Details**: Get individual monitor details by ID
- **Status Filtering**: Filter by monitor state (Alert, Warn, No Data, OK)
- **Severity Mapping**: Automatic severity mapping from Datadog priority levels
- **Scope Filtering**: Filter monitors by service, team, environment tags

### Incidents
- **Create Incidents**: Create new Datadog incidents with title and customer impact metadata
- **Query Incidents**: Search incidents with client-side filters for status, severity, and scope
- **Get Incident**: Retrieve individual incident details
- **Update Incidents**: Modify supported incident fields (title, customer impact metadata)
- **Timeline Operations**: Retrieve and append timeline entries (via Todos API)
- **Customer Impact Tracking**: Track customer impact with free-form scope descriptions

### Services
- **Service Discovery**: List all services from Datadog Service Catalog
- **Service Query**: Filter services by tags and metadata
- **Schema Support**: Supports service definition schemas v2, v2.1, and v2.2
- **Metadata Extraction**: Extract team, tags, and other metadata from service definitions

### Version Compatibility

- **Adapter Version**: 0.1.0
- **Requires OpsOrch Core**: >=0.1.0
- **Datadog SDK**: v2 (github.com/DataDog/datadog-api-client-go/v2)
- **Go Version**: 1.22+

## Configuration

All adapters share the same base configuration:

| Field | Type | Required | Description | Default |
|-------|------|----------|-------------|---------|
| `apiKey` | string | Yes | Datadog API key | - |
| `appKey` | string | Yes | Datadog Application key | - |
| `site` | string | No | Datadog site (e.g., `datadoghq.com`, `datadoghq.eu`) | `datadoghq.com` |
| `source` | string | No | Source identifier for metadata | `datadog` |

### Supported Sites

- `datadoghq.com` (US1, default)
- `us3.datadoghq.com` (US3)
- `us5.datadoghq.com` (US5)
- `datadoghq.eu` (EU1)
- `ap1.datadoghq.com` (AP1)

### Authentication Setup

#### 1. Obtain Datadog API Key

1. Log in to your Datadog account
2. Navigate to **Organization Settings** → **API Keys**
3. Click **New Key** or copy an existing API key
4. Store the key securely

#### 2. Obtain Datadog Application Key

1. In Datadog, navigate to **Organization Settings** → **Application Keys**
2. Click **New Key**
3. Give it a descriptive name (e.g., `opsorch-adapter`)
4. Copy the application key
5. Store the key securely

⚠️ **Important**: Store both keys in your secret manager (Vault, AWS SSM, Kubernetes secrets, etc.) and inject them into OpsOrch config at runtime.

#### 3. Test Your Credentials (Optional)

Verify your credentials work:

```bash
curl -X GET "https://api.datadoghq.com/api/v1/validate" \
  -H "DD-API-KEY: ${DD_API_KEY}" \
  -H "DD-APPLICATION-KEY: ${DD_APP_KEY}"
```

If credentials are valid, you'll receive a `{"valid": true}` response.

### Example Configuration

**JSON format:**
```json
{
  "apiKey": "your-api-key",
  "appKey": "your-application-key",
  "site": "datadoghq.com",
  "source": "datadog"
}
```

**Environment variables (Metric):**
```bash
export OPSORCH_METRIC_PLUGIN=/path/to/bin/metricplugin
export OPSORCH_METRIC_CONFIG='{"apiKey":"...","appKey":"...","site":"datadoghq.com"}'
```

**Environment variables (Log):**
```bash
export OPSORCH_LOG_PLUGIN=/path/to/bin/logplugin
export OPSORCH_LOG_CONFIG='{"apiKey":"...","appKey":"...","site":"datadoghq.com"}'
```

**Environment variables (Alert):**
```bash
export OPSORCH_ALERT_PLUGIN=/path/to/bin/alertplugin
export OPSORCH_ALERT_CONFIG='{"apiKey":"...","appKey":"...","site":"datadoghq.com"}'
```

**Environment variables (Incident):**
```bash
export OPSORCH_INCIDENT_PLUGIN=/path/to/bin/incidentplugin
export OPSORCH_INCIDENT_CONFIG='{"apiKey":"...","appKey":"...","site":"datadoghq.com"}'
```

**Environment variables (Service):**
```bash
export OPSORCH_SERVICE_PLUGIN=/path/to/bin/serviceplugin
export OPSORCH_SERVICE_CONFIG='{"apiKey":"...","appKey":"...","site":"datadoghq.com"}'
```

## Field Mapping

### Log Adapter

#### Query Mapping

| OpsOrch Field | Datadog Behavior | Notes |
|---------------|------------------|-------|
| `expression.search` | Literal search string appended to query | Full-text search across log messages |
| `expression.severityIn` | Converted to `status:(...)` filter | Uses `mapSeverityToDatadog` mapping |
| `expression.filters` | Rendered into Datadog clauses | Supports `field:value`, negative, and wildcard filters |
| `scope.service` | Adds `service:<name>` filter | Automatic tag filter |
| `scope.team` | Adds `team:<name>` filter | Automatic tag filter |
| `scope.environment` | Adds `env:<name>` filter | Automatic tag filter |
| `metadata` (string values) | Added as `key:value` clauses | Additional filters |

#### Pagination

- **API**: Uses `meta.page.after` cursor from responses
- **Request**: Sends `page[cursor]` and `page[limit]`
- **Defaults**: 1,000 logs per request, 100 total unless `query.Limit` overrides
- **Loop**: Continues until cursor is empty, limit reached, or API returns no data

#### Response Normalization

| Datadog Field | OpsOrch Field | Transformation |
|---------------|---------------|----------------|
| `attributes.message` | `Message` | Direct mapping |
| `attributes.status` | `Severity` | Mapped back to OpsOrch levels |
| `attributes.service` | `Service` | Direct mapping |
| `attributes.tags` | `Labels` | Flattened tags array |
| `attributes.attributes` | `Fields` | Raw Datadog attributes |

**Metadata fields:**
- `source`: Always "datadog"
- `log_id`: Datadog log ID
- `host`: Log source host
- `timestamp`: Log timestamp
- `tags`: Full tags array

### Incident Adapter

#### Query Mapping

| OpsOrch Field | Datadog Field | Transformation | Notes |
|---------------|---------------|----------------|-------|
| `Scope.Service` | Client-side filter on `Service` field | Matches `incident.Service` | Extracted from custom fields |
| `Scope.Team` | Client-side filter on `Fields["team"]` | Supports single and multi-value | Matches ANY team if multiple |
| `Scope.Environment` | Client-side filter on `Fields["environment"]` or `Fields["env"]` | Supports single and multi-value | Matches ANY environment if multiple |
| `Severities` | Client-side filter | Maps OpsOrch severities to Datadog SEV levels | - |
| `Statuses` | Client-side filter | Maps OpsOrch statuses to Datadog states | - |

#### Response Normalization

| Datadog Field | OpsOrch Field | Transformation | Notes |
|---------------|---------------|----------------|-------|
| `id` | `ID` | Direct mapping | Incident UUID |
| `attributes.title` | `Title` | Direct mapping | Incident title |
| `attributes.state` | `Status` | Mapped: active→open, stable→investigating, resolved→resolved | - |
| `attributes.severity` | `Severity` | Mapped: SEV-1→critical, SEV-2→high, SEV-3→medium, SEV-4→low, SEV-5→info | - |
| `attributes.fields.service` | `Service` | Extracted from custom fields (first value if multiple) | - |
| `attributes.created` | `CreatedAt` | ISO 8601 timestamp | - |
| `attributes.modified` | `UpdatedAt` | ISO 8601 timestamp | - |

**Metadata fields:**
- `source`: Configured source (default: "datadog")
- `customer_impacted`: Whether customers were impacted (bool)
- `customer_impact_scope`: Free-form string describing impact
- `public_id`: Human-readable incident ID
- `detected`: When incident was detected
- `resolved`: When incident was resolved

**Custom fields** (stored in `Fields` map):
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

#### Query Mapping

| OpsOrch Field | Datadog Mapping | Notes |
|---------------|-----------------|-------|
| `MetricQuery.Expression.MetricName` | Metric selector (`metric{tags}`) | Base metric name |
| `MetricQuery.Expression.Filters` | Tag filters (`label:value`, `!label:value`) | Label-based filtering |
| `MetricQuery.Scope` | Additional tag filters (`service:<name>`, `team:<name>`, `env:<name>`) | Automatic scope mapping |
| `MetricQuery.Expression.Aggregation` | Prefixed aggregation (`avg:`, `sum:`, etc.) | Aggregation function |
| `MetricQuery.Expression.GroupBy` | `... by {label1,label2}` clause | Group by labels |
| `MetricQuery.Step` | Appended rollup (`.rollup(avg, <seconds>)`) | Time-based rollup |
| `query.Metadata["query"]` | Raw Datadog query | Bypasses formatter if provided |

**Describe Pagination**: Uses `/api/v2/metrics` with `filter[tags_configured]=service:<name>` when scoped. The adapter follows `meta.pagination.next_cursor` to return all pages.

### Alert Adapter

#### Query Mapping

| OpsOrch Field | Datadog Mapping | Notes |
|---------------|-----------------|-------|
| `Statuses` | Monitor `overall_state` | Maps via `MapOpsOrchStatusToDatadog` |
| `Severities` | Derived from priority (`P1..P5`) or inferred from state | - |
| `Scope.Service` | Monitor tag filter `service:<name>` | - |
| `Scope.Team` | Monitor tag filter `team:<name>` | - |
| `Scope.Environment` | Monitor tag filter `env:<name>` | - |

**Metadata fields:**
- Monitor type, query, tags, thresholds preserved under `alert.Metadata` / `alert.Fields`
- Raw monitor ID stored in metadata

### Service Adapter

#### Query Mapping

| OpsOrch Field | Datadog Mapping | Notes |
|---------------|-----------------|-------|
| Service query | Calls Service Definition API with paging | Uses `page[size]`, `page[number]` |
| Service ID | Uses Datadog service `id` | Maps to both `Service.ID` and `Service.Name` |

#### Response Normalization

- Extracts tags, team, and metadata from schema version (v2, v2.1, v2.2)
- Places extracted data in `Service.Tags`
- Stores schema origin, ingestion source, and raw schema payload in `Service.Metadata`

## Usage

### In-Process Mode

Import the adapters for side effects to register them with OpsOrch Core:

```go
import (
    _ "github.com/opsorch/opsorch-datadog-adapter/metric"
    _ "github.com/opsorch/opsorch-datadog-adapter/log"
    _ "github.com/opsorch/opsorch-datadog-adapter/alert"
    _ "github.com/opsorch/opsorch-datadog-adapter/incident"
    _ "github.com/opsorch/opsorch-datadog-adapter/service"
)
```

Configure via environment variables:

```bash
export OPSORCH_METRIC_PROVIDER=datadog
export OPSORCH_METRIC_CONFIG='{"apiKey":"...","appKey":"..."}'

export OPSORCH_LOG_PROVIDER=datadog
export OPSORCH_LOG_CONFIG='{"apiKey":"...","appKey":"..."}'

export OPSORCH_ALERT_PROVIDER=datadog
export OPSORCH_ALERT_CONFIG='{"apiKey":"...","appKey":"..."}'

export OPSORCH_INCIDENT_PROVIDER=datadog
export OPSORCH_INCIDENT_CONFIG='{"apiKey":"...","appKey":"..."}'

export OPSORCH_SERVICE_PROVIDER=datadog
export OPSORCH_SERVICE_CONFIG='{"apiKey":"...","appKey":"..."}'
```

### Plugin Mode

Build the plugin binaries:

```bash
make plugin
```

This builds five plugin binaries in `./bin/`:
- `metricplugin`
- `logplugin`
- `alertplugin`
- `incidentplugin`
- `serviceplugin`

Configure OpsOrch Core to use the plugins:

```bash
# Metric Plugin
export OPSORCH_METRIC_PLUGIN=/path/to/bin/metricplugin
export OPSORCH_METRIC_CONFIG='{"apiKey":"...","appKey":"..."}'

# Log Plugin
export OPSORCH_LOG_PLUGIN=/path/to/bin/logplugin
export OPSORCH_LOG_CONFIG='{"apiKey":"...","appKey":"..."}'

# Alert Plugin
export OPSORCH_ALERT_PLUGIN=/path/to/bin/alertplugin
export OPSORCH_ALERT_CONFIG='{"apiKey":"...","appKey":"..."}'

# Incident Plugin
export OPSORCH_INCIDENT_PLUGIN=/path/to/bin/incidentplugin
export OPSORCH_INCIDENT_CONFIG='{"apiKey":"...","appKey":"..."}'

# Service Plugin
export OPSORCH_SERVICE_PLUGIN=/path/to/bin/serviceplugin
export OPSORCH_SERVICE_CONFIG='{"apiKey":"...","appKey":"..."}'
```

### Docker Deployment

Download pre-built plugin binaries from [GitHub Releases](https://github.com/opsorch/opsorch-datadog-adapter/releases):

```dockerfile
FROM ghcr.io/opsorch/opsorch-core:latest
WORKDIR /opt/opsorch

# Download all plugin binaries
ADD https://github.com/opsorch/opsorch-datadog-adapter/releases/download/v0.1.0/metricplugin-linux-amd64 ./plugins/metricplugin
ADD https://github.com/opsorch/opsorch-datadog-adapter/releases/download/v0.1.0/logplugin-linux-amd64 ./plugins/logplugin
ADD https://github.com/opsorch/opsorch-datadog-adapter/releases/download/v0.1.0/alertplugin-linux-amd64 ./plugins/alertplugin
ADD https://github.com/opsorch/opsorch-datadog-adapter/releases/download/v0.1.0/incidentplugin-linux-amd64 ./plugins/incidentplugin
ADD https://github.com/opsorch/opsorch-datadog-adapter/releases/download/v0.1.0/serviceplugin-linux-amd64 ./plugins/serviceplugin

RUN chmod +x ./plugins/*

# Configure plugins
ENV OPSORCH_METRIC_PLUGIN=/opt/opsorch/plugins/metricplugin \
    OPSORCH_LOG_PLUGIN=/opt/opsorch/plugins/logplugin \
    OPSORCH_ALERT_PLUGIN=/opt/opsorch/plugins/alertplugin \
    OPSORCH_INCIDENT_PLUGIN=/opt/opsorch/plugins/incidentplugin \
    OPSORCH_SERVICE_PLUGIN=/opt/opsorch/plugins/serviceplugin
```

## Development

### Prerequisites

- Go 1.22 or later
- Datadog account with API access
- Datadog API key and Application key

### Building

```bash
# Download dependencies (including Datadog SDK)
go mod download

# Run unit tests
make test

# Build all packages
make build

# Build plugin binaries
make plugin

# Run integration tests (requires credentials)
make integ
```

### Testing

**Unit Tests:**
```bash
make test
```

**Integration Tests:**

Integration tests run against a real Datadog account. To seed test data (monitors, metrics, logs, incidents, services):

**Prerequisites:**
- A Datadog account with API access
- API key and Application key
- Permissions to create monitors, metrics, logs, incidents, and services

**Setup:**
```bash
# Set environment variables
export DATADOG_API_KEY="your-api-key"
export DATADOG_APP_KEY="your-app-key"
export DATADOG_SITE="datadoghq.com"  # optional

# Seed test data (creates monitors, metrics, logs, and incidents, services)
make seed

# Wait 1-2 minutes for metrics and logs to be indexed

# Run all integration tests
make integ

# Or run individual provider tests
make integ-metric
make integ-log
make integ-alert
make integ-incident
make integ-service
```

**What the tests do:**

The seeding script creates:
- **Monitors**: 5 monitors with different severities, tags, and service scopes
- **Metrics**: Custom metrics with both env:test and env:prod tags (36 data points over last hour)
  - `system.cpu.user` with env:test (12 points)
  - `system.cpu.user` with env:prod (12 points)
  - `opsorch.test.cpu.usage` with env:test (12 points)
- **Logs**: 7 log entries with JSON-formatted messages containing explicit status field
  - Uses JSON format: `{"level":"error","status":"error","message":"..."}`
  - 4 ERROR logs, 1 WARN log, 1 INFO log, 1 CRITICAL log
- **Incidents**: 5 incidents with timeline entries (severity defaults to "medium" due to API limitation)
- **Services**: 3 service definitions with team and environment tags (using v2.2 schema)

**Expected behavior:**
- ✅ Metrics: `system.cpu.user` with env:test and env:prod tags (36 data points total)
- ✅ Logs: 7 log entries with severity filtering support
- ✅ Alerts: 5 test monitors with service scope tags
- ⚠️ Incidents: 5 incidents (severity cannot be set via Create API - Datadog limitation)
- ✅ Services: 3 service definitions with team and environment tags

**Known API Limitations:**
- **Incident Severity**: Cannot be set via Create or Update API
- **Log Severity**: Uses JSON-formatted messages with explicit `status` field for reliable filtering
- **Monitor Service Scope**: Uses `monitor_tags` parameter for filtering (not `tags`)

**Important**: After running `make seed`, wait 1-2 minutes for Datadog to index logs before running `make integ`

### Project Structure

```
opsorch-datadog-adapter/
├── metric/                    # Metric provider implementation
│   ├── datadog_provider.go
│   └── datadog_provider_test.go
├── log/                       # Log provider implementation
│   ├── datadog_provider.go
│   └── datadog_provider_test.go
├── alert/                     # Alert provider implementation
│   ├── datadog_provider.go
│   └── datadog_provider_test.go
├── incident/                  # Incident provider implementation
│   ├── datadog_provider.go
│   └── datadog_provider_test.go
├── service/                   # Service provider implementation
│   ├── datadog_provider.go
│   └── datadog_provider_test.go
├── cmd/
│   ├── metricplugin/         # Metric plugin entrypoint
│   ├── logplugin/            # Log plugin entrypoint
│   ├── alertplugin/          # Alert plugin entrypoint
│   ├── incidentplugin/       # Incident plugin entrypoint
│   └── serviceplugin/        # Service plugin entrypoint
├── integ/                    # Integration tests
│   ├── seed.go               # Integration test seeding
│   ├── metric.go
│   ├── log.go
│   ├── alert.go
│   ├── incident.go
│   └── service.go
├── Makefile
└── README.md
```

**Key Components:**

- **metric/datadog_provider.go**: Implements metric.Provider interface, handles Datadog metric query building
- **log/datadog_provider.go**: Implements log.Provider interface, handles log search with pagination
- **alert/datadog_provider.go**: Implements alert.Provider interface, queries Datadog monitors
- **incident/datadog_provider.go**: Implements incident.Provider interface, manages Datadog incidents
- **service/datadog_provider.go**: Implements service.Provider interface, queries Service Catalog
- **integ/seed.go**: Script to seed test data in Datadog for integration testing

## CI/CD & Pre-Built Binaries

The repository includes GitHub Actions workflows:

- **CI** (`ci.yml`): Runs tests and linting on every push/PR to main
- **Release** (`release.yml`): Manual workflow that:
  - Runs tests and linting
  - Creates version tags (patch/minor/major)
  - Builds multi-arch binaries for all five plugins (linux-amd64, linux-arm64, darwin-amd64, darwin-arm64)
  - Publishes binaries as GitHub release assets

### Downloading Pre-Built Binaries

Pre-built plugin binaries are available from [GitHub Releases](https://github.com/opsorch/opsorch-datadog-adapter/releases).

**Supported platforms:**
- Linux (amd64, arm64)
- macOS (amd64, arm64)

**Available binaries:**
- `metricplugin-{platform}-{arch}`
- `logplugin-{platform}-{arch}`
- `alertplugin-{platform}-{arch}`
- `incidentplugin-{platform}-{arch}`
- `serviceplugin-{platform}-{arch}`

## Plugin RPC Contract

OpsOrch Core communicates with the plugins over stdin/stdout using JSON-RPC.

### Message Format

**Request:**
```json
{
  "method": "{capability}.{operation}",
  "config": { /* decrypted configuration */ },
  "payload": { /* method-specific request body */ }
}
```

**Response:**
```json
{
  "result": { /* method-specific result */ },
  "error": "optional error message"
}
```

### Configuration Injection

The `config` field contains the decrypted configuration map from `OPSORCH_{CAPABILITY}_CONFIG`. The plugin receives this on every request, so it never stores secrets on disk.

### Supported Methods

#### Metric Plugin

- `metric.query`: Execute a metric query
- `metric.describe`: List available metrics

**Example - metric.query:**
```json
{
  "method": "metric.query",
  "config": {"apiKey": "...", "appKey": "...", "site": "datadoghq.com"},
  "payload": {
    "expression": {"metricName": "system.cpu.user"},
    "start": "2024-01-01T00:00:00Z",
    "end": "2024-01-01T01:00:00Z",
    "step": 60
  }
}
```

#### Log Plugin

- `log.query`: Search logs

**Example - log.query:**
```json
{
  "method": "log.query",
  "config": {"apiKey": "...", "appKey": "...", "site": "datadoghq.com"},
  "payload": {
    "start": "2024-01-01T00:00:00Z",
    "end": "2024-01-01T01:00:00Z",
    "expression": {
      "search": "error",
      "severityIn": ["error", "critical"]
    }
  }
}
```

#### Alert Plugin

- `alert.query`: Query monitors
- `alert.get`: Get monitor details

**Example - alert.query:**
```json
{
  "method": "alert.query",
  "config": {"apiKey": "...", "appKey": "...", "site": "datadoghq.com"},
  "payload": {
    "statuses": ["Alert", "Warn"],
    "scope": {"service": "api"}
  }
}
```

#### Incident Plugin

- `incident.query`: Query incidents
- `incident.get`: Get incident details
- `incident.create`: Create new incident
- `incident.update`: Update incident
- `incident.timeline.get`: Get incident timeline
- `incident.timeline.append`: Append timeline entry

**Example - incident.create:**
```json
{
  "method": "incident.create",
  "config": {"apiKey": "...", "appKey": "...", "site": "datadoghq.com"},
  "payload": {
    "title": "API service outage",
    "metadata": {
      "customer_impacted": true,
      "customer_impact_scope": "All API users affected"
    }
  }
}
```

#### Service Plugin

- `service.query`: Query services

**Example - service.query:**
```json
{
  "method": "service.query",
  "config": {"apiKey": "...", "appKey": "...", "site": "datadoghq.com"},
  "payload": {
    "scope": {"team": "platform"}
  }
}
```

## SDK Features

The adapter leverages the following Datadog SDK features:

- **Type Safety**: All API requests and responses use strongly-typed SDK models
- **Authentication**: Automatic API key and application key injection via SDK context
- **Retry Logic**: Built-in exponential backoff and retry handling for transient failures
- **Error Handling**: Structured error types with detailed error messages
- **Site Configuration**: Automatic server URL configuration based on Datadog site

## Security Considerations

1. **Never log credentials**: Avoid logging the config, API key, or Application key in application logs
2. **Rotate keys regularly**: Rotate both API and Application keys at the cadence required by your organization's security policy
3. **Use environment variables**: Store config in secure environment variables or secrets management systems
4. **Restrict file permissions**: If storing config in files, ensure proper file permissions (e.g., 0600)
5. **Use separate keys per environment**: Use different API/Application keys for dev, staging, and production
6. **Validate TLS certificates**: The adapter validates TLS certificates by default; do not disable in production

## License

Apache 2.0

See LICENSE file for details.
