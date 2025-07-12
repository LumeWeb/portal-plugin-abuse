package config

import (
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/portal/config"
)

var _ config.Defaults = (*APIConfig)(nil)
var _ config.ConfigSchemaProvider = (*APIConfig)(nil)

// APIConfig holds configuration for the Abuse API
type APIConfig struct {
	Enabled bool `json:"enabled"`
}

func (c *APIConfig) Schema() z.ZogSchema {
	return z.Struct(z.Shape{
		"Enabled": z.Bool(),
	})
}

// Defaults returns the default configuration for the Abuse API
func (c APIConfig) Defaults() map[string]any {
	return map[string]any{
		"enabled": true,
	}
}
