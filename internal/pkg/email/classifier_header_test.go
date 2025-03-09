package email

import (
	"errors"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"net/mail"
	"strings"
	"testing"

	"github.com/mnako/letters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestClassifier_analyzeHeaders(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create the classifier
	classifier := NewClassifier(ctx)

	// Helper function to create test email with headers
	createEmailWithHeaders := func(from, replyTo, returnPath string, extraHeaders map[string][]string) *letters.Email {
		// Process from address
		var fromAddresses []*mail.Address
		if from != "" {
			// Handle display name in format "Display Name <email@example.com>"
			addr, err := mail.ParseAddress(from)
			if err == nil {
				fromAddresses = append(fromAddresses, addr)
			}
		}

		// Process reply-to address
		var replyToAddresses []*mail.Address
		if replyTo != "" {
			addr, err := mail.ParseAddress(replyTo)
			if err == nil {
				replyToAddresses = append(replyToAddresses, addr)
			}
		}

		// If returnPath is provided, add it to the extra headers
		if returnPath != "" {
			if extraHeaders == nil {
				extraHeaders = make(map[string][]string)
			}
			extraHeaders["Return-Path"] = []string{returnPath}
		}

		return &letters.Email{
			Headers: letters.Headers{
				From:         fromAddresses,
				ReplyTo:      replyToAddresses,
				ExtraHeaders: extraHeaders,
			},
		}
	}

	testCases := []struct {
		name                string
		email               *letters.Email
		expectedError       error // The main error we expect (if any)
		expectedNoReply     bool
		expectedMinScore    int
		expectedMaxScore    int
		expectErrorContains string // Substring expected in error message
	}{
		{
			name:                "Normal business email with matching headers",
			email:               createEmailWithHeaders("John Doe <john@company.com>", "John Doe <john@company.com>", "<john@company.com>", nil),
			expectedError:       nil,
			expectedNoReply:     false,
			expectedMinScore:    0,
			expectedMaxScore:    0,
			expectErrorContains: "",
		},
		{
			name:                "Missing From header",
			email:               createEmailWithHeaders("", "", "", nil),
			expectedError:       ErrMissingFromHeader,
			expectedNoReply:     false,
			expectedMinScore:    5,
			expectedMaxScore:    5,
			expectErrorContains: "",
		},
		{
			name:                "Mismatched From and Reply-To domains",
			email:               createEmailWithHeaders("John Doe <john@company.com>", "John Doe <john@different.com>", "", nil),
			expectedError:       ErrMismatchedDomains,
			expectedNoReply:     false,
			expectedMinScore:    3,
			expectedMaxScore:    3,
			expectErrorContains: "company.com",
		},
		{
			name:                "Mismatched From and Return-Path domains",
			email:               createEmailWithHeaders("John Doe <john@company.com>", "", "<bounce@mail-service.com>", nil),
			expectedError:       ErrReturnPathDomainMismatch,
			expectedNoReply:     false,
			expectedMinScore:    3,
			expectedMaxScore:    3,
			expectErrorContains: "company.com",
		},
		{
			name:                "Same domain but different addresses in From and Return-Path",
			email:               createEmailWithHeaders("John Doe <john@company.com>", "", "<marketing@company.com>", nil),
			expectedError:       ErrMismatchedReturnPath,
			expectedNoReply:     false,
			expectedMinScore:    1,
			expectedMaxScore:    1,
			expectErrorContains: "",
		},
		{
			name: "Multiple From addresses",
			email: &letters.Email{
				Headers: letters.Headers{
					From: []*mail.Address{
						{Name: "John Doe", Address: "john@company.com"},
						{Name: "Jane Smith", Address: "jane@company.com"},
					},
				},
			},
			expectedError:       ErrMultipleFromAddresses,
			expectedNoReply:     false,
			expectedMinScore:    2,
			expectedMaxScore:    2,
			expectErrorContains: "",
		},
		{
			name:                "Suspicious sender domain",
			email:               createEmailWithHeaders("John Doe <john@mailinator.com>", "", "", nil),
			expectedError:       ErrSuspiciousSenderDomain,
			expectedNoReply:     false,
			expectedMinScore:    1,
			expectedMaxScore:    1,
			expectErrorContains: "mailinator.com",
		},
		{
			name:                "High risk sender domain",
			email:               createEmailWithHeaders("Anonymous <user@mail.ru>", "", "", nil),
			expectedError:       ErrSuspiciousSenderDomain,
			expectedNoReply:     false,
			expectedMinScore:    3,
			expectedMaxScore:    3,
			expectErrorContains: "mail.ru",
		},
		{
			name:                "No-reply address",
			email:               createEmailWithHeaders("System <no-reply@company.com>", "", "", nil),
			expectedError:       nil,
			expectedNoReply:     true,
			expectedMinScore:    0,
			expectedMaxScore:    0,
			expectErrorContains: "",
		},
		{
			name:                "Suspicious display name with brand term",
			email:               createEmailWithHeaders("Amazon Security <random@phisher.com>", "", "", nil),
			expectedError:       ErrSuspiciousDisplayName,
			expectedNoReply:     false,
			expectedMinScore:    3,
			expectedMaxScore:    10, // Allow for enhanced detection to add more score
			expectErrorContains: "random@phisher.com",
		},
		{
			name:                "Malformed Return-Path",
			email:               createEmailWithHeaders("John Doe <john@company.com>", "", "malformed-not-an-email", nil),
			expectedError:       ErrMalformedReturnPath,
			expectedNoReply:     false,
			expectedMinScore:    2,
			expectedMaxScore:    2,
			expectErrorContains: "",
		},
		{
			name:                "Multiple header issues combined",
			email:               createEmailWithHeaders("PayPal Service <info@phishing-site.com>", "Support <support@different-site.com>", "<bounce@third-site.com>", nil),
			expectedError:       ErrPotentialPhishing, // the highest priority error should be returned (the most suspicious),
			expectedNoReply:     false,
			expectedMinScore:    9,  // At least: 3 (domain mismatch) + 3 (return path) + 3 (brand)
			expectedMaxScore:    15, // Allow for enhanced detection to add more score
			expectErrorContains: "phishing-site.com",
		},
		{
			name:                "Legitimate no-reply with display name mismatch (should be fine)",
			email:               createEmailWithHeaders("Customer Service <noreply@company.com>", "", "", nil),
			expectedError:       nil,
			expectedNoReply:     true,
			expectedMinScore:    0,
			expectedMaxScore:    0,
			expectErrorContains: "",
		},
		{
			name:                "Email address with reasonable display name similarity",
			email:               createEmailWithHeaders("John Doe <john.doe@company.com>", "", "", nil),
			expectedError:       nil,
			expectedNoReply:     false,
			expectedMinScore:    0,
			expectedMaxScore:    0,
			expectErrorContains: "",
		},
		{
			name:                "Lookalike Domain",
			email:               createEmailWithHeaders("User <user@paypa1.com>", "", "", nil),
			expectedError:       ErrLookalikeDomain,
			expectedNoReply:     false,
			expectedMinScore:    1,
			expectedMaxScore:    5, // varies based on similarity
			expectErrorContains: "paypa1.com",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err, isNoReply, score := classifier.analyzeHeaders(tc.email)

			assert.Equal(t, tc.expectedNoReply, isNoReply, "No-reply flag should match expected")
			assert.GreaterOrEqual(t, score, tc.expectedMinScore, "Score should be at least expectedMinScore")
			assert.LessOrEqual(t, score, tc.expectedMaxScore, "Score should be at most expectedMaxScore")

			if tc.expectedError != nil {
				unwrappedErrors := unwrapJoinError(err)
				found := false
				for _, singleErr := range unwrappedErrors {
					if errors.Is(singleErr, tc.expectedError) {
						found = true
						if tc.expectErrorContains != "" {
							assert.Contains(t, singleErr.Error(), tc.expectErrorContains, "Expected error message to contain: %s", tc.expectErrorContains)
						}
						break // Found the error, move on
					}
				}
				if !found {
					t.Errorf("Expected error of type %v not found in error chain: %v", tc.expectedError, err)
				}
			} else {
				assert.NoError(t, err, "Expected no error, but got: %v", err)
			}
		})
	}
}

func TestClassifyEmail_WithHeaderAnalysis(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create the classifier
	classifier := NewClassifier(ctx)

	// Helper function to create test emails with headers and content
	createTestEmail := func(from, replyTo, returnPath, subject, content string, extraHeaders map[string][]string) *letters.Email {
		// Process from address
		var fromAddresses []*mail.Address
		if from != "" {
			addr, err := mail.ParseAddress(from)
			if err == nil {
				fromAddresses = append(fromAddresses, addr)
			}
		}

		// Process reply-to address
		var replyToAddresses []*mail.Address
		if replyTo != "" {
			addr, err := mail.ParseAddress(replyTo)
			if err == nil {
				replyToAddresses = append(replyToAddresses, addr)
			}
		}

		// If returnPath is provided, add it to the extra headers
		if returnPath != "" {
			if extraHeaders == nil {
				extraHeaders = make(map[string][]string)
			}
			extraHeaders["Return-Path"] = []string{returnPath}
		}

		return &letters.Email{
			Headers: letters.Headers{
				Subject:      subject,
				From:         fromAddresses,
				ReplyTo:      replyToAddresses,
				ExtraHeaders: extraHeaders,
			},
			Text: content,
		}
	}

	testCases := []struct {
		name                   string
		email                  *letters.Email
		expectedHeaderError    error
		expectedNoReply        bool
		expectedPriorityChange bool
		expectedNeedsReview    bool
	}{
		{
			name: "Legitimate corporate email",
			email: createTestEmail(
				"John Smith <john@company.com>",
				"",
				"<john@company.com>",
				"Company Update",
				"This is a legitimate update from our company about recent developments.",
				nil,
			),
			expectedHeaderError:    nil,
			expectedNoReply:        false,
			expectedPriorityChange: false,
			expectedNeedsReview:    true, // Default for "other" category
		},
		{
			name: "Legitimate no-reply notification",
			email: createTestEmail(
				"System Notification <no-reply@company.com>",
				"",
				"<no-reply@company.com>",
				"Your Account Update",
				"Your account has been updated with your recent preferences.",
				nil,
			),
			expectedHeaderError:    nil,
			expectedNoReply:        true,
			expectedPriorityChange: false,
			expectedNeedsReview:    true, // Default for "other" category
		},
		{
			name: "Suspicious header phishing attempt with domain mismatch",
			email: createTestEmail(
				"PayPal Security <security@phishing-example.net>",
				"Support <reply@different-service.com>",
				"<bounce@third-party.org>",
				"URGENT: Account Verification Required",
				"Click here to verify your PayPal account immediately.",
				nil,
			),
			expectedNoReply:        false,
			expectedPriorityChange: true,
			expectedNeedsReview:    true,
		},
		{
			name: "Spam content with moderate header issues",
			email: createTestEmail(
				"Marketing <info@marketing-service.com>",
				"",
				"<bounce@mail-company.com>",
				"Special Offer Inside",
				"This is an unsolicited marketing email offering discount deals for bulk purchases and newsletter subscriptions.",
				nil,
			),
			expectedHeaderError:    ErrReturnPathDomainMismatch, // Changed from ErrMismatchedReturnPath
			expectedNoReply:        false,
			expectedPriorityChange: true,
			expectedNeedsReview:    true,
		},
		{
			name: "Multiple severe header issues with spam content",
			email: createTestEmail(
				"Amazon Account <security@amaz0n-verification.com>",
				"Customer Support <reply@different-site.net>",
				"<bounce@third-domain.com>",
				"Verify Your Amazon Account Now",
				"Your account has been locked. Click here to verify your account. This is an unsolicited email.",
				nil,
			),
			expectedHeaderError:    ErrPotentialPhishing,
			expectedNoReply:        false,
			expectedPriorityChange: true, // Should increase to at least medium
			expectedNeedsReview:    true, // Should need review due to header issues
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := classifier.ClassifyEmail(tc.email)
			require.NotNil(t, result, "Classification result should not be nil")

			// Handle phishing test case specially
			if tc.name == "Suspicious header phishing attempt with domain mismatch" {
				// Should have multiple header issues
				unwrappedErrors := unwrapJoinError(result.HeaderIssues)
				require.GreaterOrEqual(t, len(unwrappedErrors), 2, "Expected at least 2 header issues")

				var foundPhishing, foundDomainMismatch, foundReturnPathIssue bool
				for _, err := range unwrappedErrors {
					switch {
					case errors.Is(err, ErrPotentialPhishing):
						foundPhishing = true
						assert.Contains(t, err.Error(), "PayPal", "Phishing error should mention brand name")
						assert.Contains(t, err.Error(), "phishing-example.net", "Phishing error should mention suspicious domain")
					case errors.Is(err, ErrMismatchedDomains):
						foundDomainMismatch = true
					case errors.Is(err, ErrReturnPathDomainMismatch):
						foundReturnPathIssue = true
					}
				}

				assert.True(t, foundPhishing, "Missing phishing detection error")
				assert.True(t, foundDomainMismatch, "Missing From/Reply-To domain mismatch error")
				assert.True(t, foundReturnPathIssue, "Missing Return-Path domain mismatch error")

				assert.GreaterOrEqual(t, result.Priority, models.CasePriorityHigh, "Should have high priority")
				return
			}

			// Existing verification for other test cases
			if tc.expectedHeaderError != nil {
				unwrappedErrors := unwrapJoinError(result.HeaderIssues)
				found := false
				for _, singleErr := range unwrappedErrors {
					if errors.Is(singleErr, tc.expectedHeaderError) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error of type %v not found in error chain: %v", tc.expectedHeaderError, result.HeaderIssues)
				}
			} else {
				assert.NoError(t, result.HeaderIssues)
			}

			assert.Equal(t, tc.expectedNoReply, result.IsNoReplyAddress, "No-reply flag should match expected")

			assert.Equal(t, tc.expectedNeedsReview, result.NeedsReview, "NeedsReview flag should match expected")
		})
	}
}

func TestClassifier_WithCustomHeaderThresholds(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create a classifier with custom header thresholds
	customOptions := []ClassifierOption{
		WithHeaderAnalysisThresholds(3, 5, 8, 7), // Lower thresholds than default
	}
	classifier := NewClassifier(ctx, customOptions...)

	// Create test email with moderate header issues
	email := &letters.Email{
		Headers: letters.Headers{
			Subject: "Test Email",
			From: []*mail.Address{
				{Name: "John Doe", Address: "john@example.com"},
			},
			ReplyTo: []*mail.Address{
				{Name: "John Doe", Address: "john@different.com"},
			},
			ExtraHeaders: map[string][]string{
				"Return-Path": {"<bounce@third-domain.com>"},
			},
		},
		Text: "This is a test email with some mild spam indicators but nothing severe.",
	}

	// Test that custom thresholds affect the classification
	result := classifier.ClassifyEmail(email)

	require.NotNil(t, result, "Classification result should not be nil")

	// With lower thresholds, we should detect header issues and they should affect classification
	assert.Error(t, result.HeaderIssues, "Header issues should be detected with custom thresholds")

	// With our custom settings, even minor header issues should trigger review
	assert.True(t, result.NeedsReview, "Should need review with custom header thresholds")
}

// Helper function for substring assertions (case insensitive)
func assertContainsSubstring(t *testing.T, fullString, substring string) bool {
	fullLower := strings.ToLower(fullString)
	subLower := strings.ToLower(substring)
	contains := strings.Contains(fullLower, subLower)
	assert.True(t, contains, "Expected '%s' to contain '%s' (case insensitive)", fullString, substring)
	return contains
}
