package config

import (
	"go.lumeweb.com/portal/config"
)

var _ config.Defaults = (*ScanConfig)(nil)
var _ config.Defaults = (*ClamAVConfig)(nil)

// ClamAVConfig holds ClamAV scanner configuration
type ClamAVConfig struct {
	Enabled    bool   `config:"enabled"`
	Network    string `config:"network"`
	Address    string `config:"address"`
	MaxWorkers int    `config:"max_workers"`
}

// ScanConfig holds scan service configuration
type ScanConfig struct {
	MaxFileSize int64        `config:"max_file_size"`
	ClamAV      ClamAVConfig `config:"clamav"`
}

// Implement config.ServiceConfig interface
func (c *ScanConfig) Defaults() map[string]any {
	return map[string]any{
		"max_file_size": int64(10485760), // 10MB
	}
}

func (c *ClamAVConfig) Defaults() map[string]any {
	return map[string]any{
		"enabled":     true,
		"network":     "unix",
		"address":     "/var/run/clamav/clamd.ctl",
		"max_workers": 10,
	}
}
