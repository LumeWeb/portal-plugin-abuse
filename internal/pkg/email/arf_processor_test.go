package email

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"

	emailmocks "go.lumeweb.com/portal-plugin-abuse/internal/pkg/email/mocks"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// setupTestContext creates a minimal test context with just the ContentExtractor mock
func setupTestContext(t *testing.T) (testCtx coreTesting.TestContext, mockContentExtractor *emailmocks.MockContentExtractor) {
	// Create a test context using core testing
	testCtx = coreTesting.NewTestContext(t)

	// Create a mock ContentExtractor - this is the only dependency we need to mock
	mockContentExtractor = emailmocks.NewMockContentExtractor(t)

	return testCtx, mockContentExtractor
}

// TestARFProcessor_IsARF tests RFC 5965 format detection
func TestARFProcessor_IsARF(t *testing.T) {
	// Setup test context and mocks
	testCtx, mockContentExtractor := setupTestContext(t)

	// Create ARF processor
	processor := NewARFProcessor(testCtx, mockContentExtractor)

	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{
			name:     "Valid ARF format (simple)",
			email:    GenerateFromRFCExample(true),
			expected: true,
		},
		{
			name:     "Valid ARF format (full)",
			email:    GenerateFromRFCExample(false),
			expected: true,
		},
		{
			name:     "Invalid content type - not multipart/report",
			email:    NewARFSampleGenerator().GenerateInvalid("wrong-content-type"),
			expected: false,
		},
		{
			name:     "Invalid report type - not feedback-report",
			email:    NewARFSampleGenerator().GenerateInvalid("wrong-report-type"),
			expected: false,
		},
		{
			name:     "Plain email - not multipart at all",
			email:    "Subject: Test\r\nFrom: test@example.com\r\n\r\nThis is not a multipart message",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.email)
			isARF, _ := processor.IsARF(reader)
			assert.Equal(t, tt.expected, isARF)
		})
	}
}

// TestARFProcessor_ParseARF tests RFC 5965 ARF parsing
func TestARFProcessor_ParseARF(t *testing.T) {
	// Setup test context and mocks
	testCtx, mockContentExtractor := setupTestContext(t)

	// Create ARF processor
	processor := NewARFProcessor(testCtx, mockContentExtractor)

	// Test parsing the simple example from the RFC
	t.Run("Simple RFC Example", func(t *testing.T) {
		reader := strings.NewReader(GenerateFromRFCExample(true))
		report, err := processor.ParseARF(reader)

		require.NoError(t, err)
		require.NotNil(t, report)

		// Verify required fields
		assert.Equal(t, "abuse", report.FeedbackType)
		assert.Equal(t, "SomeGenerator/1.0", report.UserAgent)

		// Verify the original message parts were parsed
		assert.Equal(t, "somespammer@example.net", report.OriginalFrom)
		assert.Equal(t, "Earn money", report.OriginalSubject)

		// Verify the machine readable part
		assert.Contains(t, report.MachineReadable, "Feedback-Type")
		assert.Contains(t, report.MachineReadable, "User-Agent")
		assert.Contains(t, report.MachineReadable, "Version")
	})

	// Test parsing the full example from the RFC
	t.Run("Full RFC Example", func(t *testing.T) {
		reader := strings.NewReader(GenerateFromRFCExample(false))
		report, err := processor.ParseARF(reader)

		require.NoError(t, err)
		require.NotNil(t, report)

		// Verify required fields
		assert.Equal(t, "abuse", report.FeedbackType)
		assert.Equal(t, "SomeGenerator/1.0", report.UserAgent)

		// Verify optional fields
		assert.Equal(t, "192.0.2.1", report.SourceIP)
		assert.Equal(t, "Thu, 8 Mar 2005 14:00:00 EDT", report.ArrivalDate)

		// Verify the original message parts were parsed
		assert.Equal(t, "somespammer@example.net", report.OriginalFrom)
		assert.Equal(t, "Earn money", report.OriginalSubject)

		// Verify the machine readable part has optional fields
		assert.Contains(t, report.MachineReadable, "Original-Mail-From")
		assert.Contains(t, report.MachineReadable, "Original-Rcpt-To")
		assert.Contains(t, report.MachineReadable, "Arrival-Date")
		assert.Contains(t, report.MachineReadable, "Source-IP")
		assert.Contains(t, report.MachineReadable, "Authentication-Results")
		assert.Contains(t, report.MachineReadable, "Reported-Domain")
		assert.Contains(t, report.MachineReadable, "Reported-Uri")
	})

	// Test handling of missing parts
	t.Run("Missing Third Part", func(t *testing.T) {
		reader := strings.NewReader(NewARFSampleGenerator().GenerateInvalid("missing-original-message"))
		report, err := processor.ParseARF(reader)

		// Should not error when third part is missing - RFC says it MUST be included but parser should be robust
		require.NoError(t, err)
		require.NotNil(t, report)

		// Required fields should still be parsed
		assert.Equal(t, "abuse", report.FeedbackType)
		assert.Equal(t, "Test-Generator/1.0", report.UserAgent)

		// Original message parts should be empty
		assert.Empty(t, report.OriginalFrom)
		assert.Empty(t, report.OriginalSubject)
		assert.Empty(t, report.OriginalRecipient)
		assert.Empty(t, report.OriginalMessageHeaders)
	})

	// Test handling of malformed original message
	t.Run("Malformed Original Message", func(t *testing.T) {
		reader := strings.NewReader(NewARFSampleGenerator().GenerateInvalid("malformed-original-message"))
		report, err := processor.ParseARF(reader)

		// Should not error with malformed third part - parser should be robust
		require.NoError(t, err)
		require.NotNil(t, report)

		// Required fields should still be parsed
		assert.Equal(t, "abuse", report.FeedbackType)
		assert.Equal(t, "Test-Generator/1.0", report.UserAgent)

		// Original message parts should be empty
		assert.Empty(t, report.OriginalFrom)
		assert.Empty(t, report.OriginalSubject)
		assert.Empty(t, report.OriginalRecipient)
		assert.Empty(t, report.OriginalMessageHeaders)
	})

	// Test handling of completely invalid email
	t.Run("Not An Email At All", func(t *testing.T) {
		reader := strings.NewReader("This is not a valid email at all")
		report, err := processor.ParseARF(reader)

		// Should error on completely invalid input
		assert.Error(t, err)
		assert.Nil(t, report)
	})
}

// TestARFProcessor_ParseMachineReadable tests the RFC 5965 machine-readable part parsing
func TestARFProcessor_ParseMachineReadable(t *testing.T) {
	// Setup test context and mocks
	testCtx, mockContentExtractor := setupTestContext(t)

	// Create ARF processor
	processor := NewARFProcessor(testCtx, mockContentExtractor)

	tests := []struct {
		name               string
		content            string
		expectFields       map[string]string
		expectReportFields map[string]string
	}{
		{
			name: "Required fields only",
			content: `Feedback-Type: abuse
User-Agent: Test/1.0
Version: 1`,
			expectFields: map[string]string{
				"Feedback-Type": "abuse",
				"User-Agent":    "Test/1.0",
				"Version":       "1",
			},
			expectReportFields: map[string]string{
				"FeedbackType": "abuse",
				"UserAgent":    "Test/1.0",
			},
		},
		{
			name: "All optional single fields",
			content: `Feedback-Type: fraud
User-Agent: Test/1.0
Version: 1
Source-IP: 192.168.1.1
Arrival-Date: Mon, 01 Jan 2023 00:00:00 +0000
Original-Envelope-Id: 12345abcde
Original-Mail-From: sender@example.com
Reporting-MTA: dns; mail.example.com
Incidents: 5`,
			expectFields: map[string]string{
				"Feedback-Type":        "fraud",
				"User-Agent":           "Test/1.0",
				"Version":              "1",
				"Source-IP":            "192.168.1.1",
				"Arrival-Date":         "Mon, 01 Jan 2023 00:00:00 +0000",
				"Original-Envelope-Id": "12345abcde",
				"Original-Mail-From":   "sender@example.com",
				"Reporting-MTA":        "dns; mail.example.com",
				"Incidents":            "5",
			},
			expectReportFields: map[string]string{
				"FeedbackType": "fraud",
				"UserAgent":    "Test/1.0",
				"SourceIP":     "192.168.1.1",
				"ArrivalDate":  "Mon, 01 Jan 2023 00:00:00 +0000",
			},
		},
		{
			name: "Multiple occurrence fields",
			content: `Feedback-Type: abuse
User-Agent: Test/1.0
Version: 1
Authentication-Results: server1.example.com; dkim=pass
Authentication-Results: server2.example.com; spf=pass
Original-Rcpt-To: user1@example.com
Original-Rcpt-To: user2@example.com
Reported-Domain: example.net
Reported-Domain: example.org
Reported-Uri: http://example.net/spam1
Reported-Uri: http://example.net/spam2`,
			expectFields: map[string]string{
				"Feedback-Type":          "abuse",
				"User-Agent":             "Test/1.0",
				"Version":                "1",
				"Authentication-Results": "server2.example.com; spf=pass",
				"Original-Rcpt-To":       "user2@example.com",
				"Reported-Domain":        "example.org",
				"Reported-Uri":           "http://example.net/spam2",
			},
			expectReportFields: map[string]string{
				"FeedbackType": "abuse",
				"UserAgent":    "Test/1.0",
			},
		},
		{
			name: "Historic field",
			content: `Feedback-Type: abuse
User-Agent: Test/1.0
Version: 1
Received-Date: Mon, 01 Jan 2023 00:00:00 +0000`,
			expectFields: map[string]string{
				"Feedback-Type": "abuse",
				"User-Agent":    "Test/1.0",
				"Version":       "1",
				"Received-Date": "Mon, 01 Jan 2023 00:00:00 +0000",
			},
			expectReportFields: map[string]string{
				"FeedbackType": "abuse",
				"UserAgent":    "Test/1.0",
				"ArrivalDate":  "Mon, 01 Jan 2023 00:00:00 +0000", // Should be copied to ArrivalDate
			},
		},
		{
			name: "Malformed lines",
			content: `Feedback-Type: abuse
User-Agent: Test/1.0
Version: 1
This line has no colon
Empty value:
: Empty key
Space in key : value`,
			expectFields: map[string]string{
				"Feedback-Type": "abuse",
				"User-Agent":    "Test/1.0",
				"Version":       "1",
				"Empty value":   "",
				"":              "Empty key",
				"Space in key":  "value",
			},
			expectReportFields: map[string]string{
				"FeedbackType": "abuse",
				"UserAgent":    "Test/1.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &ARFReport{
				MachineReadable: make(map[string]string),
			}

			processor.parseMachineReadable(report, tt.content)

			// Check machine readable map has expected fields
			for key, expectedValue := range tt.expectFields {
				assert.Equal(t, expectedValue, report.MachineReadable[key])
			}

			// Check report object fields were set
			for fieldName, expectedValue := range tt.expectReportFields {
				var actualValue string

				// Use field name to get the actual value
				switch fieldName {
				case "FeedbackType":
					actualValue = report.FeedbackType
				case "UserAgent":
					actualValue = report.UserAgent
				case "ArrivalDate":
					actualValue = report.ArrivalDate
				case "SourceIP":
					actualValue = report.SourceIP
				case "ReporterEmail":
					actualValue = report.ReporterEmail
				}

				assert.Equal(t, expectedValue, actualValue)
			}
		})
	}
}

// TestARFDetectionAndParsing tests the email processing functionality focusing on ARF detection and parsing
// This test does not test the Process method which requires service mocks
func TestARFDetectionAndParsing(t *testing.T) {
	t.Run("Verify ARF detection and parsing", func(t *testing.T) {
		// Setup test context and mocks
		testCtx, mockContentExtractor := setupTestContext(t)

		// Create the processor
		processor := NewARFProcessor(testCtx, mockContentExtractor)

		// Test ARF detection with full RFC example
		t.Run("Detect RFC example ARF format", func(t *testing.T) {
			reader := strings.NewReader(GenerateFromRFCExample(false))
			isARF, buffer := processor.IsARF(reader)

			assert.True(t, isARF, "Email should be detected as ARF format")
			assert.NotNil(t, buffer, "Buffer should not be nil")
		})

		// Test ARF parsing with full RFC example
		t.Run("Parse RFC example ARF format", func(t *testing.T) {
			reader := strings.NewReader(GenerateFromRFCExample(false))
			isARF, buffer := processor.IsARF(reader)
			assert.True(t, isARF)

			report, err := processor.ParseARF(buffer)
			assert.NoError(t, err)
			assert.NotNil(t, report)

			// Verify important report fields
			assert.Equal(t, "abuse", report.FeedbackType)
			assert.Equal(t, "SomeGenerator/1.0", report.UserAgent)
			assert.Equal(t, "192.0.2.1", report.SourceIP)
			assert.NotEmpty(t, report.OriginalFrom)
			assert.NotEmpty(t, report.OriginalSubject)
		})

		// Test non-ARF email detection
		t.Run("Detect non-ARF email", func(t *testing.T) {
			reader := strings.NewReader("Subject: Normal email\r\nFrom: sender@example.com\r\n\r\nThis is a normal email, not ARF.")
			isARF, _ := processor.IsARF(reader)
			assert.False(t, isARF, "Email should not be detected as ARF format")
		})

		// Test ARF with missing reporter email
		t.Run("Parse ARF with no reporter email", func(t *testing.T) {
			// Create a sample with no From header
			generator := NewARFSampleGenerator()
			generator.SetOuterHeader("From", "")
			sample := generator.Generate()

			reader := strings.NewReader(sample)
			isARF, buffer := processor.IsARF(reader)
			assert.True(t, isARF)

			report, err := processor.ParseARF(buffer)
			assert.NoError(t, err)
			assert.Empty(t, report.ReporterEmail, "Reporter email should be empty")
		})
	})
}

// TestAllFeedbackTypes verifies that all feedback types generate valid ARF structure
func TestAllFeedbackTypes(t *testing.T) {
	// Generate samples for all feedback types
	samples := GenerateWithAllFeedbackTypes()

	// Setup test context and mocks
	testCtx, mockContentExtractor := setupTestContext(t)

	// Create ARF processor
	processor := NewARFProcessor(testCtx, mockContentExtractor)

	for feedbackType, sample := range samples {
		t.Run(feedbackType, func(t *testing.T) {
			// Verify ARF detection
			reader := strings.NewReader(sample)
			isARF, buffer := processor.IsARF(reader)
			assert.True(t, isARF, "Generated sample should be valid ARF format")

			// Verify parsing
			report, err := processor.ParseARF(buffer)
			assert.NoError(t, err, "Failed to parse generated ARF sample")
			assert.Equal(t, feedbackType, report.FeedbackType, "Incorrect feedback type parsed")

			// Basic sanity checks
			assert.NotEmpty(t, report.UserAgent, "Missing User-Agent")
			assert.NotEmpty(t, report.OriginalFrom, "Missing Original-From")
			assert.NotEmpty(t, report.OriginalSubject, "Missing Original-Subject")
		})
	}
}
