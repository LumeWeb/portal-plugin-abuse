package email

/*
// newTestPipeline creates a new pipeline instance with all required dependencies for testing
func newTestPipeline(ctx coreTesting.TestContext) (*PipelineDefault, *mocks.MockCommunicationService, *mocks.MockCaseService) {
	mockCaseService := mocks.NewMockCaseService(ctx.T())
	mockCommService := mocks.NewMockCommunicationService(ctx.T())

	pipeline := NewPipeline(
		ctx,
		NewARFProcessor(ctx, NewContentExtractor(ctx.Logger())),
		NewClassifier(ctx),
		NewThreadDetector(ctx),
		NewTemplateProcessor(ctx, NewContentExtractor(ctx.Logger())),
	)

	return pipeline, mockCommService, mockCaseService
}

func TestPipeline_NewPipeline(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		pipeline, _, _ := newTestPipeline(ctx)

		assert.NotNil(tb, pipeline, "Pipeline should not be nil")
		assert.NotNil(tb, pipeline.metrics, "Metrics should be initialized")
		assert.NotNil(tb, pipeline.arfProcessor, "ARF processor should be initialized")
		assert.NotNil(tb, pipeline.classifier, "Classifier should be initialized")
		assert.NotNil(tb, pipeline.threadDetector, "Thread detector should be initialized")
		assert.NotNil(tb, pipeline.templateProc, "Template processor should be initialized")
	})
}

func TestPipeline_Start(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Create a mock IMAP client

		mockClient := NewMockIMAPClient(t)
		mockClient.EXPECT().SetEmailHandler(mock.Anything).Return()
		mockClient.EXPECT().Start()

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

		// Manually create the pipeline and set the config callback
		pipeline, _, _ := newTestPipeline(ctx)
		pipeline.SetConfigCallback(func() *config.EmailConfig {
			return &config.EmailConfig{ReceiveEnabled: true}
		})

		err := pipeline.Start(processor)
		assert.NoError(tb, err, "Start should not return an error")
		assert.True(tb, pipeline.started, "Pipeline should be marked as started")
		assert.NotNil(tb, pipeline.processor, "Processor should be stored")
		mockClient.AssertExpectations(tb)
	})
}

func TestPipeline_Start_AlreadyStarted(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Create a mock IMAP client
		mockClient := NewMockIMAPClient(t)
		mockClient.EXPECT().SetEmailHandler(mock.Anything).Return()
		mockClient.EXPECT().Start()

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

		// Manually create the pipeline and set the config callback
		pipeline, _, _ := newTestPipeline(ctx)
		pipeline.SetConfigCallback(func() *config.EmailConfig {
			return &config.EmailConfig{ReceiveEnabled: true}
		})

		err := pipeline.Start(processor)
		assert.NoError(tb, err, "First start should succeed")

		// Second start attempt
		err = pipeline.Start(processor)
		assert.NoError(tb, err, "Subsequent starts should be no-ops")
		assert.True(tb, pipeline.started, "Pipeline should remain started")
		mockClient.AssertExpectations(tb)
	})
}

func TestPipeline_Stop(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Create a mock IMAP client
		mockClient := NewMockIMAPClient(t)
		mockClient.EXPECT().SetEmailHandler(mock.Anything).Return()
		mockClient.EXPECT().Stop().Return(nil)

		// Create the IMAP client and set it as running
		pipeline := &PipelineDefault{
			ctx:         ctx,
			logger:      ctx.Logger(),
			emailClient: mockClient,
			started:     true,
		}

		// Test stopping the pipeline
		err := pipeline.Stop()

		assert.NoError(tb, err, "Stop should not return an error")
		assert.True(tb, pipeline.stopped, "Pipeline should be marked as stopped")

	})
}

func TestPipeline_Stop_AlreadyStopped(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Create the IMAP client and set it as not running
		pipeline := &PipelineDefault{
			ctx:     ctx,
			logger:  ctx.Logger(),
			started: false,
			stopped: true,
		}

		err := pipeline.Stop()
		assert.NoError(tb, err, "Stopping an already stopped pipeline should not error")
	})
}

func TestPipeline_ProcessEmail_Success_NewCase(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get mock services
		mockCaseService := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		// Mock empty case list response for thread detection
		mockCaseService.On("List", mock.Anything, mock.Anything, mock.Anything).
			Return([]models.Case{}, int64(0), nil)

		// Create classifier with test-specific weights through public API
		testClassifier := NewClassifier(ctx)

		// Create new pipeline with configured classifier
		pipeline, _, _ := newTestPipeline(ctx)
		pipeline.classifier = testClassifier
		pipeline.arfProcessor = &ARFProcessor{
			contentExtractor: &ContentExtractorDefault{
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

		assert.NoError(tb, err)
		assert.NotNil(tb, result)
		assert.NotNil(tb, result.Case, "Should create new case")
		assert.Equal(tb, models.CaseStatusNew, result.Case.Status)
		assert.Contains(tb, result.Case.Description, "Email Subject: Abuse report")

		pipeline.metrics.mutex.Lock()
		defer pipeline.metrics.mutex.Unlock()
		assert.Equal(tb, int64(1), pipeline.metrics.totalReceived)
		assert.Equal(tb, int64(1), pipeline.metrics.totalProcessed)
		assert.Equal(tb, int64(1), pipeline.metrics.totalNew)
	})
}

func TestPipeline_ProcessEmail_Success_ARF(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		pipeline, _, _ := newTestPipeline(ctx)

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

		assert.NoError(tb, err)
		assert.NotNil(tb, result)
		assert.True(tb, result.IsARF, "Should detect ARF report")
		assert.NotNil(tb, result.ARFData, "Should contain ARF data")
		assert.Equal(tb, "abuse", result.ARFData.FeedbackType)
		assert.Equal(tb, "TestAgent/1.0", result.ARFData.UserAgent)

		pipeline.metrics.mutex.Lock()
		defer pipeline.metrics.mutex.Unlock()
		assert.Equal(tb, int64(1), pipeline.metrics.totalARF)
	})
}

func TestPipeline_ProcessEmail_Error_InvalidEmail(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		pipeline, _, _ := newTestPipeline(ctx)

		// Invalid email data (missing headers)
		data := strings.NewReader("invalid email content")
		_, err := pipeline.ProcessEmail(context.Background(), data)

		assert.Error(tb, err)
		assert.Contains(tb, err.Error(), "failed to parse email")

		pipeline.metrics.mutex.Lock()
		defer pipeline.metrics.mutex.Unlock()
		assert.Equal(tb, int64(1), pipeline.metrics.totalErrors)
	})
}

func TestPipeline_ProcessEmail_Success_ThreadMatch(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Create the pipeline
		pipeline, _, _ := newTestPipeline(ctx)

		// Create existing thread in database
		comm := &models.Communication{
			ThreadID:  "12345",
			Content:   "Original report",
			Direction: models.CommunicationDirectionIncoming,
		}
		err := ctx.DB().Create(comm).Error
		require.NoError(tb, err)

		// Process email with matching thread ID
		data := strings.NewReader(fmt.Sprintf(`From: user@example.com
References: <12345>
Subject: Re: Original report

Follow-up message`))

		result, err := pipeline.ProcessEmail(context.Background(), data)

		assert.NoError(tb, err)
		assert.NotNil(tb, result.ThreadMatch, "Should detect thread match")
		assert.Equal(tb, uint(comm.ID), result.ThreadMatch.Communication.ID)

		pipeline.metrics.mutex.Lock()
		defer pipeline.metrics.mutex.Unlock()
		assert.Equal(tb, int64(1), pipeline.metrics.totalThreaded)
	})
}

func TestPipeline_GetMetrics(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		pipeline, _, _ := newTestPipeline(ctx)

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
		assert.Equal(tb, int64(100), metrics["total_received"], "total_received")
		assert.Equal(tb, int64(90), metrics["total_processed"], "total_processed")
		assert.Equal(tb, int64(10), metrics["total_errors"], "total_errors")
		assert.Equal(tb, int64(500), metrics["avg_processing_time_ms"], "avg_processing_time_ms")
	})
}

func TestPipeline_ProcessEmailLimitsProcessingTimes(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		pipeline, _, _ := newTestPipeline(ctx)

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

		assert.NoError(tb, err, "ProcessEmail should not return an error")
		assert.NotNil(tb, result, "Should return processing result")

		pipeline.metrics.mutex.Lock()
		defer pipeline.metrics.mutex.Unlock()
		assert.Equal(tb, 1000, len(pipeline.metrics.processingTimes), "Should trim to 1000 entries")
		assert.Equal(tb, int64(1), pipeline.metrics.totalProcessed, "totalProcessed should be incremented")
	})
}
*/
