package email

import (
	"context"
	"fmt"
	tfidf "github.com/dkgv/go-tf-idf"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/config"
	"go.lumeweb.com/portal-plugin-abuse/internal/pkg/email/interfaces"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// Helper function to create a test context with email config
func createTestContext(t *testing.T) (coreTesting.TestContext, *mocks.MockCaseService, *mocks.MockCommunicationService) {
	// Create a test context with necessary configurations
	emailConfig := &config.EmailConfig{
		// IMAP settings
		ReceiveEnabled:   true,
		IMAPHost:         "imap.test.example.com",
		IMAPPort:         993,
		IMAPUser:         "test@example.com",
		IMAPPassword:     "password123",
		IMAPMailbox:      "INBOX",
		PollInterval:     300,
		ReceiveAddresses: []string{"abuse@test.example.com"},

		// SMTP settings
		SMTPHost:           "smtp.test.example.com",
		SMTPPort:           587,
		ReceiveBindAddress: "127.0.0.1", // Legacy
		ReceivePort:        2525,        // Legacy
	}

	testCtx := coreTesting.NewTestContext(t)

	// Create mock communication service
	mockCommService := mocks.NewMockCommunicationService(t)
	mockCaseService := mocks.NewMockCaseService(t)

	testCtx.RegisterService(typesSvc.COMMUNICATION_SERVICE, mockCommService)
	testCtx.RegisterService(typesSvc.CASE_SERVICE, mockCaseService)

	// Set up the mock expectations for ConfigureService
	mockCfg, ok := testCtx.Config().(*coreTesting.MockConfigManager)
	if !ok {
		t.Fatalf("Config should be a MockConfigManager")
	}

	// Set up the mock to return no error for the ConfigureService call
	mockCfg.MockManager.On("ConfigureService", "abuse", typesSvc.EMAIL_SERVICE, emailConfig).Return(nil)

	// Call ConfigureService to configure the service
	err := mockCfg.ConfigureService("abuse", typesSvc.EMAIL_SERVICE, emailConfig)
	if err != nil {
		t.Fatalf("Failed to configure email service: %v", err)
	}

	// Set up the mock to return the emailConfig from GetService, but only when needed
	mockCfg.MockManager.On("GetService", typesSvc.EMAIL_SERVICE).Return(emailConfig).Maybe()

	return testCtx, mockCaseService, mockCommService
}

// newTestPipeline creates a new pipeline instance with all required dependencies for testing
func newTestPipeline(t *testing.T) (*PipelineDefault, *mocks.MockCommunicationService, *mocks.MockCaseService) {
	ctx, mockCaseService, mockCommService := createTestContext(t)

	pipeline := NewPipeline(
		ctx,
		NewARFProcessor(ctx, NewContentExtractor(ctx.Logger())),
		NewClassifier(ctx),
		NewThreadDetector(ctx),
		NewTemplateProcessor(ctx, NewContentExtractor(ctx.Logger())),
	)

	// Manually inject test configuration
	testConfig := &config.EmailConfig{
		ReceiveEnabled:   true,
		IMAPHost:         "imap.test.example.com",
		IMAPPort:         993,
		IMAPUser:         "test@example.com",
		IMAPPassword:     "password123",
		IMAPMailbox:      "INBOX",
		PollInterval:     300,
		ReceiveAddresses: []string{"abuse@test.example.com"},
	}

	pipeline.SetConfigCallback(func() *config.EmailConfig {
		return testConfig
	})

	return pipeline, mockCommService, mockCaseService
}

func TestPipeline_NewPipeline(t *testing.T) {
	pipeline, _, _ := newTestPipeline(t)

	assert.NotNil(t, pipeline, "Pipeline should not be nil")
	assert.NotNil(t, pipeline.metrics, "Metrics should be initialized")
	assert.NotNil(t, pipeline.arfProcessor, "ARF processor should be initialized")
	assert.NotNil(t, pipeline.classifier, "Classifier should be initialized")
	assert.NotNil(t, pipeline.threadDetector, "Thread detector should be initialized")
	assert.NotNil(t, pipeline.templateProc, "Template processor should be initialized")
}

func TestPipeline_Start(t *testing.T) {
	pipeline, _, _ := newTestPipeline(t)

	// Create a mock IMAP client
	mockClient := new(MockIMAPClient)
	mockClient.On("SetEmailHandler", mock.Anything).Return()
	mockClient.On("Start").Return(nil)

	// Replace the NewIMAPClient function temporarily
	originalNewIMAPClient := NewIMAPClient
	defer func() { NewIMAPClient = originalNewIMAPClient }()
	NewIMAPClient = func(ctx core.Context, config *config.EmailConfig) IMAPClient {
		return mockClient
	}

	// Test starting the pipeline
	var processor Processor = func(ctx context.Context, data io.Reader) error {
		return nil
	}

	err := pipeline.Start(processor)
	assert.NoError(t, err, "Start should not return an error")
	assert.True(t, pipeline.started, "Pipeline should be marked as started")
	assert.NotNil(t, pipeline.processor, "Processor should be stored")
	mockClient.AssertExpectations(t)
}

func TestPipeline_Start_AlreadyStarted(t *testing.T) {
	pipeline, _, _ := newTestPipeline(t)

	// Create a mock IMAP client
	mockClient := new(MockIMAPClient)
	mockClient.On("SetEmailHandler", mock.Anything).Return()
	mockClient.On("Start").Return(nil)

	// Replace the NewIMAPClient function temporarily
	originalNewIMAPClient := NewIMAPClient
	defer func() { NewIMAPClient = originalNewIMAPClient }()
	NewIMAPClient = func(ctx core.Context, config *config.EmailConfig) IMAPClient {
		return mockClient
	}

	// First start
	var processor Processor = func(ctx context.Context, data io.Reader) error {
		return nil
	}
	err := pipeline.Start(processor)
	assert.NoError(t, err, "First start should succeed")

	// Second start attempt
	err = pipeline.Start(processor)
	assert.NoError(t, err, "Subsequent starts should be no-ops")
	assert.True(t, pipeline.started, "Pipeline should remain started")
	mockClient.AssertExpectations(t)
}

// MockIMAPClient implements IMAPClient interface for testing
type MockIMAPClient struct {
	mock.Mock
	HandlerFunction interfaces.EmailHandlerFunc
}

func (m *MockIMAPClient) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockIMAPClient) Stop() error {
	args := m.Called()
	return args.Error(0)
}

// SetEmailHandler uses a custom matcher to handle function comparison
func (m *MockIMAPClient) SetEmailHandler(handler interfaces.EmailHandlerFunc) {
	// Store the handler function so we can use it for verification
	m.HandlerFunction = handler

	// Rather than comparing function pointers (which doesn't work well),
	// we just note that the method was called with any function
	m.Called(mock.AnythingOfType("interfaces.EmailHandlerFunc"))
}

func TestPipeline_Stop(t *testing.T) {
	pipeline, _, _ := newTestPipeline(t)

	// Set up a mock IMAP client
	mockClient := new(MockIMAPClient)
	mockClient.On("Stop").Return(nil)

	// Start the pipeline with the mock client
	pipeline.emailClient = mockClient
	pipeline.started = true

	// Test stopping the pipeline
	err := pipeline.Stop()

	assert.NoError(t, err, "Stop should not return an error")
	assert.True(t, pipeline.stopped, "Pipeline should be marked as stopped")
	mockClient.AssertExpectations(t)
}

func TestPipeline_Stop_AlreadyStopped(t *testing.T) {
	pipeline, _, _ := newTestPipeline(t)
	pipeline.stopped = true // Mark as already stopped

	err := pipeline.Stop()
	assert.NoError(t, err, "Stopping an already stopped pipeline should not error")
}

func TestPipeline_ProcessEmail_Success_NewCase(t *testing.T) {
	pipeline, _, mockCaseService := newTestPipeline(t)
	// Mock empty case list response for thread detection
	mockCaseService.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.Case{}, int64(0), nil)

	// Create classifier with test-specific weights through public API
	testClassifier := NewClassifier(pipeline.ctx,
		WithClassifierWeights(2, 0.5, 0.2),
		WithClassifierTFIDF(tfidf.New(tfidf.WithDefaultStopWords())),
	)

	// Create new pipeline with configured classifier
	pipeline = NewPipeline(
		pipeline.ctx,
		NewARFProcessor(pipeline.ctx, NewContentExtractor(pipeline.ctx.Logger())),
		testClassifier,
		pipeline.threadDetector,
		pipeline.templateProc,
	)
	pipeline.arfProcessor = &ARFProcessor{
		contentExtractor: &ContentExtractor{
			logger:  pipeline.logger,
			options: &ContentExtractorOptions{},
		},
	}

	// Process a properly formatted sample email with complete MIME structure
	data := strings.NewReader(`From: reporter@example.com
Subject: Abuse report
MIME-Version: 1.0
Content-Type: multipart/alternative; boundary="==BOUNDARY=="

--==BOUNDARY==
Content-Type: text/plain; charset=UTF-8
Content-Transfer-Encoding: 7bit

Test content

--==BOUNDARY==
Content-Type: text/html; charset=UTF-8
Content-Transfer-Encoding: 7bit

<div>Test content</div>

--==BOUNDARY==--`)
	result, err := pipeline.ProcessEmail(context.Background(), data)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Case, "Should create new case")
	assert.Equal(t, models.CaseStatusNew, result.Case.Status)
	assert.Contains(t, result.Case.Description, "Email Subject: Abuse report")

	pipeline.metrics.mutex.Lock()
	defer pipeline.metrics.mutex.Unlock()
	assert.Equal(t, int64(1), pipeline.metrics.totalReceived)
	assert.Equal(t, int64(1), pipeline.metrics.totalProcessed)
	assert.Equal(t, int64(1), pipeline.metrics.totalNew)
}

func TestPipeline_ProcessEmail_Success_ARF(t *testing.T) {
	pipeline, _, _ := newTestPipeline(t)

	// Sample ARF email data with valid multipart structure
	arfData := `From: abuse@example.com
Content-Type: multipart/report; report-type=feedback-report; boundary="boundary123"
MIME-Version: 1.0

--boundary123
Content-Type: text/plain; charset="UTF-8"

This is a human-readable abuse report

--boundary123
Content-Type: message/feedback-report

Feedback-Type: abuse
User-Agent: TestAgent/1.0
Arrival-Date: Wed, 31 Mar 2025 18:44:40 -0400

--boundary123
Content-Type: message/rfc822

Received: from example.com
Subject: Test abuse report
From: <abuser@example.com>

Original message content
--boundary123--`
	data := strings.NewReader(arfData)

	// Process through the public API
	result, err := pipeline.ProcessEmail(context.Background(), data)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsARF, "Should detect ARF report")
	assert.NotNil(t, result.ARFData, "Should contain ARF data")
	assert.Equal(t, "abuse", result.ARFData.FeedbackType)
	assert.Equal(t, "TestAgent/1.0", result.ARFData.UserAgent)

	pipeline.metrics.mutex.Lock()
	defer pipeline.metrics.mutex.Unlock()
	assert.Equal(t, int64(1), pipeline.metrics.totalARF)
}

func TestPipeline_ProcessEmail_Error_InvalidEmail(t *testing.T) {
	pipeline, _, _ := newTestPipeline(t)

	// Invalid email data (missing headers)
	data := strings.NewReader("invalid email content")
	_, err := pipeline.ProcessEmail(context.Background(), data)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse email")

	pipeline.metrics.mutex.Lock()
	defer pipeline.metrics.mutex.Unlock()
	assert.Equal(t, int64(1), pipeline.metrics.totalErrors)
}

func TestPipeline_ProcessEmail_Success_ThreadMatch(t *testing.T) {
	pipeline, _, _ := newTestPipeline(t)

	// Mock thread detector to return a match
	pipeline.threadDetector = &ThreadDetector{
		db: pipeline.ctx.DB(),
	}

	// Create existing thread in database
	comm := &models.Communication{
		ThreadID:  "12345",
		Content:   "Original report",
		Direction: models.CommunicationDirectionIncoming,
	}
	assert.NoError(t, pipeline.ctx.DB().Create(comm).Error)

	// Process email with matching thread ID
	data := strings.NewReader(fmt.Sprintf(`From: user@example.com
References: <12345>
Subject: Re: Original report

Follow-up message`))

	result, err := pipeline.ProcessEmail(context.Background(), data)

	assert.NoError(t, err)
	assert.NotNil(t, result.ThreadMatch, "Should detect thread match")
	assert.Equal(t, uint(comm.ID), result.ThreadMatch.Communication.ID)

	pipeline.metrics.mutex.Lock()
	defer pipeline.metrics.mutex.Unlock()
	assert.Equal(t, int64(1), pipeline.metrics.totalThreaded)
}

func TestPipeline_GetMetrics(t *testing.T) {
	pipeline, _, _ := newTestPipeline(t)

	// Set some test metrics
	pipeline.metrics.mutex.Lock()
	pipeline.metrics.totalReceived = 100
	pipeline.metrics.totalProcessed = 90
	pipeline.metrics.totalErrors = 10
	pipeline.metrics.processingTimes = []time.Duration{500 * time.Millisecond}
	pipeline.metrics.mutex.Unlock()

	// Get metrics
	metrics := pipeline.GetMetrics()

	// Check metrics values
	assert.Equal(t, int64(100), metrics["total_received"], "total_received")
	assert.Equal(t, int64(90), metrics["total_processed"], "total_processed")
	assert.Equal(t, int64(10), metrics["total_errors"], "total_errors")
	assert.Equal(t, int64(500), metrics["avg_processing_time_ms"], "avg_processing_time_ms")
}

// MockCommunicationService implements CommunicationService for testing
type MockCommunicationService struct {
	mock.Mock
}

func (m *MockCommunicationService) Create(comm *models.Communication) (*models.Communication, error) {
	args := m.Called(comm)
	return args.Get(0).(*models.Communication), args.Error(1)
}

func (m *MockCommunicationService) GetByID(id uint) (*models.Communication, error) {
	args := m.Called(id)
	return args.Get(0).(*models.Communication), args.Error(1)
}

func (m *MockCommunicationService) GetByThreadID(threadID string) (*models.Communication, error) {
	args := m.Called(threadID)
	return args.Get(0).(*models.Communication), args.Error(1)
}

func TestPipeline_ProcessEmailLimitsProcessingTimes(t *testing.T) {
	pipeline, _, _ := newTestPipeline(t)

	// Set up a successful processor
	var successProcessor Processor = func(ctx context.Context, data io.Reader) error {
		return nil
	}
	pipeline.processor = successProcessor

	// Pre-populate with excess processing times
	pipeline.metrics.processingTimes = make([]time.Duration, 1001)
	for i := 0; i < 1001; i++ {
		pipeline.metrics.processingTimes[i] = 100 * time.Millisecond
	}

	// Process an email
	data := strings.NewReader("test email data")
	result, err := pipeline.ProcessEmail(context.Background(), data)

	assert.NoError(t, err, "ProcessEmail should not return an error")
	assert.NotNil(t, result, "Should return processing result")

	pipeline.metrics.mutex.Lock()
	defer pipeline.metrics.mutex.Unlock()
	assert.Equal(t, 1000, len(pipeline.metrics.processingTimes), "Should trim to 1000 entries")
	assert.Equal(t, int64(1), pipeline.metrics.totalProcessed, "totalProcessed should be incremented")
}
