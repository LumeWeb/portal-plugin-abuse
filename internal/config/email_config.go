package config

// ProviderTemplateConfig defines configuration for a provider email template
type ProviderTemplateConfig struct {
	Name            string   `config:"name"`
	Enabled         bool     `config:"enabled" default:"true"`
	Priority        int      `config:"priority" default:"100"` // Higher values = lower priority
	DomainPatterns  []string `config:"domain_patterns"`        // Regular expressions to match email domains
	HeaderPatterns  []string `config:"header_patterns"`        // Regular expressions to match header names
	ContentPatterns []string `config:"content_patterns"`       // Regular expressions to match content
}

// EmailConfig holds email service configuration
type EmailConfig struct {
	// SMTP settings for sending emails
	SMTPHost     string `config:"smtp_host" default:"smtp.example.com"`
	SMTPPort     int    `config:"smtp_port" default:"587"`
	SMTPUser     string `config:"smtp_user" default:""`
	SMTPPassword string `config:"smtp_password" default:""`

	// Email identity settings
	FromEmail    string `config:"email_from_address" default:"noreply@example.com"`
	FromName     string `config:"email_from_name" default:"Abuse Management System"`
	ReplyToEmail string `config:"email_reply_to" default:"abuse@example.com"`

	// General email settings
	Enabled bool   `config:"email_enabled" default:"true"`
	SiteURL string `config:"site_url" default:"https://example.com"`

	// IMAP settings for receiving emails
	ReceiveEnabled   bool     `config:"receive_enabled" default:"false"`
	IMAPHost         string   `config:"imap_host" default:"imap.example.com"`
	IMAPPort         int      `config:"imap_port" default:"993"`
	IMAPUser         string   `config:"imap_user" default:""`
	IMAPPassword     string   `config:"imap_password" default:""`
	IMAPMailbox      string   `config:"imap_mailbox" default:"INBOX"`
	PollInterval     int      `config:"poll_interval" default:"300"` // In seconds, defaults to 5 minutes
	ReceiveAddresses []string `config:"receive_addresses" default:"abuse@example.com,report-abuse@example.com"`

	// Legacy SMTP server settings (deprecated)
	ReceiveBindAddress string `config:"receive_bind_address" default:"0.0.0.0"`
	ReceivePort        int    `config:"receive_port" default:"25"`

	// Provider template settings
	StandardProviders []string                 `config:"standard_providers" default:"gmail,microsoft,yahoo,protonmail"`
	CustomProviders   []ProviderTemplateConfig `config:"custom_providers"`
}

// Implement config.ServiceConfig interface
func (c *EmailConfig) Defaults() map[string]any {
	return map[string]any{
		// SMTP sending settings
		"smtp_host":     "smtp.example.com",
		"smtp_port":     587,
		"smtp_user":     "",
		"smtp_password": "",

		// Email identity settings
		"email_from_address": "noreply@example.com",
		"email_from_name":    "Abuse Management System",
		"email_reply_to":     "abuse@example.com",

		// General email settings
		"email_enabled": true,
		"site_url":      "https://example.com",

		// IMAP receiving settings
		"receive_enabled":   false,
		"imap_host":         "imap.example.com",
		"imap_port":         993,
		"imap_user":         "",
		"imap_password":     "",
		"imap_mailbox":      "INBOX",
		"poll_interval":     300, // 5 minutes in seconds
		"receive_addresses": []string{"abuse@example.com", "report-abuse@example.com"},

		// Legacy SMTP receiving settings (deprecated)
		"receive_bind_address": "0.0.0.0",
		"receive_port":         25,

		// Provider template settings
		"standard_providers": []string{"gmail", "microsoft", "yahoo", "protonmail"},
		"custom_providers":   []ProviderTemplateConfig{},
	}
}
