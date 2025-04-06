package config

import (
	"fmt"
	"go.lumeweb.com/portal/config"
)

var _ config.Defaults = (*EmailConfig)(nil)
var _ config.Validator = (*EmailConfig)(nil)

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

func (c *EmailConfig) Validate() error {
	// Validate IMAP settings when receiving is enabled
	if c.ReceiveEnabled {
		if c.IMAPHost == "" {
			return fmt.Errorf("imap_host is required when receive_enabled is true")
		}
		if c.IMAPUser == "" {
			return fmt.Errorf("imap_user is required when receive_enabled is true")
		}
		if c.IMAPPassword == "" {
			return fmt.Errorf("imap_password is required when receive_enabled is true")
		}
		if c.IMAPPort <= 0 || c.IMAPPort > 65535 {
			return fmt.Errorf("invalid imap_port: %d - must be between 1-65535", c.IMAPPort)
		}
	}

	return nil
}

// Implement config.ServiceConfig interface
func (c *EmailConfig) Defaults() map[string]any {
	return map[string]any{
		// IMAP receiving settings
		"receive_enabled": true,
		"imap_port":       993,
		"imap_mailbox":    "INBOX",
		"poll_interval":   300, // 5 minutes in seconds
	}
}
