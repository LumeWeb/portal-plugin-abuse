package email

import (
	"net/mail"
	"testing"

	"github.com/mnako/letters"
	"github.com/stretchr/testify/assert"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestNewProviderFactory(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Get logger for content extractor
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Test factory creation
	factory := NewProviderFactory(testCtx, contentExtractor)

	// Verify factory is created correctly
	assert.NotNil(t, factory, "Factory should not be nil")
	assert.NotNil(t, factory.contentExtractor, "Content extractor should be set")
	assert.NotNil(t, factory.logger, "Logger should be set")
	assert.Equal(t, testCtx, factory.ctx, "Context should be set")
}

func TestRegisterStandardProviders(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Get logger for content extractor
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create factory and registry
	factory := NewProviderFactory(testCtx, contentExtractor)
	registry := NewProviderTemplateRegistry(testCtx)

	// Register standard providers
	factory.RegisterStandardProviders(registry)

	// Verify all standard providers are registered
	providers := registry.GetAllProviders()
	assert.Len(t, providers, 4, "Should register 4 standard providers")
	assert.Contains(t, providers, "gmail", "Gmail provider should be registered")
	assert.Contains(t, providers, "microsoft", "Microsoft provider should be registered")
	assert.Contains(t, providers, "yahoo", "Yahoo provider should be registered")
	assert.Contains(t, providers, "protonmail", "ProtonMail provider should be registered")

	// Verify the providers have correct templates
	for _, providerID := range []string{"gmail", "microsoft", "yahoo", "protonmail"} {
		template, ok := registry.GetTemplate(providerID)
		assert.True(t, ok, "Template should exist for provider %s", providerID)
		assert.NotNil(t, template, "Template should not be nil for provider %s", providerID)
	}
}

func TestGmailProviderDetection(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Get logger for content extractor
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create factory and registry
	factory := NewProviderFactory(testCtx, contentExtractor)
	registry := NewProviderTemplateRegistry(testCtx)

	// Register only Gmail provider
	factory.registerGmailProvider(registry)

	// Test cases
	testCases := []struct {
		name        string
		setupEmail  func() *letters.Email
		shouldMatch bool
	}{
		{
			name: "gmail domain match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@gmail.com"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "google domain match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@google.com"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "google header match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.ExtraHeaders = map[string][]string{
					"X-Google-Smtp-Source": {"test"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "gmail content match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Text = "Gmail has received a complaint about this content"
				return email
			},
			shouldMatch: true,
		},
		{
			name: "non-gmail email",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@example.com"},
				}
				email.Text = "Regular email content"
				return email
			},
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			email := tc.setupEmail()
			provider, matched := registry.DetectProvider(email)

			if tc.shouldMatch {
				assert.True(t, matched, "Email should match Gmail provider")
				assert.Equal(t, "gmail", provider, "Provider should be 'gmail'")
			} else {
				assert.False(t, matched, "Email should not match any provider")
				assert.Empty(t, provider, "Provider should be empty")
			}
		})
	}
}

func TestMicrosoftProviderDetection(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Get logger for content extractor
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create factory and registry
	factory := NewProviderFactory(testCtx, contentExtractor)
	registry := NewProviderTemplateRegistry(testCtx)

	// Register only Microsoft provider
	factory.registerMicrosoftProvider(registry)

	// Test cases
	testCases := []struct {
		name        string
		setupEmail  func() *letters.Email
		shouldMatch bool
	}{
		{
			name: "outlook domain match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@outlook.com"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "hotmail domain match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@hotmail.com"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "microsoft domain match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@microsoft.com"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "microsoft header match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.ExtraHeaders = map[string][]string{
					"X-Microsoft-Antispam": {"test"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "outlook content match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Text = "Outlook has received a complaint about this content"
				return email
			},
			shouldMatch: true,
		},
		{
			name: "non-microsoft email",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@example.com"},
				}
				email.Text = "Regular email content"
				return email
			},
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			email := tc.setupEmail()
			provider, matched := registry.DetectProvider(email)

			if tc.shouldMatch {
				assert.True(t, matched, "Email should match Microsoft provider")
				assert.Equal(t, "microsoft", provider, "Provider should be 'microsoft'")
			} else {
				assert.False(t, matched, "Email should not match any provider")
				assert.Empty(t, provider, "Provider should be empty")
			}
		})
	}
}

func TestYahooProviderDetection(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Get logger for content extractor
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create factory and registry
	factory := NewProviderFactory(testCtx, contentExtractor)
	registry := NewProviderTemplateRegistry(testCtx)

	// Register only Yahoo provider
	factory.registerYahooProvider(registry)

	// Test cases
	testCases := []struct {
		name        string
		setupEmail  func() *letters.Email
		shouldMatch bool
	}{
		{
			name: "yahoo domain match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@yahoo.com"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "yahoo header match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.ExtraHeaders = map[string][]string{
					"X-Yahoo-Newman-Property": {"test"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "yahoo content match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Text = "Yahoo received a complaint about this content"
				return email
			},
			shouldMatch: true,
		},
		{
			name: "non-yahoo email",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@example.com"},
				}
				email.Text = "Regular email content"
				return email
			},
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			email := tc.setupEmail()
			provider, matched := registry.DetectProvider(email)

			if tc.shouldMatch {
				assert.True(t, matched, "Email should match Yahoo provider")
				assert.Equal(t, "yahoo", provider, "Provider should be 'yahoo'")
			} else {
				assert.False(t, matched, "Email should not match any provider")
				assert.Empty(t, provider, "Provider should be empty")
			}
		})
	}
}

func TestProtonMailProviderDetection(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Get logger for content extractor
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create factory and registry
	factory := NewProviderFactory(testCtx, contentExtractor)
	registry := NewProviderTemplateRegistry(testCtx)

	// Register only ProtonMail provider
	factory.registerProtonMailProvider(registry)

	// Test cases
	testCases := []struct {
		name        string
		setupEmail  func() *letters.Email
		shouldMatch bool
	}{
		{
			name: "protonmail domain match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@protonmail.com"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "proton.me domain match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@proton.me"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "protonmail header match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.ExtraHeaders = map[string][]string{
					"X-PM-Origin": {"web"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "non-protonmail email",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@example.com"},
				}
				email.Text = "Regular email content"
				return email
			},
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			email := tc.setupEmail()
			provider, matched := registry.DetectProvider(email)

			if tc.shouldMatch {
				assert.True(t, matched, "Email should match ProtonMail provider")
				assert.Equal(t, "protonmail", provider, "Provider should be 'protonmail'")
			} else {
				assert.False(t, matched, "Email should not match any provider")
				assert.Empty(t, provider, "Provider should be empty")
			}
		})
	}
}

func TestCreateCustomProviderTemplate(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Get logger for content extractor
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create factory
	factory := NewProviderFactory(testCtx, contentExtractor)

	// Create a custom provider template
	domainPatterns := []string{`example\.com$`, `custom\.org$`}
	headerPatterns := []string{`X-Custom-Header`}
	contentPatterns := []string{`custom abuse report`}

	template, detector := factory.CreateCustomProviderTemplate(
		"custom-provider",
		domainPatterns,
		headerPatterns,
		contentPatterns,
	)

	// Verify the template and detector are created
	assert.NotNil(t, template, "Template should not be nil")
	assert.NotNil(t, detector, "Detector should not be nil")

	// Test the detector with different emails
	testCases := []struct {
		name        string
		setupEmail  func() *letters.Email
		shouldMatch bool
	}{
		{
			name: "domain pattern match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@example.com"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "another domain pattern match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@custom.org"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "header pattern match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.ExtraHeaders = map[string][]string{
					"X-Custom-Header": {"test"},
				}
				return email
			},
			shouldMatch: true,
		},
		{
			name: "content pattern match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Text = "This is a custom abuse report for testing"
				return email
			},
			shouldMatch: true,
		},
		{
			name: "non-matching email",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Test User", Address: "user@othercompany.com"},
				}
				email.Text = "Regular email content"
				return email
			},
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			email := tc.setupEmail()
			result := detector(email)
			assert.Equal(t, tc.shouldMatch, result, "Detector match result should be %v", tc.shouldMatch)
		})
	}
}

func TestExtractDomainFromAddress(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Get logger for content extractor
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create factory
	factory := NewProviderFactory(testCtx, contentExtractor)

	// Test cases
	testCases := []struct {
		name           string
		address        string
		expectedDomain string
	}{
		{
			name:           "standard email",
			address:        "user@example.com",
			expectedDomain: "example.com",
		},
		{
			name:           "email with subdomain",
			address:        "user@subdomain.example.com",
			expectedDomain: "subdomain.example.com",
		},
		{
			name:           "invalid email (no @)",
			address:        "userexample.com",
			expectedDomain: "",
		},
		{
			name:           "empty string",
			address:        "",
			expectedDomain: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			domain := factory.extractDomainFromAddress(tc.address)
			assert.Equal(t, tc.expectedDomain, domain, "Extracted domain should match expected value")
		})
	}
}
