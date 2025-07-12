package config

import (
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/portal/config"
)

var _ config.Defaults = (*ScanConfig)(nil)
var _ config.Defaults = (*ClamAVConfig)(nil)
var _ config.ConfigSchemaProvider = (*ScanConfig)(nil)
var _ config.ConfigSchemaProvider = (*ClamAVConfig)(nil)

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

func (c *ScanConfig) Schema() z.ZogSchema {
	return z.Struct(z.Shape{
		"MaxFileSize": z.Int64().GT(0),
		"ClamAV": z.Struct(z.Shape{
			"Enabled":    z.Bool(),
			"Network":    z.String(),
			"Address":    z.String(),
			"MaxWorkers": z.Int().GT(0),
		}),
	})
}

func (c *ClamAVConfig) Schema() z.ZogSchema {
	return z.Struct(z.Shape{
		"Enabled":    z.Bool(),
		"Network":    z.String(),
		"Address":    z.String(),
		"MaxWorkers": z.Int().GT(0),
	})
}

// Implement config.ServiceConfig interface
func (c *ScanConfig) Defaults() map[string]any {
	return map[string]any{
		"max_file_size": int64(10485760), // 10MB
	}
}

func (c *ClamAVConfig) Defaults() map[string]any {
	return map[string]any{
		"Enabled":    true,
		"Network":    "unix",
		"Address":    "/var/run/clamav/clamd.ctl",
		"MaxWorkers": 10,
	}
}
