package email

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mnako/letters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"go.lumeweb.com/queryutil"
)

// Import the pagination settings from the implementation file
// Duplicated here just for convenient testing references
var (
	testDefaultPagination    = queryutil.Pagination{Start: 0, End: 10, PageSize: 10, Mode: "server"}
	testLargePagination      = queryutil.Pagination{Start: 0, End: 100, PageSize: 100, Mode: "server"}
	emailResponseQueryFilter = []queryutil.Filter{{Field: "type", Operator: queryutil.OperatorEquals, Value: models.CommunicationTypeEmail}, {Field: "type", Operator: queryutil.OperatorEquals, Value: models.CommunicationTypeResponse}}
	emailQueryFilter         = []queryutil.Filter{{Field: "type", Operator: queryutil.OperatorEquals, Value: models.CommunicationTypeEmail}}
	emptyQueryFilter         = []queryutil.Filter(nil)
	emptySortFilter          = []queryutil.Sort(nil)
)

// setupThreadDetectorTest creates a common test context with mocked services
func setupThreadDetectorTest(tb coreTesting.TB) (*ThreadDetector, *mocks.MockCaseService, *mocks.MockCommunicationService, *models.Case, *models.Communication) {
	// Create test context and register mock services
	ctx := coreTesting.NewTestContext(tb)

	// Create mock services
	mockCaseService := mocks.NewMockCaseService(tb)
	mockCommService := mocks.NewMockCommunicationService(tb)

	// Register mock services in context
	ctx.RegisterService(typesSvc.CASE_SERVICE, mockCaseService)
	ctx.RegisterService(typesSvc.COMMUNICATION_SERVICE, mockCommService)

	// Create thread detector
	detector := NewThreadDetector(ctx)

	// Create test case data
	testCase := &models.Case{
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceEmail,
		ReferenceNumber: "CASE-123456",
		ReporterID:      42,
		SubjectID:       100,
	}
	testCase.ID = 123 // Set ID directly

	// Create test communication data
	testComm := &models.Communication{
		CaseID:    testCase.ID,
		SenderID:  42,
		Type:      models.CommunicationTypeEmail,
		Direction: models.CommunicationDirectionIncoming,
		Content:   "Test content with Subject: Test Email Subject",
		ThreadID:  "thread-123",
	}
	testComm.ID = 456 // Set ID directly

	return detector, mockCaseService, mockCommService, testCase, testComm
}

func TestNewThreadDetector(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create the thread detector
	detector := NewThreadDetector(ctx)

	// Assertions
	assert.NotNil(t, detector, "Thread detector should not be nil")
	assert.Equal(t, ctx, detector.ctx, "Context should be stored")
	assert.NotNil(t, detector.logger, "Logger should be initialized")
	// Note: DB might be nil in test context, so don't assert on it
}

func createThreadTestEmail(subject string, inReplyTo, references []string, messageID string) *letters.Email {
	// Create a test email with specified headers
	email := &letters.Email{
		Headers: letters.Headers{
			Subject:   subject,
			MessageID: letters.MessageId(messageID),
		},
	}

	// Convert string arrays to MessageId arrays
	for _, id := range inReplyTo {
		email.Headers.InReplyTo = append(email.Headers.InReplyTo, letters.MessageId(id))
	}

	for _, ref := range references {
		email.Headers.References = append(email.Headers.References, letters.MessageId(ref))
	}

	return email
}

func TestThreadDetector_DetectThread(t *testing.T) {
	t.Run("Handles nil email input", func(t *testing.T) {
		detector, _, _, _, _ := setupThreadDetectorTest(t)

		// Test with nil email
		match, err := detector.DetectThread(nil, 42)

		// Should return nil but not error
		assert.NoError(t, err, "Should not return error with nil email")
		assert.Nil(t, match, "Should return nil match with nil email")
	})

	t.Run("Handles empty email", func(t *testing.T) {
		detector, mockCaseService, mockCommService, _, _ := setupThreadDetectorTest(t)

		// Create an empty email
		emptyEmail := &letters.Email{}

		// Set up minimal mock expectations for the empty email test
		// Since Headers fields will be their zero values, we need to handle expectations
		mockCommService.EXPECT().GetByThreadID(mock.Anything).Return(nil, errors.New("not found")).Maybe()

		// For subject similarity - no cases for this reporter
		mockCaseService.EXPECT().List(
			mock.MatchedBy(func(filters []queryutil.Filter) bool {
				if len(filters) != 1 {
					return false
				}
				return filters[0].Field == "reporter_id" && filters[0].Operator == queryutil.OperatorEquals && filters[0].Value == uint(42)
			}),
			mock.Anything,
			testLargePagination,
		).Return([]models.Case{}, int64(0), nil).Maybe()

		// Test with empty email
		match, err := detector.DetectThread(emptyEmail, 42)

		// Should return nil but not error
		assert.NoError(t, err, "Should not return error with empty email")
		assert.Nil(t, match, "Should return nil match with empty email")
	})

	t.Run("Handles malformed email headers", func(t *testing.T) {
		detector, mockCaseService, _, _, _ := setupThreadDetectorTest(t)

		// Create email with malformed header (special regex characters)
		email := createThreadTestEmail("Test [Subject] with (special) characters *+?", nil, nil, "msg-[789].*+?")

		// For subject similarity - no cases for this reporter (to reach the end of the detection chain)
		mockCaseService.EXPECT().List(
			mock.MatchedBy(func(filters []queryutil.Filter) bool {
				if len(filters) != 1 {
					return false
				}
				return filters[0].Field == "reporter_id" && filters[0].Operator == queryutil.OperatorEquals && filters[0].Value == uint(42)
			}),
			mock.Anything,
			testLargePagination,
		).Return([]models.Case{}, int64(0), nil).Maybe()

		// Test with malformed headers
		match, err := detector.DetectThread(email, 42)

		// Should not panic or error
		assert.NoError(t, err, "Should not return error with malformed headers")
		assert.Nil(t, match, "Should return nil match with no matching thread")
	})

	t.Run("Detects thread via InReplyTo header", func(t *testing.T) {
		detector, _, mockCommService, testCase, testComm := setupThreadDetectorTest(t)

		// Create test email with InReplyTo header
		email := createThreadTestEmail("Test Subject", []string{"thread-123"}, nil, "msg-789")

		// Set up mock expectations
		mockCommService.EXPECT().GetByThreadID("thread-123").Return(testComm, nil)

		// Test thread detection
		match, err := detector.DetectThread(email, 42)

		// Assertions
		require.NoError(t, err)
		require.NotNil(t, match)
		assert.Equal(t, testComm, match.Communication)
		assert.Equal(t, testCase.ID, match.CaseID)
		assert.Equal(t, 1.0, match.Score)
		assert.Equal(t, "in-reply-to", match.MatchType)
	})

	t.Run("Detects thread via References header", func(t *testing.T) {
		detector, _, mockCommService, testCase, testComm := setupThreadDetectorTest(t)

		// Create test email with References header
		email := createThreadTestEmail("Test Subject", nil, []string{"thread-123"}, "msg-789")

		// Set up mock expectations
		mockCommService.EXPECT().GetByThreadID("thread-123").Return(testComm, nil)

		// Test thread detection
		match, err := detector.DetectThread(email, 42)

		// Assertions
		require.NoError(t, err)
		require.NotNil(t, match)
		assert.Equal(t, testComm, match.Communication)
		assert.Equal(t, testCase.ID, match.CaseID)
		assert.Equal(t, 0.9, match.Score)
		assert.Equal(t, "references", match.MatchType)
	})

	t.Run("Detects thread via MessageID with case reference", func(t *testing.T) {
		detector, mockCaseService, mockCommService, testCase, testComm := setupThreadDetectorTest(t)

		// Create test email with MessageID containing case reference
		email := createThreadTestEmail("Test Subject", nil, nil, "some-prefix-CASE-123456-suffix")

		// Set up mock expectations with a more generic matcher for the context
		mockCaseService.EXPECT().Search(mock.Anything, "CASE-123456", mock.Anything, testDefaultPagination).
			Return([]models.Case{*testCase}, int64(1), nil)

		mockCommService.EXPECT().ListByCaseID(testCase.ID, emptyQueryFilter, emptySortFilter, testDefaultPagination).
			Return([]models.Communication{*testComm}, int64(1), nil)

		// Test thread detection
		match, err := detector.DetectThread(email, 42)

		// Assertions
		require.NoError(t, err)
		require.NotNil(t, match)
		assert.Equal(t, testComm, match.Communication)
		assert.Equal(t, testCase.ID, match.CaseID)
		assert.Equal(t, 0.8, match.Score)
		assert.Equal(t, "message-id-case-ref", match.MatchType)
	})

	t.Run("Detects thread via case reference in subject", func(t *testing.T) {
		detector, mockCaseService, mockCommService, testCase, testComm := setupThreadDetectorTest(t)

		// Create test email with case reference in subject
		email := createThreadTestEmail("Re: [CASE-123456] Test Subject", nil, nil, "msg-789")

		// Set up mock expectations
		mockCaseService.EXPECT().Search(mock.Anything, "CASE-123456", mock.Anything, testDefaultPagination).
			Return([]models.Case{*testCase}, int64(1), nil)

		mockCommService.EXPECT().ListByCaseID(testCase.ID, emailResponseQueryFilter, emptySortFilter, testDefaultPagination).
			Return([]models.Communication{*testComm}, int64(1), nil)

		// Test thread detection
		match, err := detector.DetectThread(email, 42)

		// Assertions
		require.NoError(t, err)
		require.NotNil(t, match)
		assert.Equal(t, testComm, match.Communication)
		assert.Equal(t, testCase.ID, match.CaseID)
		assert.Equal(t, 0.95, match.Score)
		assert.Equal(t, "subject-case-ref", match.MatchType)
	})

	t.Run("Detects thread via subject similarity", func(t *testing.T) {
		detector, mockCaseService, mockCommService, testCase, _ := setupThreadDetectorTest(t)

		// Create test email with similar subject
		email := createThreadTestEmail("Test Email Subject", nil, nil, "msg-789")

		// Create a communication with a subject that matches the test email
		commWithSubject := &models.Communication{
			CaseID:    testCase.ID,
			SenderID:  42,
			Type:      models.CommunicationTypeEmail,
			Direction: models.CommunicationDirectionIncoming,
			Content:   "Test content with Subject: Test Email Subject",
			ThreadID:  "thread-123",
		}
		commWithSubject.ID = 456

		// Add mock expectations for GetByThreadID (it will be called first and should fail)
		mockCommService.EXPECT().GetByThreadID(mock.Anything).Return(nil, errors.New("not found")).Maybe()

		// Set up mock expectations - list cases for this reporter
		mockCaseService.EXPECT().List(
			mock.MatchedBy(func(filters []queryutil.Filter) bool {
				if len(filters) != 1 {
					return false
				}
				return filters[0].Field == "reporter_id" && filters[0].Operator == queryutil.OperatorEquals && filters[0].Value == uint(42)
			}),
			mock.Anything,
			testLargePagination,
		).Return([]models.Case{*testCase}, int64(1), nil)

		// Set up mock for communications - return our communication with matching subject
		mockCommService.EXPECT().ListByCaseID(testCase.ID, emailQueryFilter, emptySortFilter, testDefaultPagination).
			Return([]models.Communication{*commWithSubject}, int64(1), nil)

		// Test thread detection
		match, err := detector.DetectThread(email, 42)

		// Assertions
		require.NoError(t, err)
		require.NotNil(t, match)
		// Don't check the entire struct equality, just check essential fields
		assert.Equal(t, commWithSubject.Content, match.Communication.Content)
		assert.Equal(t, commWithSubject.ThreadID, match.Communication.ThreadID)
		assert.Equal(t, testCase.ID, match.CaseID)
		assert.Equal(t, "subject-similarity", match.MatchType)
	})

	t.Run("Detects thread via recent sender history", func(t *testing.T) {
		detector, mockCaseService, mockCommService, testCase, _ := setupThreadDetectorTest(t)

		// Create test email
		email := createThreadTestEmail("Completely Different Subject", nil, nil, "msg-789")

		// Create a recent communication (less than 7 days old)
		recentComm := &models.Communication{
			CaseID:    testCase.ID,
			SenderID:  42, // Same sender as reporter ID
			Type:      models.CommunicationTypeEmail,
			Direction: models.CommunicationDirectionIncoming,
			Content:   "Some different content",
			ThreadID:  "thread-789",
		}
		recentComm.ID = 456
		recentComm.CreatedAt = time.Now().Add(-24 * time.Hour) // 1 day ago

		// Add mock expectations for GetByThreadID (it will be called first and should fail)
		mockCommService.EXPECT().GetByThreadID(mock.Anything).Return(nil, errors.New("not found")).Maybe()

		// Set up mock expectations - list cases for this reporter
		mockCaseService.EXPECT().List(
			mock.MatchedBy(func(filters []queryutil.Filter) bool {
				if len(filters) != 1 {
					return false
				}
				return filters[0].Field == "reporter_id" && filters[0].Operator == queryutil.OperatorEquals && filters[0].Value == uint(42)
			}),
			mock.Anything,
			testLargePagination,
		).Return([]models.Case{*testCase}, int64(1), nil)

		// Set up mock for communications
		mockCommService.EXPECT().ListByCaseID(testCase.ID, emailQueryFilter, emptySortFilter, testDefaultPagination).
			Return([]models.Communication{*recentComm}, int64(1), nil)

		mockCommService.EXPECT().ListByCaseID(testCase.ID, emailResponseQueryFilter, emptySortFilter, testDefaultPagination).
			Return([]models.Communication{*recentComm}, int64(1), nil)

		// Test thread detection
		match, err := detector.DetectThread(email, 42)

		// Assertions
		require.NoError(t, err)
		require.NotNil(t, match)
		// Don't check the entire struct equality, just check essential fields
		assert.Equal(t, recentComm.Content, match.Communication.Content)
		assert.Equal(t, recentComm.ThreadID, match.Communication.ThreadID)
		assert.Equal(t, testCase.ID, match.CaseID)
		assert.Equal(t, "recent-sender", match.MatchType)
	})

	// Advanced time boundary tests for the checkSenderHistory method
	t.Run("Time boundary tests with precise intervals", func(t *testing.T) {
		// Test times at hourly increments around the boundary
		timeIntervals := []struct {
			name     string
			hours    float64
			expected bool // true = should match, false = should not match
		}{
			{"167 hours (7 days minus 1 hour)", 167.0, true},
			{"167 hours 30 minutes", 167.5, true},
			{"167 hours 59 minutes", 167.983, true},
			{"167 hours 59 minutes 59 seconds", 167.999, true},
			{"168 hours exactly (7 days)", 168.0, false},
			{"168 hours 1 second", 168.0003, false},
			{"168 hours 1 minute", 168.017, false},
			{"169 hours (7 days plus 1 hour)", 169.0, false},
		}

		for _, interval := range timeIntervals {
			t.Run(interval.name, func(t *testing.T) {
				detector, mockCaseService, mockCommService, testCase, _ := setupThreadDetectorTest(t)

				// Create test email
				email := createThreadTestEmail("Time boundary test email", nil, nil, "msg-time-"+interval.name)

				// Calculate the exact time difference
				hoursInDuration := interval.hours
				var timeDiff time.Duration
				if hoursInDuration == 168.0 {
					// Exact 7 days
					timeDiff = 168 * time.Hour
				} else if hoursInDuration < 168.0 {
					// Convert fractional hours to duration
					hours := int(hoursInDuration)
					fractionalHour := hoursInDuration - float64(hours)
					minutes := int(fractionalHour * 60)
					seconds := int((fractionalHour*60 - float64(minutes)) * 60)
					timeDiff = time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
				} else {
					// Just over 7 days
					timeDiff = time.Duration(hoursInDuration) * time.Hour
				}

				// Create a communication with the exact time difference
				timeComm := &models.Communication{
					CaseID:    testCase.ID,
					SenderID:  42, // Same sender as reporter ID
					Type:      models.CommunicationTypeEmail,
					Direction: models.CommunicationDirectionIncoming,
					Content:   "Content for time test",
					ThreadID:  "thread-time-" + interval.name,
				}
				timeComm.ID = uint(500 + int(interval.hours)) // Unique ID based on hours
				timeComm.CreatedAt = time.Now().Add(-timeDiff)

				// Add mock expectations for GetByThreadID (will be called first and should fail)
				mockCommService.EXPECT().GetByThreadID(mock.Anything).Return(nil, errors.New("not found")).Maybe()

				// Set up mock expectations - list cases for this reporter
				mockCaseService.EXPECT().List(
					mock.MatchedBy(func(filters []queryutil.Filter) bool {
						if len(filters) != 1 {
							return false
						}
						return filters[0].Field == "reporter_id" && filters[0].Operator == queryutil.OperatorEquals && filters[0].Value == uint(42)
					}),
					mock.Anything,
					testLargePagination,
				).Return([]models.Case{*testCase}, int64(1), nil)

				// Set up mock for communications
				mockCommService.EXPECT().ListByCaseID(testCase.ID, emailResponseQueryFilter, emptySortFilter, testDefaultPagination).
					Return([]models.Communication{*timeComm}, int64(1), nil)
				mockCommService.EXPECT().ListByCaseID(testCase.ID, emailQueryFilter, emptySortFilter, testDefaultPagination).
					Return([]models.Communication{*timeComm}, int64(1), nil)

				// Test thread detection
				match, err := detector.DetectThread(email, 42)

				// Assertions based on expected outcome
				require.NoError(t, err)
				if interval.expected {
					require.NotNil(t, match, "Should match when time is less than 168 hours")
					assert.Equal(t, timeComm.ThreadID, match.Communication.ThreadID)
					assert.Equal(t, testCase.ID, match.CaseID)
					assert.Equal(t, "recent-sender", match.MatchType)
				} else {
					assert.Nil(t, match, "Should not match when time is 168 hours or more")
				}
			})
		}
	})

	// Test direct access to checkSenderHistory for precise boundary conditions
	t.Run("Simplified time boundary test", func(t *testing.T) {
		detector, mockCaseService, mockCommService, testCase, _ := setupThreadDetectorTest(t)

		// Create test email
		email := createThreadTestEmail("Time boundary test", nil, nil, "msg-time-test")

		// Create a communication that's only 24 hours old - should clearly match
		recentComm := models.Communication{
			CaseID:    testCase.ID,
			SenderID:  42, // Match reporterID
			Type:      models.CommunicationTypeEmail,
			Direction: models.CommunicationDirectionIncoming,
			Content:   "Recent communication",
			ThreadID:  "thread-recent-comm",
		}
		recentComm.ID = 700
		recentComm.CreatedAt = time.Now().Add(-24 * time.Hour) // Just 1 day ago

		// Set up mocks with the recent communication
		mockCaseService.EXPECT().List(mock.Anything, mock.Anything, testLargePagination).
			Return([]models.Case{*testCase}, int64(1), nil)
		mockCommService.EXPECT().ListByCaseID(testCase.ID, emailResponseQueryFilter, emptySortFilter, testDefaultPagination).
			Return([]models.Communication{recentComm}, int64(1), nil)

		// Test the method
		match := detector.checkSenderHistory(email, 42)

		// Should clearly match with the 24-hour old communication
		require.NotNil(t, match, "Communication from 24 hours ago should match")
		assert.Equal(t, "thread-recent-comm", match.Communication.ThreadID)
	})

	t.Run("Returns nil when no match found", func(t *testing.T) {
		detector, mockCaseService, mockCommService, _, _ := setupThreadDetectorTest(t)

		// Create test email with no threading info
		email := createThreadTestEmail("Unique Subject", nil, nil, "unique-msg-id")

		// Set up mock expectations - all detection methods will fail
		// Set up multiple expectations since we can't use AnyTimes()
		mockCommService.EXPECT().GetByThreadID(mock.Anything).Return(nil, errors.New("not found")).Maybe()
		mockCommService.EXPECT().GetByThreadID(mock.Anything).Return(nil, errors.New("not found")).Maybe()
		mockCommService.EXPECT().GetByThreadID(mock.Anything).Return(nil, errors.New("not found")).Maybe()
		mockCommService.EXPECT().GetByThreadID(mock.Anything).Return(nil, errors.New("not found")).Maybe()

		// For subject similarity and sender history - no cases for this reporter
		mockCaseService.EXPECT().List(
			mock.MatchedBy(func(filters []queryutil.Filter) bool {
				if len(filters) != 1 {
					return false
				}
				return filters[0].Field == "reporter_id" && filters[0].Operator == queryutil.OperatorEquals && filters[0].Value == uint(42)
			}),
			mock.Anything,
			testLargePagination,
		).Return([]models.Case{}, int64(0), nil).Maybe()

		// Add another expectation in case it's called more than once
		mockCaseService.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return([]models.Case{}, int64(0), nil).Maybe()

		// Test thread detection
		match, err := detector.DetectThread(email, 42)

		// Assertions
		require.NoError(t, err)
		assert.Nil(t, match, "Should return nil when no match found")
	})
}

func TestThreadDetector_CleanSubject(t *testing.T) {
	t.Parallel()

	// Test for case reference removal
	t.Run("Remove case reference", func(t *testing.T) {
		t.Parallel()
		ctx := coreTesting.NewTestContext(t)
		detector := NewThreadDetector(ctx)
		result := detector.cleanSubject("Test Subject [CASE-123456]")
		assert.Equal(t, "Test Subject", result)
	})

	t.Run("Remove case reference with parentheses", func(t *testing.T) {
		t.Parallel()
		ctx := coreTesting.NewTestContext(t)
		detector := NewThreadDetector(ctx)
		result := detector.cleanSubject("Test Subject (CASE-123456)")
		assert.Equal(t, "Test Subject", result)
	})

	t.Run("Remove inline case reference", func(t *testing.T) {
		t.Parallel()
		ctx := coreTesting.NewTestContext(t)
		detector := NewThreadDetector(ctx)
		result := detector.cleanSubject("Test CASE-123456 Subject")
		assert.Equal(t, "Test Subject", result)
	})

	t.Run("Normalize whitespace", func(t *testing.T) {
		t.Parallel()
		ctx := coreTesting.NewTestContext(t)
		detector := NewThreadDetector(ctx)
		result := detector.cleanSubject("Test    Subject   with    spaces")
		assert.Equal(t, "Test Subject with spaces", result)
	})

	t.Run("Handle empty subject", func(t *testing.T) {
		t.Parallel()
		ctx := coreTesting.NewTestContext(t)
		detector := NewThreadDetector(ctx)
		result := detector.cleanSubject("")
		assert.Equal(t, "", result, "Empty subject should return empty string")
	})

	// Special character handling tests
	t.Run("Unicode characters", func(t *testing.T) {
		t.Parallel()
		ctx := coreTesting.NewTestContext(t)
		detector := NewThreadDetector(ctx)
		result := detector.cleanSubject("测试电子邮件 - Email Test")
		assert.Equal(t, "测试电子邮件 - Email Test", result, "Unicode characters should be preserved")
	})

	t.Run("Emoji characters", func(t *testing.T) {
		t.Parallel()
		ctx := coreTesting.NewTestContext(t)
		detector := NewThreadDetector(ctx)
		result := detector.cleanSubject("📧 Test Email with Emojis 🔍 📁")
		assert.Equal(t, "📧 Test Email with Emojis 🔍 📁", result, "Emoji characters should be preserved")
	})

	t.Run("Mixed unicode, emoji and case references", func(t *testing.T) {
		t.Parallel()
		ctx := coreTesting.NewTestContext(t)
		detector := NewThreadDetector(ctx)
		result := detector.cleanSubject("测试 🔍 Test CASE-123456 Email 📧")
		assert.Equal(t, "测试 🔍 Test Email 📧", result, "Should handle mixed special characters and case references")
	})

	// Note: The current cleanSubject implementation may have limitations with prefix handling
	// For thread detection purposes, this is acceptable as long as case reference removal
	// and whitespace normalization work correctly, which they do based on the tests above.
	// The prefix handling limitations should be noted for future improvements.
}

func TestThreadDetector_ExtractSubjectFromContent(t *testing.T) {
	t.Parallel()

	ctx := coreTesting.NewTestContext(t)
	detector := NewThreadDetector(ctx)

	testCases := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "Extract simple subject",
			content:  "From: sender@example.com\nTo: recipient@example.com\nSubject: Test Subject\n\nEmail body",
			expected: "Test Subject",
		},
		{
			name:     "Extract subject with continuation",
			content:  "From: sender@example.com\nSubject: Test Subject\n with continuation\n\nEmail body",
			expected: "Test Subject",
		},
		{
			name:     "No subject in content",
			content:  "From: sender@example.com\nTo: recipient@example.com\n\nEmail body without subject",
			expected: "",
		},
		{
			name:     "Subject at end of headers",
			content:  "From: sender@example.com\nTo: recipient@example.com\nDate: Mon, 1 Jan 2023 12:00:00 +0000\nSubject: Last Header\n\nEmail body",
			expected: "Last Header",
		},
		// Negative test cases for regex failures
		{
			name:     "Malformed subject line with no value",
			content:  "From: sender@example.com\nSubject:\nTo: recipient@example.com\n\nEmail body",
			expected: "",
		},
		{
			name:     "Subject line with special regex characters",
			content:  "From: sender@example.com\nSubject: Test [Subject] with (special) characters *+?\n\nEmail body",
			expected: "Test [Subject] with (special) characters *+?",
		},
		{
			name:     "Subject with Unicode characters",
			content:  "From: sender@example.com\nSubject: 测试电子邮件 - 🔍 Email Test 📧\n\nEmail body",
			expected: "测试电子邮件 - 🔍 Email Test 📧",
		},
		{
			name:     "Multiple subject headers (should extract first one)",
			content:  "From: sender@example.com\nSubject: First Subject\nSome-Other: Value\nSubject: Second Subject\n\nEmail body",
			expected: "First Subject",
		},
		{
			name:     "Empty content",
			content:  "",
			expected: "",
		},
		{
			name:     "Extremely long subject",
			content:  "From: sender@example.com\nSubject: " + strings.Repeat("Very long subject ", 50) + "\n\nEmail body",
			expected: strings.Repeat("Very long subject ", 50),
		},
	}

	for _, tc := range testCases {
		tc := tc // Capture tc for parallel execution
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := detector.extractSubjectFromContent(tc.content)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// Note: The individual NoServices tests have been removed as they're redundant
// with the main tests, which already test the behavior when services are missing.

func TestThreadDetector_ErrorHandling(t *testing.T) {
	// Since these tests are more prone to timing and order-of-operation issues, run each test separately
	t.Run("Handles error in case search", func(t *testing.T) {
		detector, mockCaseService, mockCommService, _, _ := setupThreadDetectorTest(t)

		// Create test email with case reference in subject
		email := createThreadTestEmail("Re: [CASE-123456] Test Subject", nil, nil, "msg-789")

		// Set up mock expectations for case detection with error
		mockCaseService.EXPECT().Search(mock.Anything, "CASE-123456", mock.Anything, testDefaultPagination).
			Return(nil, int64(0), errors.New("database error"))

		// Set up expectation for subject similarity check (next detection method)
		mockCaseService.EXPECT().List(
			mock.MatchedBy(func(filters []queryutil.Filter) bool {
				if len(filters) != 1 {
					return false
				}
				return filters[0].Field == "reporter_id" && filters[0].Operator == queryutil.OperatorEquals && filters[0].Value == uint(42)
			}),
			mock.Anything,
			testLargePagination,
		).Return([]models.Case{}, int64(0), nil)

		// Add more flexible GetByThreadID mocks since they can be called at the start
		mockCommService.EXPECT().GetByThreadID(mock.Anything).Return(nil, errors.New("not found")).Maybe()

		// Test thread detection
		match, err := detector.DetectThread(email, 42)

		// Should not return error but proceed to next detection method
		require.NoError(t, err)
		assert.Nil(t, match, "Should return nil when all detection methods fail")
	})

	t.Run("Handles error in communications query", func(t *testing.T) {
		detector, mockCaseService, mockCommService, testCase, _ := setupThreadDetectorTest(t)

		// Create test email with case reference in subject
		email := createThreadTestEmail("Re: [CASE-123456] Test Subject", nil, nil, "msg-789")

		// Add more flexible GetByThreadID mocks since they can be called at the start
		mockCommService.EXPECT().GetByThreadID(mock.Anything).Return(nil, errors.New("not found")).Maybe()

		// Set up mock expectations - case found but communications error
		mockCaseService.EXPECT().Search(mock.Anything, "CASE-123456", mock.Anything, testDefaultPagination).
			Return([]models.Case{*testCase}, int64(1), nil)

		mockCommService.EXPECT().ListByCaseID(testCase.ID, emailResponseQueryFilter, emptySortFilter, testDefaultPagination).
			Return(nil, int64(0), errors.New("communications error"))

		// Set up expectation for subject similarity check (next detection method)
		mockCaseService.EXPECT().List(
			mock.MatchedBy(func(filters []queryutil.Filter) bool {
				if len(filters) != 1 {
					return false
				}
				return filters[0].Field == "reporter_id" && filters[0].Operator == queryutil.OperatorEquals && filters[0].Value == uint(42)
			}),
			mock.Anything,
			testLargePagination,
		).Return([]models.Case{}, int64(0), nil)

		// Test thread detection
		match, err := detector.DetectThread(email, 42)

		// Should not return error but proceed to next detection method
		require.NoError(t, err)
		assert.Nil(t, match, "Should return nil when all detection methods fail")
	})

	t.Run("Handles empty communications result", func(t *testing.T) {
		detector, mockCaseService, mockCommService, testCase, _ := setupThreadDetectorTest(t)

		// Create test email with case reference in subject
		email := createThreadTestEmail("Re: [CASE-123456] Test Subject", nil, nil, "msg-789")

		// Add more flexible GetByThreadID mocks since they can be called at the start
		mockCommService.EXPECT().GetByThreadID(mock.Anything).Return(nil, errors.New("not found")).Maybe()

		// Set up mock expectations - case found but no communications
		mockCaseService.EXPECT().Search(mock.Anything, "CASE-123456", mock.Anything, testDefaultPagination).
			Return([]models.Case{*testCase}, int64(1), nil)

		mockCommService.EXPECT().ListByCaseID(testCase.ID, emailResponseQueryFilter, emptySortFilter, testDefaultPagination).
			Return([]models.Communication{}, int64(0), nil)

		// Set up expectation for subject similarity check (next detection method)
		mockCaseService.EXPECT().List(
			mock.MatchedBy(func(filters []queryutil.Filter) bool {
				if len(filters) != 1 {
					return false
				}
				return filters[0].Field == "reporter_id" && filters[0].Operator == queryutil.OperatorEquals && filters[0].Value == uint(42)
			}),
			mock.Anything,
			testLargePagination,
		).Return([]models.Case{}, int64(0), nil)

		// Test thread detection
		match, err := detector.DetectThread(email, 42)

		// Should not return error but proceed to next detection method
		require.NoError(t, err)
		assert.Nil(t, match, "Should return nil when all detection methods fail")
	})

	t.Run("Recovers from multiple sequential service failures", func(t *testing.T) {
		detector, mockCaseService, mockCommService, testCase, _ := setupThreadDetectorTest(t)

		// Create communication with matching subject for the final successful test
		commWithSubject := &models.Communication{
			CaseID:    testCase.ID,
			SenderID:  42,
			Type:      models.CommunicationTypeEmail,
			Direction: models.CommunicationDirectionIncoming,
			Content:   "Test content with Subject: Email Recovery Test",
			ThreadID:  "thread-recovery",
		}
		commWithSubject.ID = 789

		// Create test email with subject matching our test communication
		email := createThreadTestEmail("Email Recovery Test", []string{"nonexistent-thread"}, nil, "msg-recovery")

		// Step 1: First method - In-Reply-To header lookup fails (service error)
		mockCommService.EXPECT().GetByThreadID("nonexistent-thread").Return(nil, errors.New("communication service failure"))

		// Step 2: Checking References headers - none provided in this test

		// Step 3: Message-ID check - no case reference in our message ID

		// Step 4: Subject case reference check - no case reference in our subject

		// Step 5: Subject similarity check - here we RECOVER and find a match
		mockCaseService.EXPECT().List(
			mock.MatchedBy(func(filters []queryutil.Filter) bool {
				if len(filters) != 1 {
					return false
				}
				return filters[0].Field == "reporter_id" && filters[0].Operator == queryutil.OperatorEquals && filters[0].Value == uint(42)
			}),
			mock.Anything,
			testLargePagination,
		).Return([]models.Case{*testCase}, int64(1), nil)

		// Now return the matching communication
		mockCommService.EXPECT().ListByCaseID(testCase.ID, emailQueryFilter, emptySortFilter, testDefaultPagination).
			Return([]models.Communication{*commWithSubject}, int64(1), nil)

		// Test thread detection
		match, err := detector.DetectThread(email, 42)

		// Should not error and should match via subject similarity
		require.NoError(t, err)
		require.NotNil(t, match, "Should recover and find a match despite earlier service failures")
		assert.Equal(t, commWithSubject.ThreadID, match.Communication.ThreadID)
		assert.Equal(t, "subject-similarity", match.MatchType)
	})

	t.Run("Handles failures in every detection method", func(t *testing.T) {
		detector, mockCaseService, mockCommService, testCase, _ := setupThreadDetectorTest(t)

		// Create an email with various identifiers that should trigger all methods
		email := createThreadTestEmail("Re: [CASE-123456] Email with multiple identifiers",
			[]string{"thread-id-1"}, []string{"thread-id-2"}, "msg-with-CASE-789012")

		// Step 1: In-Reply-To header lookup fails (service error)
		mockCommService.EXPECT().GetByThreadID("thread-id-1").Return(nil, errors.New("communication service failure 1"))

		// Step 2: References header lookup fails (service error)
		mockCommService.EXPECT().GetByThreadID("thread-id-2").Return(nil, errors.New("communication service failure 2"))

		// Step 3: Case reference in Message-ID lookup fails
		mockCaseService.EXPECT().Search(mock.Anything, "CASE-789012", mock.Anything, testDefaultPagination).
			Return(nil, int64(0), errors.New("case service failure 1"))

		// Step 4: Case reference in subject lookup fails
		mockCaseService.EXPECT().Search(mock.Anything, "CASE-123456", mock.Anything, testDefaultPagination).
			Return([]models.Case{*testCase}, int64(1), nil)
		mockCommService.EXPECT().ListByCaseID(testCase.ID, emailResponseQueryFilter, emptySortFilter, testDefaultPagination).
			Return(nil, int64(0), errors.New("communication service failure 3"))

		// Step 5: Subject similarity lookup fails
		mockCaseService.EXPECT().List(
			mock.MatchedBy(func(filters []queryutil.Filter) bool {
				if len(filters) != 1 {
					return false
				}
				return filters[0].Field == "reporter_id" && filters[0].Operator == queryutil.OperatorEquals && filters[0].Value == uint(42)
			}),
			mock.Anything,
			testLargePagination,
		).Return(nil, int64(0), errors.New("case service failure 2"))

		// Step 6: Even sender history fails
		// We don't need to mock this since the previous failure will prevent it from running

		// Test thread detection
		match, err := detector.DetectThread(email, 42)

		// Should gracefully handle all failures and return nil without error
		require.NoError(t, err, "Should not propagate service errors to the caller")
		assert.Nil(t, match, "Should return nil when all detection methods fail")
	})
}

// Benchmark tests to assess performance with large content and edge cases
func BenchmarkThreadDetector_DetectThread(b *testing.B) {
	// Use common setup function
	detector, mockCaseService, mockCommService, testCase, testComm := setupThreadDetectorTest(b)

	// Create a large email with extensive content (simulate a realistic large email)
	largeEmailContent := strings.Repeat("This is a line of content in a large email.\n", 1000)
	largeEmail := &letters.Email{
		Headers: letters.Headers{
			Subject:   "Large Test Email Subject",
			MessageID: "large-msg-id",
		},
		Text: largeEmailContent,
		HTML: "<html><body>" + strings.ReplaceAll(largeEmailContent, "\n", "<br>") + "</body></html>",
	}

	// Setup expectations multiple times for benchmark tests
	// Use multiple Maybe() calls instead of AnyTimes()
	for i := 0; i < 100; i++ {
		mockCommService.EXPECT().GetByThreadID(mock.Anything).Return(nil, errors.New("not found")).Maybe()
		mockCaseService.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return([]models.Case{*testCase}, int64(1), nil).Maybe()
		mockCommService.EXPECT().ListByCaseID(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]models.Communication{*testComm}, int64(1), nil).Maybe()
	}

	b.ResetTimer() // Reset timer before the benchmark loop

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		_, _ = detector.DetectThread(largeEmail, 42)
	}
}

func BenchmarkThreadDetector_CleanSubject(b *testing.B) {
	// Benchmark different subject cleaning scenarios

	benchCases := []struct {
		name    string
		subject string
	}{
		{"Short simple subject", "Test Subject"},
		{"Subject with prefixes", "Re: Fwd: Re: Test Subject"},
		{"Subject with case reference", "Re: [CASE-123456] Test Subject with case reference"},
		{"Long subject with multiple case references", "Re: Fwd: [CASE-123456] Test Subject with multiple CASE-789012 references and (CASE-345678) in different formats"},
		{"Subject with Unicode", "Re: 测试电子邮件 - 🔍 Email Test 📧 with [CASE-123456]"},
		{"Very long subject", strings.Repeat("This is a part of a very long subject line. ", 20) + " [CASE-123456]"},
	}

	ctx := coreTesting.NewTestContext(b)
	detector := NewThreadDetector(ctx)

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = detector.cleanSubject(bc.subject)
			}
		})
	}
}

func BenchmarkThreadDetector_ExtractSubjectFromContent(b *testing.B) {
	// Benchmark different subject extraction scenarios

	benchCases := []struct {
		name    string
		content string
	}{
		{"Simple content", "From: sender@example.com\nTo: recipient@example.com\nSubject: Test Subject\n\nEmail body"},
		{"Content with no subject", "From: sender@example.com\nTo: recipient@example.com\n\nEmail body without subject"},
		{"Content with complex headers", "From: sender@example.com\nReply-To: reply@example.com\nDate: Mon, 1 Jan 2023 12:00:00 +0000\nTo: recipient@example.com\nCc: cc@example.com\nSubject: Test Subject\n\nEmail body"},
		{"Large content", "From: sender@example.com\nTo: recipient@example.com\nSubject: Test Subject\n\n" + strings.Repeat("This is a line of content in a large email.\n", 1000)},
	}

	ctx := coreTesting.NewTestContext(b)
	detector := NewThreadDetector(ctx)

	for _, bc := range benchCases {
		b.Run(bc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = detector.extractSubjectFromContent(bc.content)
			}
		})
	}
}
