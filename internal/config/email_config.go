package config

import (
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/portal/config"
)

var _ config.Defaults = (*EmailConfig)(nil)
var _ config.ConfigSchemaProvider = (*EmailConfig)(nil)

// ProviderTemplateConfig defines configuration for a provider email template
type ProviderTemplateConfig struct {
	Name            string   `config:"name"`
	Enabled         bool     `config:"enabled"`
	Priority        int      `config:"priority"`         // Higher values = lower priority
	DomainPatterns  []string `config:"domain_patterns"`  // Regular expressions to match email domains
	HeaderPatterns  []string `config:"header_patterns"`  // Regular expressions to match header names
	ContentPatterns []string `config:"content_patterns"` // Regular expressions to match content
}

// EmailConfig holds email service configuration
type EmailConfig struct {

	// IMAP settings for receiving emails
	ReceiveEnabled   bool     `config:"receive_enabled"`
	IMAPHost         string   `config:"imap_host"`
	IMAPPort         int      `config:"imap_port"`
	IMAPUser         string   `config:"imap_user"`
	IMAPPassword     string   `config:"imap_password"`
	IMAPMailbox      string   `config:"imap_mailbox"`
	PollInterval     int      `config:"poll_interval"` // In seconds, defaults to 5 minutes
	ReceiveAddresses []string `config:"receive_addresses"`

	// Provider template settings
	StandardProviders []string                 `config:"standard_providers"`
	CustomProviders   []ProviderTemplateConfig `config:"custom_providers"`
}

func (c *EmailConfig) Schema() z.ZogSchema {
	return z.Struct(z.Shape{
		"ReceiveEnabled":    z.Bool(),
		"IMAPHost":          z.String().Optional(),
		"IMAPPort":          z.Int().GTE(1).LTE(65535),
		"IMAPUser":          z.String().Optional(),
		"IMAPPassword":      z.String().Optional(),
		"IMAPMailbox":       z.String(),
		"PollInterval":      z.Int().GT(0),
		"ReceiveAddresses":  z.Slice(z.String().Email()).Optional(),
		"StandardProviders": z.Slice(z.String()).Optional(),
		"CustomProviders": z.Slice(
			z.Struct(z.Shape{
				"Name":            z.String().Required(),
				"Enabled":         z.Bool(),
				"Priority":        z.Int(),
				"DomainPatterns":  z.Slice(z.String()).Optional(),
				"HeaderPatterns":  z.Slice(z.String()).Optional(),
				"ContentPatterns": z.Slice(z.String()).Optional(),
			}),
		).Optional(),
	})
}

// Implement config.ServiceConfig interface
func (c *EmailConfig) Defaults() map[string]any {
	return map[string]any{
		// IMAP receiving settings
		"ReceiveEnabled": true,
		"IMAPPort":       993,
		"IMAPMailbox":    "INBOX",
		"PollInterval":   300, // 5 minutes in seconds
	}
}
