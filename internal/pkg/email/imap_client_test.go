package email

import (
	"context"
	"crypto/tls"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/config"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// TestIMAPClientDefault_SetEmailHandler tests the SetEmailHandler method
func TestIMAPClientDefault_SetEmailHandler(t *testing.T) {
	// Create a client with a handler
	imapClient := &IMAPClientDefault{}

	// Define a handler function
	handler := func(ctx context.Context, data io.Reader) error {
		return nil
	}

	// Set the handler
	imapClient.SetEmailHandler(handler)

	// Verify the handler was set
	assert.NotNil(t, imapClient.emailHandler, "Handler should be set")
	// Can't compare functions directly
	assert.NotNil(t, imapClient.emailHandler, "Handler function should be set")
}

// TestIMAPClientDefault_Start tests the Start method
func TestIMAPClientDefault_Start(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create email config
	emailConfig := &config.EmailConfig{
		ReceiveEnabled: true,
		IMAPHost:       "imap.test.com",
		IMAPPort:       993,
		IMAPUser:       "test@example.com",
		IMAPPassword:   "password123",
		IMAPMailbox:    "INBOX",
		PollInterval:   60,
	}

	// Create mock IMAP client connection
	mockConn := NewMockIMAPClientConn(t)
	mockConn.On("Login", "test@example.com", "password123").Return(nil)

	// Create mock IMAP dialer
	mockDialer := NewMockIMAPDialer(t)
	mockDialer.On("DialTLS", "imap.test.com:993", (*tls.Config)(nil)).Return(mockConn, nil)

	// Create the IMAP client with our mock dialer
	imapClient := &IMAPClientDefault{
		ctx:          ctx,
		logger:       ctx.NamedLogger("test-imap"),
		config:       emailConfig,
		stopChan:     make(chan struct{}),
		pollInterval: 60 * time.Second,
		dialer:       mockDialer,
	}

	// Start the IMAP client
	err := imapClient.Start()

	// Assertions
	assert.NoError(t, err, "Start should not return an error")
	assert.True(t, imapClient.running, "Client should be marked as running")

	// Verify mock expectations
	mockDialer.AssertExpectations(t)
	mockConn.AssertExpectations(t)

	// Cleanup
	mockConn.On("Logout").Return(nil)
	imapClient.Stop()
}

// TestIMAPClientDefault_Start_Disabled tests starting with ReceiveEnabled=false
func TestIMAPClientDefault_Start_Disabled(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create email config with receiving disabled
	emailConfig := &config.EmailConfig{
		ReceiveEnabled: false,
		IMAPHost:       "imap.test.com",
		IMAPPort:       993,
		IMAPUser:       "test@example.com",
		IMAPPassword:   "password123",
	}

	// Create the IMAP client
	imapClient := &IMAPClientDefault{
		ctx:      ctx,
		logger:   ctx.NamedLogger("test-imap"),
		config:   emailConfig,
		stopChan: make(chan struct{}),
	}

	// Start the IMAP client
	err := imapClient.Start()

	// Assertions
	assert.NoError(t, err, "Start should not return an error when disabled")
	assert.False(t, imapClient.running, "Client should not be marked as running when disabled")
}

// TestIMAPClientDefault_Start_ConnectionError tests handling of connection errors
func TestIMAPClientDefault_Start_ConnectionError(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create email config
	emailConfig := &config.EmailConfig{
		ReceiveEnabled: true,
		IMAPHost:       "imap.test.com",
		IMAPPort:       993,
		IMAPUser:       "test@example.com",
		IMAPPassword:   "password123",
	}

	// Create mock IMAP dialer that returns an error
	mockDialer := NewMockIMAPDialer(t)
	mockDialer.On("DialTLS", "imap.test.com:993", (*tls.Config)(nil)).Return(nil, assert.AnError)

	// Create the IMAP client with our mock dialer
	imapClient := &IMAPClientDefault{
		ctx:      ctx,
		logger:   ctx.NamedLogger("test-imap"),
		config:   emailConfig,
		stopChan: make(chan struct{}),
		dialer:   mockDialer,
	}

	// Start the IMAP client - should fail
	err := imapClient.Start()

	// Assertions
	assert.Error(t, err, "Start should return an error when connection fails")
	assert.False(t, imapClient.running, "Client should not be marked as running when connection fails")
	assert.Contains(t, err.Error(), "failed to connect to IMAP server", "Error message should indicate connection failure")

	// Verify mock expectations
	mockDialer.AssertExpectations(t)
}

// TestIMAPClientDefault_Start_LoginError tests handling of login errors
func TestIMAPClientDefault_Start_LoginError(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create email config
	emailConfig := &config.EmailConfig{
		ReceiveEnabled: true,
		IMAPHost:       "imap.test.com",
		IMAPPort:       993,
		IMAPUser:       "test@example.com",
		IMAPPassword:   "password123",
	}

	// Create mock IMAP client connection that fails login
	mockConn := NewMockIMAPClientConn(t)
	mockConn.On("Login", "test@example.com", "password123").Return(assert.AnError)

	// Create mock IMAP dialer
	mockDialer := NewMockIMAPDialer(t)
	mockDialer.On("DialTLS", "imap.test.com:993", (*tls.Config)(nil)).Return(mockConn, nil)

	// Create the IMAP client with our mock dialer
	imapClient := &IMAPClientDefault{
		ctx:      ctx,
		logger:   ctx.NamedLogger("test-imap"),
		config:   emailConfig,
		stopChan: make(chan struct{}),
		dialer:   mockDialer,
	}

	// Start the IMAP client - should fail during login
	err := imapClient.Start()

	// Assertions
	assert.Error(t, err, "Start should return an error when login fails")
	assert.False(t, imapClient.running, "Client should not be marked as running when login fails")
	assert.Contains(t, err.Error(), "failed to login to IMAP server", "Error message should indicate login failure")

	// Verify mock expectations
	mockDialer.AssertExpectations(t)
	mockConn.AssertExpectations(t)
}

// TestIMAPClientDefault_Stop tests the Stop method
func TestIMAPClientDefault_Stop(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create a mock IMAP client connection
	mockConn := NewMockIMAPClientConn(t)
	mockConn.On("Logout").Return(nil)

	// Create the IMAP client and set it as running
	imapClient := &IMAPClientDefault{
		ctx:          ctx,
		logger:       ctx.NamedLogger("test-imap"),
		config:       &config.EmailConfig{},
		client:       mockConn,
		running:      true,
		stopChan:     make(chan struct{}),
		pollInterval: 60 * time.Second,
	}

	// Create a WaitGroup and add one to it
	imapClient.waitGroup.Add(1)

	// Start a goroutine that will decrement the WaitGroup when stopChan is closed
	go func() {
		<-imapClient.stopChan
		imapClient.waitGroup.Done()
	}()

	// Stop the IMAP client
	err := imapClient.Stop()

	// Assertions
	assert.NoError(t, err, "Stop should not return an error")
	assert.False(t, imapClient.running, "Client should not be marked as running after stopping")

	// Verify mock expectations
	mockConn.AssertExpectations(t)
}

// TestIMAPClientDefault_Stop_AlreadyStopped tests stopping an already stopped client
func TestIMAPClientDefault_Stop_AlreadyStopped(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create the IMAP client and set it as not running
	imapClient := &IMAPClientDefault{
		ctx:      ctx,
		logger:   ctx.NamedLogger("test-imap"),
		config:   &config.EmailConfig{},
		running:  false,
		stopChan: make(chan struct{}),
	}

	// Stop the already stopped IMAP client
	err := imapClient.Stop()

	// Assertions
	assert.NoError(t, err, "Stop should not return an error when already stopped")
	assert.False(t, imapClient.running, "Client should remain marked as not running")
}

// TestIMAPClientDefault_CheckForNewEmails tests the checkForNewEmails method
func TestIMAPClientDefault_CheckForNewEmails(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create email config
	emailConfig := &config.EmailConfig{
		ReceiveEnabled: true,
		IMAPHost:       "imap.test.com",
		IMAPPort:       993,
		IMAPUser:       "test@example.com",
		IMAPPassword:   "password123",
		IMAPMailbox:    "INBOX",
	}

	// Create mailbox status to return
	mboxStatus := &imap.MailboxStatus{
		Messages: 2, // 2 messages in the mailbox
	}

	// Create a mock IMAP client connection
	mockConn := NewMockIMAPClientConn(t)

	// Setup mock expectations for the sequence of IMAP calls
	mockConn.On("Select", "INBOX", false).Return(mboxStatus, nil)

	// Setup search results - return some message IDs
	messageIDs := []uint32{1, 2}
	mockConn.On("Search", mock.AnythingOfType("*imap.SearchCriteria")).Return(messageIDs, nil)

	// Setup the mock to handle the Fetch call by sending messages to the channel
	mockConn.On("Fetch", mock.AnythingOfType("*imap.SeqSet"), mock.AnythingOfType("[]imap.FetchItem"), mock.AnythingOfType("chan *imap.Message")).
		Run(func(args mock.Arguments) {
			// Extract the channel argument
			ch := args.Get(2).(chan *imap.Message)

			// Create a test message with Body map properly populated
			bodySection := &imap.BodySectionName{}

			// First message
			msg := &imap.Message{
				Uid:   1,
				Items: make(map[imap.FetchItem]interface{}),
				Body:  make(map[*imap.BodySectionName]imap.Literal),
			}

			// Populate the Body map with test content
			msg.Body[bodySection] = strings.NewReader("From: test@example.com\r\nSubject: Test\r\n\r\nThis is a test email")

			// Send the first message to the channel
			ch <- msg

			// Second message
			msg2 := &imap.Message{
				Uid:   2,
				Items: make(map[imap.FetchItem]interface{}),
				Body:  make(map[*imap.BodySectionName]imap.Literal),
			}

			// Populate the Body map with test content
			msg2.Body[bodySection] = strings.NewReader("From: test2@example.com\r\nSubject: Test 2\r\n\r\nThis is another test email")

			// Send the second message to the channel
			ch <- msg2

			// Close the channel to signal completion
			close(ch)
		}).Return(nil)

	// Setup Store call to mark messages as read
	// Use a nil channel of the correct type
	var nilChannel chan *imap.Message
	// Match AddFlags as the exact StoreItem constant
	mockConn.On("Store", mock.AnythingOfType("*imap.SeqSet"),
		mock.MatchedBy(func(item imap.StoreItem) bool {
			return item == imap.AddFlags // Compare with actual constant
		}),
		[]interface{}{imap.SeenFlag},
		nilChannel).Return(nil)

	// Create a handler that will be called for each email
	emailsProcessed := 0
	emailHandler := func(ctx context.Context, data io.Reader) error {
		// Read the email data to verify it
		buf := new(strings.Builder)
		_, err := io.Copy(buf, data)
		require.NoError(t, err, "Should be able to read email data")

		// Verify the email contains expected data
		assert.Contains(t, buf.String(), "Subject: Test", "Email should contain the subject")

		emailsProcessed++
		return nil
	}

	// Create the IMAP client and set required fields
	imapClient := &IMAPClientDefault{
		ctx:          ctx,
		logger:       ctx.NamedLogger("test-imap"),
		config:       emailConfig,
		client:       mockConn,
		emailHandler: emailHandler,
		stopChan:     make(chan struct{}),
	}

	// Call checkForNewEmails
	imapClient.checkForNewEmails()

	// Assertions
	assert.Equal(t, 2, emailsProcessed, "Both emails should be processed")

	// Verify all mock expectations
	mockConn.AssertExpectations(t)
}

// TestIMAPClientDefault_CheckForNewEmails_NoClient tests when client is nil
func TestIMAPClientDefault_CheckForNewEmails_NoClient(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create the IMAP client
	imapClient := &IMAPClientDefault{
		ctx:      ctx,
		logger:   ctx.NamedLogger("test-imap"),
		config:   &config.EmailConfig{},
		client:   nil, // No client
		stopChan: make(chan struct{}),
	}

	// This should not panic
	imapClient.checkForNewEmails()

	// No assertions needed - the test passes if it doesn't panic
}

// TestIMAPClientDefault_CheckForNewEmails_NoHandler tests when handler is nil
func TestIMAPClientDefault_CheckForNewEmails_NoHandler(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create a mock IMAP client connection
	mockConn := NewMockIMAPClientConn(t)

	// Create the IMAP client
	imapClient := &IMAPClientDefault{
		ctx:          ctx,
		logger:       ctx.NamedLogger("test-imap"),
		config:       &config.EmailConfig{},
		client:       mockConn,
		emailHandler: nil, // No handler
		stopChan:     make(chan struct{}),
	}

	// This should not panic
	imapClient.checkForNewEmails()

	// No assertions needed - the test passes if it doesn't panic
}

// TestIMAPClientDefault_CheckForNewEmails_SelectError tests error during mailbox selection
func TestIMAPClientDefault_CheckForNewEmails_SelectError(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create email config
	emailConfig := &config.EmailConfig{
		IMAPMailbox: "INBOX",
	}

	// Create a mock IMAP client connection
	mockConn := NewMockIMAPClientConn(t)

	// Setup mock to return an error on Select
	mockConn.On("Select", "INBOX", false).Return(&imap.MailboxStatus{}, assert.AnError)

	// Create the IMAP client
	imapClient := &IMAPClientDefault{
		ctx:          ctx,
		logger:       ctx.NamedLogger("test-imap"),
		config:       emailConfig,
		client:       mockConn,
		emailHandler: func(ctx context.Context, data io.Reader) error { return nil },
		stopChan:     make(chan struct{}),
	}

	// Call checkForNewEmails - should handle the error gracefully
	imapClient.checkForNewEmails()

	// Verify all mock expectations
	mockConn.AssertExpectations(t)
}

// TestIMAPClientDefault_CheckForNewEmails_NoMessages tests when mailbox has no messages
func TestIMAPClientDefault_CheckForNewEmails_NoMessages(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create email config
	emailConfig := &config.EmailConfig{
		IMAPMailbox: "INBOX",
	}

	// Create a mock IMAP client connection
	mockConn := NewMockIMAPClientConn(t)

	// Setup mock to return an empty mailbox
	mboxStatus := &imap.MailboxStatus{
		Messages: 0, // No messages in the mailbox
	}
	mockConn.On("Select", "INBOX", false).Return(mboxStatus, nil)

	// Create the IMAP client
	imapClient := &IMAPClientDefault{
		ctx:          ctx,
		logger:       ctx.NamedLogger("test-imap"),
		config:       emailConfig,
		client:       mockConn,
		emailHandler: func(ctx context.Context, data io.Reader) error { return nil },
		stopChan:     make(chan struct{}),
	}

	// Call checkForNewEmails - should exit early
	imapClient.checkForNewEmails()

	// Verify all mock expectations
	mockConn.AssertExpectations(t)
}

// TestIMAPClientDefault_CheckForNewEmails_SearchError tests error during search
func TestIMAPClientDefault_CheckForNewEmails_SearchError(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create email config
	emailConfig := &config.EmailConfig{
		IMAPMailbox: "INBOX",
	}

	// Create a mock IMAP client connection
	mockConn := NewMockIMAPClientConn(t)

	// Setup mock to return a mailbox with messages
	mboxStatus := &imap.MailboxStatus{
		Messages: 2, // 2 messages in the mailbox
	}
	mockConn.On("Select", "INBOX", false).Return(mboxStatus, nil)

	// Return an error on Search
	mockConn.On("Search", mock.AnythingOfType("*imap.SearchCriteria")).Return([]uint32{}, assert.AnError)

	// Create the IMAP client
	imapClient := &IMAPClientDefault{
		ctx:          ctx,
		logger:       ctx.NamedLogger("test-imap"),
		config:       emailConfig,
		client:       mockConn,
		emailHandler: func(ctx context.Context, data io.Reader) error { return nil },
		stopChan:     make(chan struct{}),
	}

	// Call checkForNewEmails - should handle the error gracefully
	imapClient.checkForNewEmails()

	// Verify all mock expectations
	mockConn.AssertExpectations(t)
}

// TestNewIMAPClient tests the default IMAP client factory
func TestNewIMAPClient(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create email config
	emailConfig := &config.EmailConfig{
		PollInterval: 120, // 2 minutes
	}

	// Call the factory function
	client := defaultNewIMAPClient(ctx, emailConfig)

	// Assertions
	assert.NotNil(t, client, "Client should not be nil")

	// Type assertion to access unexported fields
	defaultClient, ok := client.(*IMAPClientDefault)
	assert.True(t, ok, "Client should be of type *IMAPClientDefault")

	// Check fields were set correctly
	assert.Equal(t, ctx, defaultClient.ctx, "Context should be set")
	assert.NotNil(t, defaultClient.logger, "Logger should be initialized")
	assert.Equal(t, emailConfig, defaultClient.config, "Config should be set")
	assert.NotNil(t, defaultClient.stopChan, "Stop channel should be initialized")
	assert.Equal(t, 120*time.Second, defaultClient.pollInterval, "Poll interval should be set to 120 seconds")
	assert.Equal(t, DefaultDialer, defaultClient.dialer, "Dialer should be set to DefaultDialer")
}

// TestNewIMAPClient_DefaultPollInterval tests default poll interval
func TestNewIMAPClient_DefaultPollInterval(t *testing.T) {
	// Create a test context
	ctx := coreTesting.NewTestContext(t)

	// Create email config with no poll interval
	emailConfig := &config.EmailConfig{
		PollInterval: 0, // Not set, should use default
	}

	// Call the factory function
	client := defaultNewIMAPClient(ctx, emailConfig)

	// Type assertion to access unexported fields
	defaultClient, ok := client.(*IMAPClientDefault)
	assert.True(t, ok, "Client should be of type *IMAPClientDefault")

	// Check the default poll interval was used
	assert.Equal(t, 5*time.Minute, defaultClient.pollInterval, "Default poll interval should be 5 minutes")
}
