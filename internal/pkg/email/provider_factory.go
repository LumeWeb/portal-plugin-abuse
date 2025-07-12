package email

import (
	"regexp"
	"strings"

	"github.com/mnako/letters"
	"go.lumeweb.com/portal/core"
)

// ProviderFactory creates provider templates for various email providers
type ProviderFactory struct {
	ctx              core.Context
	logger           *core.Logger
	contentExtractor *ContentExtractorDefault
}

// NewProviderFactory creates a new provider factory
func NewProviderFactory(ctx core.Context, contentExtractor *ContentExtractorDefault) *ProviderFactory {
	return &ProviderFactory{
		ctx:              ctx,
		logger:           ctx.NamedLogger("provider-factory"),
		contentExtractor: contentExtractor,
	}
}

// RegisterStandardProviders registers the standard set of provider templates
func (f *ProviderFactory) RegisterStandardProviders(registry *ProviderTemplateRegistry) {
	// Register Gmail provider
	f.registerGmailProvider(registry)

	// Register Microsoft provider
	f.registerMicrosoftProvider(registry)

	// Register Yahoo provider
	f.registerYahooProvider(registry)

	// Register Proton Mail provider
	f.registerProtonMailProvider(registry)
}

// registerGmailProvider registers the Gmail provider template
func (f *ProviderFactory) registerGmailProvider(registry *ProviderTemplateRegistry) {
	template := NewGmailTemplate(f.ctx, f.contentExtractor)

	// Custom detector for Gmail
	detector := func(email *letters.Email) bool {
		// Check From domain for Gmail
		if len(email.Headers.From) > 0 {
			domain := f.extractDomainFromAddress(email.Headers.From[0].Address)
			if strings.Contains(domain, "gmail.com") || strings.Contains(domain, "google.com") {
				return true
			}
		}

		// Check for Gmail-specific headers
		for key := range email.Headers.ExtraHeaders {
			if strings.HasPrefix(strings.ToLower(key), "x-google") {
				return true
			}
		}

		// Check for Gmail-specific phrases in content
		content := email.Text
		if content == "" {
			content = email.HTML
		}

		return strings.Contains(content, "Google received a complaint") ||
			strings.Contains(content, "Gmail has received a complaint") ||
			strings.Contains(content, "Gmail abuse report")
	}

	// Register with priority 10 (high priority)
	registry.RegisterProvider("gmail", template, detector, 10)
}

// registerMicrosoftProvider registers the Microsoft/Outlook provider template
func (f *ProviderFactory) registerMicrosoftProvider(registry *ProviderTemplateRegistry) {
	template := NewMicrosoftTemplate(f.ctx, f.contentExtractor)

	// Custom detector for Microsoft
	detector := func(email *letters.Email) bool {
		// Check From domain for Microsoft
		if len(email.Headers.From) > 0 {
			domain := f.extractDomainFromAddress(email.Headers.From[0].Address)
			if strings.Contains(domain, "microsoft.com") ||
				strings.Contains(domain, "outlook.com") ||
				strings.Contains(domain, "hotmail.com") ||
				strings.Contains(domain, "live.com") {
				return true
			}
		}

		// Check for Microsoft-specific headers
		for key := range email.Headers.ExtraHeaders {
			if strings.HasPrefix(strings.ToLower(key), "x-microsoft") {
				return true
			}
		}

		// Check for Microsoft-specific phrases in content
		content := email.Text
		if content == "" {
			content = email.HTML
		}

		return strings.Contains(content, "Microsoft received a complaint") ||
			strings.Contains(content, "Outlook has received a complaint") ||
			strings.Contains(content, "Microsoft abuse report")
	}

	// Register with priority 20
	registry.RegisterProvider("microsoft", template, detector, 20)
}

// registerYahooProvider registers the Yahoo provider template
func (f *ProviderFactory) registerYahooProvider(registry *ProviderTemplateRegistry) {
	// Create a Yahoo-specific detector
	detector := func(email *letters.Email) bool {
		// Check From domain for Yahoo
		if len(email.Headers.From) > 0 {
			domain := f.extractDomainFromAddress(email.Headers.From[0].Address)
			if strings.Contains(domain, "yahoo.com") {
				return true
			}
		}

		// Check for Yahoo-specific headers
		for key := range email.Headers.ExtraHeaders {
			if strings.HasPrefix(strings.ToLower(key), "x-yahoo") {
				return true
			}
		}

		// Check for Yahoo-specific phrases in content
		content := email.Text
		if content == "" {
			content = email.HTML
		}

		return strings.Contains(content, "Yahoo received a complaint") ||
			strings.Contains(content, "Yahoo abuse report")
	}

	// For now, use Microsoft template as a fallback for Yahoo
	// In a real implementation, you would create a specific Yahoo template
	template := NewMicrosoftTemplate(f.ctx, f.contentExtractor)

	// Register with priority 30
	registry.RegisterProvider("yahoo", template, detector, 30)
}

// registerProtonMailProvider registers the ProtonMail provider template
func (f *ProviderFactory) registerProtonMailProvider(registry *ProviderTemplateRegistry) {
	// Create a ProtonMail-specific detector
	detector := func(email *letters.Email) bool {
		// Check From domain for ProtonMail
		if len(email.Headers.From) > 0 {
			domain := f.extractDomainFromAddress(email.Headers.From[0].Address)
			if strings.Contains(domain, "protonmail.com") ||
				strings.Contains(domain, "proton.me") {
				return true
			}
		}

		// Check for ProtonMail-specific headers
		for key := range email.Headers.ExtraHeaders {
			if strings.HasPrefix(strings.ToLower(key), "x-pm-") {
				return true
			}
		}

		return false
	}

	// For now, use a generic template (Gmail) for ProtonMail
	// In a real implementation, you would create a specific ProtonMail template
	template := NewGmailTemplate(f.ctx, f.contentExtractor)

	// Register with priority 40
	registry.RegisterProvider("protonmail", template, detector, 40)
}

// CreateCustomProviderTemplate creates a provider template based on rules
func (f *ProviderFactory) CreateCustomProviderTemplate(
	providerName string,
	domainPatterns []string,
	headerPatterns []string,
	contentPatterns []string,
) (ProviderTemplate, ProviderDetector) {
	// Compile domain patterns into regexes
	domainRegexes := make([]*regexp.Regexp, 0, len(domainPatterns))
	for _, pattern := range domainPatterns {
		regex, err := regexp.Compile(pattern)
		if err == nil {
			domainRegexes = append(domainRegexes, regex)
		}
	}

	// Compile header patterns
	headerRegexes := make([]*regexp.Regexp, 0, len(headerPatterns))
	for _, pattern := range headerPatterns {
		regex, err := regexp.Compile(pattern)
		if err == nil {
			headerRegexes = append(headerRegexes, regex)
		}
	}

	// Compile content patterns
	contentRegexes := make([]*regexp.Regexp, 0, len(contentPatterns))
	for _, pattern := range contentPatterns {
		regex, err := regexp.Compile(pattern)
		if err == nil {
			contentRegexes = append(contentRegexes, regex)
		}
	}

	// Create a detector function
	detector := func(email *letters.Email) bool {
		// Check domains
		if len(email.Headers.From) > 0 {
			domain := f.extractDomainFromAddress(email.Headers.From[0].Address)
			for _, regex := range domainRegexes {
				if regex.MatchString(domain) {
					return true
				}
			}
		}

		// Check headers
		for key := range email.Headers.ExtraHeaders {
			for _, regex := range headerRegexes {
				if regex.MatchString(key) {
					return true
				}
			}
		}

		// Check content
		content := email.Text
		if content == "" {
			content = email.HTML
		}

		for _, regex := range contentRegexes {
			if regex.MatchString(content) {
				return true
			}
		}

		return false
	}

	// For now, use a "lowest common denominator" approach - the Gmail template
	// In a production environment, you would have more sophisticated template generation
	template := NewGmailTemplate(f.ctx, f.contentExtractor)

	return template, detector
}

// extractDomainFromAddress extracts the domain part from an email address
func (f *ProviderFactory) extractDomainFromAddress(address string) string {
	parts := strings.Split(address, "@")
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
}
