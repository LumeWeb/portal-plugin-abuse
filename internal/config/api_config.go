package config

import portalConfig "go.lumeweb.com/portal/config"

// APIConfig holds configuration for the Abuse API
type APIConfig struct {
	Enabled bool `json:"enabled"`
}

// Defaults returns the default configuration for the Abuse API
func (c APIConfig) Defaults() map[string]any {
	return map[string]any{
		"enabled": true,
	}
}

// Ensure APIConfig implements portalConfig.APIConfig
var _ portalConfig.APIConfig = (*APIConfig)(nil)
