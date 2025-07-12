package email

import (
	"context"
	"errors"
	"go.lumeweb.com/portal/core"
	"net/mail"
	"strings"
	"testing"

	"github.com/mnako/letters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

// Test cases begin here

func TestNewTemplateProcessor(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Create dependencies
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create template processor
	processor := NewTemplateProcessor(testCtx, contentExtractor).(*DefaultTemplateProcessor)

	// Verify processor is created correctly
	assert.NotNil(t, processor, "Processor should not be nil")
	assert.NotNil(t, processor.contentExtractor, "Content extractor should be set")
	assert.NotNil(t, processor.registry, "Registry should be initialized")
}

func TestGetSetRegistry(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Create dependencies
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create template processor
	processor := NewTemplateProcessor(testCtx, contentExtractor)

	// Create a new registry
	newRegistry := NewProviderTemplateRegistry(testCtx)

	// Get the current registry
	originalRegistry := processor.GetRegistry()
	assert.NotNil(t, originalRegistry, "Original registry should not be nil")

	// Set the new registry
	processor.SetRegistry(newRegistry)

	// Verify the registry was updated
	assert.Equal(t, newRegistry, processor.GetRegistry(), "Registry should be updated")
	assert.NotEqual(t, originalRegistry, processor.GetRegistry(), "Registry should be different from original")
}

func TestRegisterTemplates(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Create dependencies
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create template processor
	processor := NewTemplateProcessor(testCtx, contentExtractor)

	// Verify standard templates are registered
	registry := processor.GetRegistry()
	providers := registry.GetAllProviders()

	assert.Contains(t, providers, "gmail", "Gmail provider should be registered")
	assert.Contains(t, providers, "microsoft", "Microsoft provider should be registered")
}

func TestRegisterProviderTemplate(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Create dependencies
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create template processor
	processor := NewTemplateProcessor(testCtx, contentExtractor)

	// Create a mock template
	mockTemplate := new(MockProviderTemplate)
	mockDetector := func(email *letters.Email) bool {
		return email.Headers.Subject == "Custom Template Test"
	}

	// Register the custom template
	processor.RegisterProviderTemplate("custom", mockTemplate, mockDetector, 100)

	// Verify the template was registered
	registry := processor.GetRegistry()
	providers := registry.GetAllProviders()
	assert.Contains(t, providers, "custom", "Custom provider should be registered")

	template, ok := registry.GetTemplate("custom")
	assert.True(t, ok, "Template should be retrievable")
	assert.Equal(t, mockTemplate, template, "Template should match the registered one")
}

func TestTemplateProcessorDetectProvider(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Create dependencies
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create template processor
	processor := NewTemplateProcessor(testCtx, contentExtractor)

	// Test cases
	testCases := []struct {
		name             string
		emailContent     string
		expectMatch      bool
		expectedProvider string
	}{
		{
			name: "gmail abuse report",
			emailContent: `From: abuse@google.com
Subject: Gmail Abuse Report
Date: Mon, 15 Jan 2024 10:00:00 -0700

Gmail has received a complaint regarding content.
`,
			expectMatch:      true,
			expectedProvider: "gmail",
		},
		{
			name: "microsoft abuse report",
			emailContent: `From: abuse@microsoft.com
Subject: Microsoft Abuse Report
Date: Mon, 15 Jan 2024 10:00:00 -0700

Microsoft received a complaint about content.
`,
			expectMatch:      true,
			expectedProvider: "microsoft",
		},
		{
			name: "unknown report format",
			emailContent: `From: user@example.com
Subject: Regular Email
Date: Mon, 15 Jan 2024 10:00:00 -0700

This is a regular email with no abuse report indicators.
`,
			expectMatch:      false,
			expectedProvider: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a reader from the test email content
			reader := strings.NewReader(tc.emailContent)

			// Detect provider
			provider, email, matched := processor.DetectProvider(reader)

			// Verify results
			assert.Equal(t, tc.expectMatch, matched, "Match result should be as expected")
			assert.Equal(t, tc.expectedProvider, provider, "Provider should match expected")
			assert.NotNil(t, email, "Email should be parsed even if no provider matches")
		})
	}
}

func TestDetectProvider_DomainExtraction(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Create dependencies
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create template processor
	processor := NewTemplateProcessor(testCtx, contentExtractor)

	// Test cases
	testCases := []struct {
		name           string
		emailContent   string
		expectedDomain string
	}{
		{
			name: "single address",
			emailContent: `From: Test User <user@example.com>
Subject: Test Email
Date: Mon, 15 Jan 2024 10:00:00 -0700

Test content.
`,
			expectedDomain: "example.com",
		},
		{
			name: "multiple addresses",
			emailContent: `From: Test User <user@example.com>, Other User <other@other.com>
Subject: Test Email
Date: Mon, 15 Jan 2024 10:00:00 -0700

Test content.
`,
			expectedDomain: "example.com",
		},
		{
			name: "invalid email format",
			emailContent: `From: Invalid <invalid-email>
Subject: Test Email
Date: Mon, 15 Jan 2024 10:00:00 -0700

Test content.
`,
			expectedDomain: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reader := strings.NewReader(tc.emailContent)
			_, email, _ := processor.DetectProvider(reader)
			
			// Verify domain extraction by checking the From header
			if email != nil && len(email.Headers.From) > 0 {
				addr := email.Headers.From[0].Address
				if tc.expectedDomain != "" {
					assert.Contains(t, addr, "@"+tc.expectedDomain, 
						"Email address should contain expected domain")
				} else {
					assert.NotContains(t, addr, "@", 
						"Invalid email should not contain domain separator")
				}
			} else {
				assert.Empty(t, tc.expectedDomain, 
					"Expected empty domain when email parsing fails")
			}
		})
	}
}

func TestTemplateProcessor_Process(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Create dependencies
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create template processor
	processor := NewTemplateProcessor(testCtx, contentExtractor)

	// Create and register a mock template using the existing mock from provider_registry_test.go
	mockTemplate := &MockProviderTemplate{
		ProcessFunc: func(ctx context.Context, email *letters.Email) error {
			return nil
		},
	}

	// Replace registry with a new one that contains our test template
	registry := NewProviderTemplateRegistry(testCtx)
	registry.RegisterProvider("test-provider", mockTemplate, func(email *letters.Email) bool {
		return true
	}, 10)
	processor.SetRegistry(registry)

	// Test valid template processing
	t.Run("successful_processing", func(t *testing.T) {
		// Create a reader with test email content
		emailContent := `From: test@example.com
Subject: Test Email
Date: Mon, 15 Jan 2024 10:00:00 -0700

Test content.
`
		reader := strings.NewReader(emailContent)

		// Process with our test provider
		err := processor.Process(context.Background(), reader, "test-provider")
		assert.NoError(t, err, "Should not return an error for successful processing")
	})

	// Test error when template is not available
	t.Run("no_template_available", func(t *testing.T) {
		// Create a reader with test email content
		emailContent := `From: test@example.com
Subject: Test Email
Date: Mon, 15 Jan 2024 10:00:00 -0700

Test content.
`
		reader := strings.NewReader(emailContent)

		// Process with a provider that doesn't exist
		err := processor.Process(context.Background(), reader, "unknown-provider")
		assert.Error(t, err, "Should return an error")
		assert.Contains(t, err.Error(), "no template available for provider: unknown-provider",
			"Error message should indicate template not available")
	})

	// Test error during template processing
	t.Run("template_processing_error", func(t *testing.T) {
		// Create a new mockTemplate with an error response
		errorMockTemplate := &MockProviderTemplate{
			ProcessFunc: func(ctx context.Context, email *letters.Email) error {
				return errors.New("template processing error")
			},
		}

		// Register the error-producing template
		errorRegistry := NewProviderTemplateRegistry(testCtx)
		errorRegistry.RegisterProvider("error-provider", errorMockTemplate, func(email *letters.Email) bool {
			return true
		}, 10)
		processor.SetRegistry(errorRegistry)

		// Create a reader with test email content
		emailContent := `From: test@example.com
Subject: Test Email
Date: Mon, 15 Jan 2024 10:00:00 -0700

Test content.
`
		reader := strings.NewReader(emailContent)

		// Process with our error-producing provider
		err := processor.Process(context.Background(), reader, "error-provider")
		assert.Error(t, err, "Should return an error")
		assert.Contains(t, err.Error(), "template processing error",
			"Error message should match the template error")
	})
}

func TestGmailTemplateMatch(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Create dependencies
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create Gmail template
	gmailTemplate := NewGmailTemplate(testCtx, contentExtractor)

	// Test cases
	testCases := []struct {
		name        string
		setupEmail  func() *letters.Email
		shouldMatch bool
	}{
		{
			name: "gmail subject match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.Subject = "Gmail Abuse Report"
				return email
			},
			shouldMatch: true,
		},
		{
			name: "google subject match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.Subject = "Google Abuse Report"
				return email
			},
			shouldMatch: true,
		},
		{
			name: "text content match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Text = "Google received a complaint regarding this content"
				return email
			},
			shouldMatch: true,
		},
		{
			name: "html content match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.HTML = "<p>Gmail has received a complaint about this content</p>"
				return email
			},
			shouldMatch: true,
		},
		{
			name: "no match",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.Subject = "Regular Email"
				email.Text = "This is just a regular email"
				return email
			},
			shouldMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			email := tc.setupEmail()
			result := gmailTemplate.Match(email)
			assert.Equal(t, tc.shouldMatch, result, "Match result should be %v", tc.shouldMatch)
		})
	}
}

func TestGmailTemplateExtractReporter(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Create dependencies
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create Gmail template
	gmailTemplate := NewGmailTemplate(testCtx, contentExtractor)

	// Test cases
	testCases := []struct {
		name          string
		setupEmail    func() *letters.Email
		expectedEmail string
		expectedName  string
		expectError   bool
	}{
		{
			name: "with from header",
			setupEmail: func() *letters.Email {
				email := &letters.Email{}
				email.Headers.From = []*mail.Address{
					{Name: "Google Abuse Team", Address: "abuse-team@google.com"},
				}
				return email
			},
			expectedEmail: "abuse-team@google.com",
			expectedName:  "Google Abuse Team",
			expectError:   false,
		},
		{
			name: "without from header",
			setupEmail: func() *letters.Email {
				return &letters.Email{}
			},
			expectedEmail: "google-abuse@gmail.com", // Default value
			expectedName:  "Google Abuse Team",      // Default value
			expectError:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			email := tc.setupEmail()
			reporterEmail, reporterName, err := gmailTemplate.ExtractReporter(email)

			if tc.expectError {
				assert.Error(t, err, "Should return an error")
			} else {
				assert.NoError(t, err, "Should not return an error")
				assert.Equal(t, tc.expectedEmail, reporterEmail, "Reporter email should match expected")
				assert.Equal(t, tc.expectedName, reporterName, "Reporter name should match expected")
			}
		})
	}
}

func TestGmailTemplateExtractCategory(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Create dependencies
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create Gmail template
	gmailTemplate := NewGmailTemplate(testCtx, contentExtractor)

	// Test cases
	testCases := []struct {
		name             string
		content          string
		expectedCategory models.CaseType
	}{
		{
			name:             "spam content",
			content:          "This email contains spam and unwanted email content",
			expectedCategory: models.CaseTypeSpam,
		},
		{
			name:             "phishing content",
			content:          "This email contains phishing attempts and fraud",
			expectedCategory: models.CaseTypeOther,
		},
		{
			name:             "copyright content",
			content:          "This is a DMCA notice about copyright infringement",
			expectedCategory: models.CaseTypeCopyrightViolation,
		},
		{
			name:             "harassment content",
			content:          "This email contains harassment and bullying content",
			expectedCategory: models.CaseTypeHarassment,
		},
		{
			name:             "unrecognized category",
			content:          "This email contains general issues",
			expectedCategory: models.CaseTypeOther,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test email with the content
			email := &letters.Email{
				Text: tc.content,
			}

			// Extract category
			category := gmailTemplate.ExtractCategory(email)

			// Verify result
			assert.Equal(t, tc.expectedCategory, category, "Category should match expected")
		})
	}
}

func TestGmailTemplateExtractSubjectURLs(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Create dependencies
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create Gmail template
	gmailTemplate := NewGmailTemplate(testCtx, contentExtractor)

	// Test cases
	testCases := []struct {
		name         string
		content      string
		expectedURLs []string
	}{
		{
			name: "with reported url section",
			content: `
Email content here.

Reported URL: https://malicious-site.example.com/page
More content here.
`,
			expectedURLs: []string{"https://malicious-site.example.com/page"},
		},
		{
			name: "with reported content section",
			content: `
Email content here.

Reported Content: https://content-issues.example.org/post
More content here.
`,
			expectedURLs: []string{"https://content-issues.example.org/post"},
		},
		{
			name: "with multiple urls",
			content: `
Email content with https://example.com

Reported URL: https://malicious-site.example.com/page
Also check https://another-issue.example.net
`,
			expectedURLs: []string{
				"https://example.com",
				"https://malicious-site.example.com/page",
				"https://another-issue.example.net",
			},
		},
		{
			name: "no urls",
			content: `
Email content with no URLs.
Just plain text here.
`,
			expectedURLs: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test email with the content
			email := &letters.Email{
				Text: tc.content,
			}

			// Extract URLs
			urls := gmailTemplate.ExtractSubjectURLs(email)

			// Verify that all expected URLs are present
			// Using ElementsMatch because order might vary
			if len(tc.expectedURLs) > 0 {
				assert.True(t, len(urls) > 0, "Should extract at least one URL")

				// Check that each expected URL is in the results
				for _, expectedURL := range tc.expectedURLs {
					found := false
					for _, extractedURL := range urls {
						if extractedURL == expectedURL {
							found = true
							break
						}
					}
					assert.True(t, found, "Expected URL %s should be extracted", expectedURL)
				}
			} else {
				assert.Empty(t, urls, "Should not extract any URLs")
			}
		})
	}
}

func TestGmailTemplateProcess(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	// Create dependencies
	logger := testCtx.Logger()
	contentExtractor := NewContentExtractor(logger)

	// Create mock services
	mockReporterService := mocks.NewMockReporterService(t)
	mockCaseService := mocks.NewMockCaseService(t)
	mockSubjectService := mocks.NewMockSubjectService(t)
	mockCommunicationService := mocks.NewMockCommunicationService(t)

	// Register services with the test context
	testCtx.RegisterService(typesSvc.REPORTER_SERVICE, mockReporterService)
	testCtx.RegisterService(typesSvc.CASE_SERVICE, mockCaseService)
	testCtx.RegisterService(typesSvc.SUBJECT_SERVICE, mockSubjectService)
	testCtx.RegisterService(typesSvc.COMMUNICATION_SERVICE, mockCommunicationService)

	// Create Gmail template
	gmailTemplate := NewGmailTemplate(testCtx, contentExtractor)

	// Test case for successful processing
	t.Run("successful_process", func(t *testing.T) {
		// Reset mocks
		mockReporterService.ExpectedCalls = nil
		mockCaseService.ExpectedCalls = nil
		mockSubjectService.ExpectedCalls = nil
		mockCommunicationService.ExpectedCalls = nil

		// Setup test email
		email := &letters.Email{
			Headers: letters.Headers{
				From: []*mail.Address{
					{Name: "Google Abuse", Address: "abuse@google.com"},
				},
				MessageID: "test-message-id",
			},
			Text: "This is a spam report about https://spam-site.example.com",
		}

		// Setup mock expectations
		reporter := &models.Reporter{
			Email: "abuse@google.com",
			Name:  "Google Abuse",
		}
		// Set the ID field separately
		reporter.ID = 123

		caseModel := &models.Case{
			ReporterID:      reporter.ID,
			Type:            models.CaseTypeSpam,
			Status:          models.CaseStatusNew,
			Source:          models.ReportSourceEmail,
			Description:     "This is a spam report about https://spam-site.example.com",
			ReferenceNumber: "CASE-12345",
		}
		// Set the ID field separately
		caseModel.ID = 456

		hash, err := core.ParseStorageHash(exampleCID)
		if err != nil {
			t.Fatal(err)
		}

		subject := &models.Subject{
			Type:       models.SubjectTypeURL,
			Identifier: hash.Multihash(),
			SourceURL:  "https://spam-site.example.com",
		}
		// Set the ID field separately
		subject.ID = 789

		comm := &models.Communication{
			CaseID:    caseModel.ID,
			SenderID:  reporter.ID,
			Type:      models.CommunicationTypeEmail,
			Direction: models.CommunicationDirectionExternal,
			Content:   email.Text,
			ThreadID:  "test-message-id",
		}
		// Set the ID field separately
		comm.ID = 101

		// Reporter service mock
		mockReporterService.On("GetByEmail", "abuse@google.com").
			Return(reporter, nil)

		// Case service mock
		mockCaseService.On("Create", mock.MatchedBy(func(c *models.Case) bool {
			return c.ReporterID == reporter.ID &&
				c.Type == models.CaseTypeSpam &&
				c.Status == models.CaseStatusNew &&
				c.Source == models.ReportSourceEmail
		})).Return(caseModel, nil)

		// Subject service mock
		mockSubjectService.On("FindOrCreateByURL", "https://spam-site.example.com", models.SubjectTypeURL).
			Return(subject, nil)

		// Communication service mock
		mockCommunicationService.On("Create", mock.MatchedBy(func(c *models.Communication) bool {
			return c.CaseID == caseModel.ID &&
				c.SenderID == reporter.ID &&
				c.Type == models.CommunicationTypeEmail &&
				c.Direction == models.CommunicationDirectionExternal
		})).Return(comm, nil)

		// Test the process method
		err = gmailTemplate.Process(context.Background(), email)
		assert.NoError(t, err, "Process should not return an error")

		// Verify all expectations were met
		mockReporterService.AssertExpectations(t)
		mockCaseService.AssertExpectations(t)
		mockSubjectService.AssertExpectations(t)
		mockCommunicationService.AssertExpectations(t)
	})

	// Test case for error in reporter service
	t.Run("reporter_service_error", func(t *testing.T) {
		// Reset mocks
		mockReporterService.ExpectedCalls = nil
		mockCaseService.ExpectedCalls = nil
		mockSubjectService.ExpectedCalls = nil
		mockCommunicationService.ExpectedCalls = nil

		// Setup test email
		email := &letters.Email{
			Headers: letters.Headers{
				From: []*mail.Address{
					{Name: "Google Abuse", Address: "abuse@google.com"},
				},
			},
			Text: "This is a spam report",
		}

		// Setup reporter service to return an error
		mockReporterService.On("GetByEmail", "abuse@google.com").
			Return(nil, errors.New("reporter service error"))

		// Test the process method
		err := gmailTemplate.Process(context.Background(), email)
		assert.Error(t, err, "Process should return an error")
		assert.Contains(t, err.Error(), "failed to get reporter by email", "Error message should indicate reporter service issue")

		// Verify expectations
		mockReporterService.AssertExpectations(t)
	})

	// Test case for creating a new reporter
	t.Run("create_new_reporter", func(t *testing.T) {
		// Reset mocks
		mockReporterService.ExpectedCalls = nil
		mockCaseService.ExpectedCalls = nil
		mockSubjectService.ExpectedCalls = nil
		mockCommunicationService.ExpectedCalls = nil

		// Setup test email
		email := &letters.Email{
			Headers: letters.Headers{
				From: []*mail.Address{
					{Name: "New Reporter", Address: "new@example.com"},
				},
				MessageID: "test-message-id",
			},
			Text: "This is a report",
		}

		// Setup reporter service to return not found
		mockReporterService.On("GetByEmail", "new@example.com").
			Return(nil, gorm.ErrRecordNotFound)

		// Setup reporter creation
		newReporter := &models.Reporter{
			Model: gorm.Model{ID: 123},
			Email: "new@example.com",
			Name:  "New Reporter",
		}
		mockReporterService.On("Create", mock.MatchedBy(func(r *models.Reporter) bool {
			return r.Email == "new@example.com" && r.Name == "New Reporter"
		})).Return(newReporter, nil)

		// Mock case service
		caseModel := &models.Case{
			Model:           gorm.Model{ID: 456},
			ReporterID:      newReporter.ID,
			Status:          models.CaseStatusNew,
			Source:          models.ReportSourceEmail,
			ReferenceNumber: "CASE-12345",
		}
		mockCaseService.On("Create", mock.Anything).Return(caseModel, nil)

		// Mock communication service
		response := &models.Communication{}
		response.ID = 101
		mockCommunicationService.On("Create", mock.Anything).Return(response, nil)

		// Test the process method
		err := gmailTemplate.Process(context.Background(), email)
		assert.NoError(t, err, "Process should not return an error")

		// Verify expectations
		mockReporterService.AssertExpectations(t)
		mockCaseService.AssertExpectations(t)
		mockCommunicationService.AssertExpectations(t)
	})
}
