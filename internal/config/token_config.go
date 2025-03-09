package config

// TokenConfig holds token service configuration
type TokenConfig struct {
	Secret string `config:"secret" default:""`
}

// Implement config.ServiceConfig interface
func (c *TokenConfig) Defaults() map[string]any {
	return map[string]any{
		"secret": "",
	}
}
