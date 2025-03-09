// Package config provides configuration structures and defaults for the template plugin
package config

import (
	"go.lumeweb.com/portal/config"
)

// Verify Config implements config.ProtocolConfig interface
var _ config.ProtocolConfig = (*Config)(nil)

// Verify APIConfig implements config.APIConfig interface 
var _ config.APIConfig = (*APIConfig)(nil)

// Config defines all configuration options for the template plugin
type Config struct {
	StoragePath  string `config:"storage_path"`  // Path to store protocol data
	MaxItems     int    `config:"max_items"`     // Maximum number of items to store
	CacheEnabled bool   `config:"cache_enabled"` // Whether to enable caching
	API          APIConfig `config:"api"`        // API-specific configuration
}

// APIConfig defines the API-specific configuration options
type APIConfig struct {
	ItemsPerPage int `config:"items_per_page"` // Number of items to return per page
	SearchLimit  int `config:"search_limit"`   // Maximum number of search results
}

// Defaults provides default configuration values for API settings
func (a APIConfig) Defaults() map[string]any {
	return map[string]any{
		"items_per_page": 10,  // Default page size
		"search_limit":   100, // Default search results limit
	}
}

// Defaults provides the default configuration values
func (c Config) Defaults() map[string]any {
	return map[string]any{
		"storage_path":  "data/template",
		"max_items":     1000,
		"cache_enabled": true,
		"api": map[string]any{
			"items_per_page": 10,
			"search_limit":   100,
			"subdomain":      "template-plugin",
		},
	}
}
