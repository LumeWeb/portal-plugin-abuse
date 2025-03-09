package email

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/mnako/letters"
	"github.com/stretchr/testify/assert"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// MockProviderTemplate is a mock implementation of ProviderTemplate
type MockProviderTemplate struct {
	MatchFunc              func(email *letters.Email) bool
	ProcessFunc            func(ctx context.Context, email *letters.Email) error
	ExtractSubjectURLsFunc func(email *letters.Email) []string
	ExtractReporterFunc    func(email *letters.Email) (string, string, error)
	ExtractCategoryFunc    func(email *letters.Email) models.CaseType
}

func (m *MockProviderTemplate) Match(email *letters.Email) bool {
	return m.MatchFunc(email)
}

func (m *MockProviderTemplate) Process(ctx context.Context, email *letters.Email) error {
	return m.ProcessFunc(ctx, email)
}

func (m *MockProviderTemplate) ExtractSubjectURLs(email *letters.Email) []string {
	return m.ExtractSubjectURLsFunc(email)
}

func (m *MockProviderTemplate) ExtractReporter(email *letters.Email) (string, string, error) {
	return m.ExtractReporterFunc(email)
}

func (m *MockProviderTemplate) ExtractCategory(email *letters.Email) models.CaseType {
	return m.ExtractCategoryFunc(email)
}

func TestNewProviderTemplateRegistry(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	registry := NewProviderTemplateRegistry(testCtx)

	// Verify that registry is properly initialized
	assert.NotNil(t, registry, "Registry should not be nil")
	assert.NotNil(t, registry.templates, "Templates map should be initialized")
	assert.NotNil(t, registry.detectors, "Detectors map should be initialized")
	assert.NotNil(t, registry.priorities, "Priorities map should be initialized")
	assert.Empty(t, registry.templates, "Templates map should be empty")
	assert.Empty(t, registry.detectors, "Detectors map should be empty")
	assert.Empty(t, registry.priorities, "Priorities map should be empty")
}

func TestRegisterProvider(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	registry := NewProviderTemplateRegistry(testCtx)

	// Create mock template
	mockTemplate := &MockProviderTemplate{
		MatchFunc: func(email *letters.Email) bool {
			return true
		},
		ProcessFunc: func(ctx context.Context, email *letters.Email) error {
			return nil
		},
		ExtractSubjectURLsFunc: func(email *letters.Email) []string {
			return []string{"https://example.com"}
		},
		ExtractReporterFunc: func(email *letters.Email) (string, string, error) {
			return "test@example.com", "Test User", nil
		},
		ExtractCategoryFunc: func(email *letters.Email) models.CaseType {
			return models.CaseTypeSpam
		},
	}

	// Create detector function
	detector := func(email *letters.Email) bool {
		return email.Headers.Subject == "Test Subject"
	}

	// Register the provider
	registry.RegisterProvider("test-provider", mockTemplate, detector, 10)

	// Verify registration
	assert.Len(t, registry.templates, 1, "Registry should have one template")
	assert.Len(t, registry.detectors, 1, "Registry should have one detector")
	assert.Len(t, registry.priorities, 1, "Registry should have one priority entry")

	// Verify the registered template
	template, ok := registry.templates["test-provider"]
	assert.True(t, ok, "Template should be registered with the correct ID")
	assert.Equal(t, mockTemplate, template, "Template should match the registered template")

	// Verify the registered detector
	registeredDetector, ok := registry.detectors["test-provider"]
	assert.True(t, ok, "Detector should be registered with the correct ID")

	// Test the detector
	testEmail := &letters.Email{}
	testEmail.Headers.Subject = "Test Subject"
	assert.True(t, registeredDetector(testEmail), "Detector should match the test email")

	// Verify the priority
	priority, ok := registry.priorities["test-provider"]
	assert.True(t, ok, "Priority should be registered with the correct ID")
	assert.Equal(t, 10, priority, "Priority should match the registered priority")
}

func TestUnregisterProvider(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	registry := NewProviderTemplateRegistry(testCtx)

	// Create and register a mock template
	mockTemplate := &MockProviderTemplate{
		MatchFunc: func(email *letters.Email) bool {
			return true
		},
	}

	detector := func(email *letters.Email) bool {
		return true
	}

	registry.RegisterProvider("test-provider", mockTemplate, detector, 10)
	assert.Len(t, registry.templates, 1, "Registry should have one template")

	// Unregister the provider
	registry.UnregisterProvider("test-provider")

	// Verify unregistration
	assert.Empty(t, registry.templates, "Templates map should be empty after unregistration")
	assert.Empty(t, registry.detectors, "Detectors map should be empty after unregistration")
	assert.Empty(t, registry.priorities, "Priorities map should be empty after unregistration")

	// Verify the template is no longer accessible
	_, ok := registry.templates["test-provider"]
	assert.False(t, ok, "Template should not be accessible after unregistration")
}

func TestDetectProvider(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	registry := NewProviderTemplateRegistry(testCtx)

	// Create test emails
	gmailEmail := &letters.Email{}
	gmailEmail.Headers.Subject = "Gmail Abuse Report"

	outlookEmail := &letters.Email{}
	outlookEmail.Headers.Subject = "Outlook Abuse Report"

	unknownEmail := &letters.Email{}
	unknownEmail.Headers.Subject = "Regular Email"

	// Create and register mock templates with detectors
	gmailDetector := func(email *letters.Email) bool {
		return strings.Contains(email.Headers.Subject, "Gmail")
	}

	outlookDetector := func(email *letters.Email) bool {
		return strings.Contains(email.Headers.Subject, "Outlook")
	}

	mockGmailTemplate := &MockProviderTemplate{}
	mockOutlookTemplate := &MockProviderTemplate{}

	// Register providers with different priorities (lower number = higher priority)
	registry.RegisterProvider("gmail", mockGmailTemplate, gmailDetector, 10)
	registry.RegisterProvider("outlook", mockOutlookTemplate, outlookDetector, 20)

	// Test detection with Gmail email
	provider, found := registry.DetectProvider(gmailEmail)
	assert.True(t, found, "Should detect the Gmail provider")
	assert.Equal(t, "gmail", provider, "Should detect 'gmail' as the provider")

	// Test detection with Outlook email
	provider, found = registry.DetectProvider(outlookEmail)
	assert.True(t, found, "Should detect the Outlook provider")
	assert.Equal(t, "outlook", provider, "Should detect 'outlook' as the provider")

	// Test detection with unknown email
	provider, found = registry.DetectProvider(unknownEmail)
	assert.False(t, found, "Should not detect any provider")
	assert.Equal(t, "", provider, "Provider ID should be empty when no provider is detected")
}

func TestDetectProviderPriority(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	registry := NewProviderTemplateRegistry(testCtx)

	// Create test email that matches multiple providers
	multiMatchEmail := &letters.Email{}
	multiMatchEmail.Headers.Subject = "Abuse Report from Both Providers"

	// Create detectors that both match the same email
	detector1 := func(email *letters.Email) bool {
		return strings.Contains(email.Headers.Subject, "Abuse")
	}

	detector2 := func(email *letters.Email) bool {
		return strings.Contains(email.Headers.Subject, "Report")
	}

	mockTemplate1 := &MockProviderTemplate{}
	mockTemplate2 := &MockProviderTemplate{}

	// Register providers with different priorities
	// Provider1 has higher priority (lower number)
	registry.RegisterProvider("provider1", mockTemplate1, detector1, 10)
	registry.RegisterProvider("provider2", mockTemplate2, detector2, 20)

	// Test detection - should select the higher priority provider
	provider, found := registry.DetectProvider(multiMatchEmail)
	assert.True(t, found, "Should detect a provider")
	assert.Equal(t, "provider1", provider, "Should select the higher priority provider (provider1)")

	// Now, reverse the priorities
	registry = NewProviderTemplateRegistry(testCtx)
	registry.RegisterProvider("provider1", mockTemplate1, detector1, 30) // Lower priority
	registry.RegisterProvider("provider2", mockTemplate2, detector2, 5)  // Higher priority

	// Test detection again
	provider, found = registry.DetectProvider(multiMatchEmail)
	assert.True(t, found, "Should detect a provider")
	assert.Equal(t, "provider2", provider, "Should select the higher priority provider (provider2)")
}

func TestGetTemplate(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	registry := NewProviderTemplateRegistry(testCtx)

	// Create and register a mock template
	mockTemplate := &MockProviderTemplate{}
	detector := func(email *letters.Email) bool {
		return true
	}

	registry.RegisterProvider("test-provider", mockTemplate, detector, 10)

	// Test getting a registered template
	template, ok := registry.GetTemplate("test-provider")
	assert.True(t, ok, "Should find the registered template")
	assert.Equal(t, mockTemplate, template, "Template should match the registered template")

	// Test getting a non-registered template
	template, ok = registry.GetTemplate("non-existent")
	assert.False(t, ok, "Should not find a non-registered template")
	assert.Nil(t, template, "Template should be nil for non-registered provider")
}

func TestGetAllProviders(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	registry := NewProviderTemplateRegistry(testCtx)

	// Initially, no providers should be registered
	providers := registry.GetAllProviders()
	assert.Empty(t, providers, "Initially, no providers should be registered")

	// Register multiple providers
	mockTemplate := &MockProviderTemplate{}
	detector := func(email *letters.Email) bool {
		return true
	}

	registry.RegisterProvider("provider1", mockTemplate, detector, 10)
	registry.RegisterProvider("provider2", mockTemplate, detector, 20)
	registry.RegisterProvider("provider3", mockTemplate, detector, 30)

	// Get all providers
	providers = registry.GetAllProviders()

	// Verify the result
	assert.Len(t, providers, 3, "Should return 3 registered providers")
	assert.Contains(t, providers, "provider1", "Should contain provider1")
	assert.Contains(t, providers, "provider2", "Should contain provider2")
	assert.Contains(t, providers, "provider3", "Should contain provider3")
}

func TestConcurrentAccess(t *testing.T) {
	// Setup test context
	testCtx := coreTesting.NewTestContext(t)
	defer testCtx.Teardown()

	registry := NewProviderTemplateRegistry(testCtx)

	// Create a mock template and detector
	mockTemplate := &MockProviderTemplate{
		MatchFunc: func(email *letters.Email) bool {
			return true
		},
	}

	detector := func(email *letters.Email) bool {
		return true
	}

	// Use a wait group to coordinate goroutines
	var wg sync.WaitGroup

	// Number of concurrent operations
	const numOps = 100

	// Run registration in multiple goroutines
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			providerID := fmt.Sprintf("provider-%d", id)
			registry.RegisterProvider(providerID, mockTemplate, detector, id)
		}(i)
	}

	// Run getter operations concurrently
	for i := 0; i < numOps; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// This will access the registry concurrently with registration
			registry.GetAllProviders()
		}()
	}

	// Wait for all operations to complete
	wg.Wait()

	// Verify the registry has the correct number of providers
	providers := registry.GetAllProviders()
	assert.Len(t, providers, numOps, "Registry should have all providers registered")
}
