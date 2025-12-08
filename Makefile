GO ?= go
GOCACHE ?= $(PWD)/.gocache
GOMODCACHE ?= $(PWD)/.gocache/mod
CACHE_ENV = GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE)

.PHONY: all fmt test build plugin integ integ-metric integ-log integ-alert integ-incident integ-service seed clean

all: test

fmt:
	$(GO) fmt ./...

test:
	$(CACHE_ENV) $(GO) test $(shell $(GO) list ./... | grep -v /integ)

build:
	$(CACHE_ENV) $(GO) build ./...

plugin:
	$(CACHE_ENV) $(GO) build -o bin/metricplugin ./cmd/metricplugin
	$(CACHE_ENV) $(GO) build -o bin/logplugin ./cmd/logplugin
	$(CACHE_ENV) $(GO) build -o bin/alertplugin ./cmd/alertplugin
	$(CACHE_ENV) $(GO) build -o bin/incidentplugin ./cmd/incidentplugin
	$(CACHE_ENV) $(GO) build -o bin/serviceplugin ./cmd/serviceplugin

integ-metric:
	@if [ -z "$$DATADOG_API_KEY" ]; then \
		echo "Error: DATADOG_API_KEY environment variable is required"; \
		exit 1; \
	fi
	@if [ -z "$$DATADOG_APP_KEY" ]; then \
		echo "Error: DATADOG_APP_KEY environment variable is required"; \
		exit 1; \
	fi
	$(CACHE_ENV) $(GO) run ./integ/metric.go

integ-log:
	@if [ -z "$$DATADOG_API_KEY" ]; then \
		echo "Error: DATADOG_API_KEY environment variable is required"; \
		exit 1; \
	fi
	@if [ -z "$$DATADOG_APP_KEY" ]; then \
		echo "Error: DATADOG_APP_KEY environment variable is required"; \
		exit 1; \
	fi
	$(CACHE_ENV) $(GO) run ./integ/log.go

integ-alert:
	@if [ -z "$$DATADOG_API_KEY" ]; then \
		echo "Error: DATADOG_API_KEY environment variable is required"; \
		exit 1; \
	fi
	@if [ -z "$$DATADOG_APP_KEY" ]; then \
		echo "Error: DATADOG_APP_KEY environment variable is required"; \
		exit 1; \
	fi
	$(CACHE_ENV) $(GO) run ./integ/alert.go

integ-incident:
	@if [ -z "$$DATADOG_API_KEY" ]; then \
		echo "Error: DATADOG_API_KEY environment variable is required"; \
		exit 1; \
	fi
	@if [ -z "$$DATADOG_APP_KEY" ]; then \
		echo "Error: DATADOG_APP_KEY environment variable is required"; \
		exit 1; \
	fi
	$(CACHE_ENV) $(GO) run ./integ/incident.go

integ-service:
	@if [ -z "$$DATADOG_API_KEY" ]; then \
		echo "Error: DATADOG_API_KEY environment variable is required"; \
		exit 1; \
	fi
	@if [ -z "$$DATADOG_APP_KEY" ]; then \
		echo "Error: DATADOG_APP_KEY environment variable is required"; \
		exit 1; \
	fi
	$(CACHE_ENV) $(GO) run ./integ/service.go

integ: integ-metric integ-log integ-alert integ-incident integ-service

seed:
	@if [ -z "$$DATADOG_API_KEY" ]; then \
		echo "Error: DATADOG_API_KEY environment variable is required"; \
		exit 1; \
	fi
	@if [ -z "$$DATADOG_APP_KEY" ]; then \
		echo "Error: DATADOG_APP_KEY environment variable is required"; \
		exit 1; \
	fi
	$(CACHE_ENV) $(GO) run ./integ/seed.go

clean:
	rm -rf $(GOCACHE) bin
