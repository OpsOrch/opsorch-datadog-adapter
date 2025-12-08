package common

import (
	"fmt"
	"strings"
)

// Config holds shared Datadog configuration for all providers.
type Config struct {
	APIKey string // DD-API-KEY header value
	AppKey string // DD-APPLICATION-KEY header value
	Site   string // Datadog site (e.g., datadoghq.com, datadoghq.eu)
	Source string // Source identifier for metadata
}

// DefaultSite is the default Datadog site.
const DefaultSite = "datadoghq.com"

// DefaultSource is the default source identifier.
const DefaultSource = "datadog"

// ValidSites contains all supported Datadog sites.
var ValidSites = map[string]string{
	"datadoghq.com":     "https://api.datadoghq.com",
	"us3.datadoghq.com": "https://api.us3.datadoghq.com",
	"us5.datadoghq.com": "https://api.us5.datadoghq.com",
	"datadoghq.eu":      "https://api.datadoghq.eu",
	"ap1.datadoghq.com": "https://api.ap1.datadoghq.com",
}

// ParseConfig extracts configuration from a map.
func ParseConfig(cfg map[string]any) Config {
	out := Config{
		Site:   DefaultSite,
		Source: DefaultSource,
	}

	if v, ok := cfg["apiKey"].(string); ok {
		out.APIKey = strings.TrimSpace(v)
	}
	if v, ok := cfg["appKey"].(string); ok {
		out.AppKey = strings.TrimSpace(v)
	}
	if v, ok := cfg["site"].(string); ok && v != "" {
		out.Site = strings.TrimSpace(v)
	}
	if v, ok := cfg["source"].(string); ok && v != "" {
		out.Source = strings.TrimSpace(v)
	}

	return out
}

// ValidateConfig checks that required fields are present and valid.
func ValidateConfig(cfg Config) error {
	if cfg.APIKey == "" {
		return fmt.Errorf("%w: apiKey is required", ErrInvalidConfig)
	}
	if cfg.AppKey == "" {
		return fmt.Errorf("%w: appKey is required", ErrInvalidConfig)
	}
	if cfg.Site != "" && !IsValidSite(cfg.Site) {
		return fmt.Errorf("%w: invalid site %q, must be one of: %s",
			ErrInvalidConfig, cfg.Site, strings.Join(SiteNames(), ", "))
	}
	return nil
}

// IsValidSite checks if a site identifier is valid.
func IsValidSite(site string) bool {
	_, ok := ValidSites[site]
	return ok
}

// SiteNames returns all valid site names.
func SiteNames() []string {
	names := make([]string, 0, len(ValidSites))
	for name := range ValidSites {
		names = append(names, name)
	}
	return names
}

// BaseURL returns the API base URL for the configured site.
func (c Config) BaseURL() string {
	if url, ok := ValidSites[c.Site]; ok {
		return url
	}
	return ValidSites[DefaultSite]
}
