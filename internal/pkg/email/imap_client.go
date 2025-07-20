package email

import (
	"context"
	"fmt"
	"github.com/emersion/go-imap/client"
	"sync"
	"time"

	"github.com/emersion/go-imap"
	"go.lumeweb.com/portal-plugin-abuse/internal/config"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
)

var ErrNotLoggedIn = client.ErrNotLoggedIn

// IMAPClient defines the interface for IMAP clients
type IMAPClient interface {
	Start() error
	Stop() error
	SetEmailHandler(handler EmailHandlerFunc)
}

// IMAPClientDefault is the default implementation of the IMAP client
type IMAPClientDefault struct {
	ctx          core.Context
	logger       *core.Logger
	config       *config.EmailConfig
	client       IMAPClientConn
	running      bool
	stopChan     chan struct{}
	waitGroup    sync.WaitGroup
	emailHandler EmailHandlerFunc
	pollInterval time.Duration
	dialer       IMAPDialer // Allows replacing the dialer in tests
}

// NewIMAPClientFunc is a function type for creating IMAP clients (useful for testing)
type NewIMAPClientFunc func(ctx core.Context, config *config.EmailConfig) IMAPClient

// NewIMAPClient is the default IMAP client factory
var NewIMAPClient NewIMAPClientFunc = defaultNewIMAPClient

// defaultNewIMAPClient creates a new IMAP client
func defaultNewIMAPClient(ctx core.Context, config *config.EmailConfig) IMAPClient {
	// Use configured poll interval or default to 5 minutes
	pollInterval := 5 * time.Minute
	if config.PollInterval > 0 {
		pollInterval = time.Duration(config.PollInterval) * time.Second
	}

	return &IMAPClientDefault{
		ctx:          ctx,
		logger:       ctx.NamedLogger("imap-client"),
		config:       config,
		stopChan:     make(chan struct{}),
		pollInterval: pollInterval,
		dialer:       DefaultDialer, // Initialize with the default dialer
	}
}

// SetEmailHandler sets the handler function for processing incoming emails
func (c *IMAPClientDefault) SetEmailHandler(handler EmailHandlerFunc) {
	c.emailHandler = handler
}

// Start starts the IMAP client and begins polling for new emails
func (c *IMAPClientDefault) Start() error {
	if !c.config.ReceiveEnabled {
		c.logger.Info("IMAP client is disabled")
		return nil
	}

	// Connect to the IMAP server using our dialer
	addr := fmt.Sprintf("%s:%d", c.config.IMAPHost, c.config.IMAPPort)
	imapClient, err := c.dialer.DialTLS(addr, nil)
	if err != nil {
		c.logger.Error("Failed to connect to IMAP server", zap.Error(err))
		return fmt.Errorf("failed to connect to IMAP server: %w", err)
	}

	// Login
	if err := imapClient.Login(c.config.IMAPUser, c.config.IMAPPassword); err != nil {
		c.logger.Error("Failed to login to IMAP server", zap.Error(err))
		return fmt.Errorf("failed to login to IMAP server: %w", err)
	}

	c.client = imapClient
	c.running = true
	c.logger.Info("IMAP client started",
		zap.String("host", c.config.IMAPHost),
		zap.Int("port", c.config.IMAPPort))

	// Start polling for new emails
	c.waitGroup.Add(1)
	go c.pollForEmails()

	return nil
}

// Stop stops the IMAP client
func (c *IMAPClientDefault) Stop() error {
	if !c.running {
		return nil
	}

	// Signal polling goroutine to stop
	close(c.stopChan)

	// Wait for goroutine to finish
	c.waitGroup.Wait()

	// Logout and close the connection
	if c.client != nil {
		err := c.client.Logout()
		if err != nil {
			c.logger.Error("Error logging out from IMAP server", zap.Error(err))
		}
	}

	c.running = false
	c.logger.Info("IMAP client stopped")

	return nil
}

// reconnect establishes a new connection to the IMAP server
func (c *IMAPClientDefault) reconnect() error {
	if c.client != nil {
		_ = c.client.Logout() // Best effort logout
	}

	addr := fmt.Sprintf("%s:%d", c.config.IMAPHost, c.config.IMAPPort)
	imapClient, err := c.dialer.DialTLS(addr, nil)
	if err != nil {
		return fmt.Errorf("failed to reconnect to IMAP server: %w", err)
	}

	if err := imapClient.Login(c.config.IMAPUser, c.config.IMAPPassword); err != nil {
		return fmt.Errorf("failed to relogin to IMAP server: %w", err)
	}

	c.client = imapClient
	c.logger.Info("Successfully reconnected to IMAP server")
	return nil
}

// pollForEmails polls the IMAP server for new emails at regular intervals
func (c *IMAPClientDefault) pollForEmails() {
	defer c.waitGroup.Done()

	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	// Do an initial check for emails
	c.checkForNewEmails()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.checkForNewEmails()
		case <-time.After(5 * time.Minute): // Send NOOP every 5 minutes
			if c.client != nil {
				if err := c.client.Noop(); err != nil {
					c.logger.Warn("IMAP connection appears dead, will reconnect", zap.Error(err))
					if err := c.reconnect(); err != nil {
						c.logger.Error("Failed to reconnect", zap.Error(err))
					}
				}
			}
		}
	}
}

// checkForNewEmails checks for new emails in the configured mailbox
func (c *IMAPClientDefault) checkForNewEmails() {
	if c.client == nil || c.emailHandler == nil {
		return
	}

	// Select the mailbox (usually INBOX)
	mailbox := "INBOX"
	if c.config.IMAPMailbox != "" {
		mailbox = c.config.IMAPMailbox
	}

	mbox, err := c.client.Select(mailbox, false)
	if err != nil {
		if err == ErrNotLoggedIn {
			c.logger.Warn("Session expired, attempting to reconnect...", zap.String("mailbox", mailbox))
			if err := c.reconnect(); err != nil {
				c.logger.Error("Failed to reconnect", zap.Error(err))
				return
			}
			// Retry after reconnecting
			mbox, err = c.client.Select(mailbox, false)
			if err != nil {
				c.logger.Error("Failed to select mailbox after reconnect", zap.String("mailbox", mailbox), zap.Error(err))
				return
			}
		} else {
			c.logger.Error("Failed to select mailbox", zap.String("mailbox", mailbox), zap.Error(err))
			return
		}
	}

	if mbox.Messages == 0 {
		// No messages in the mailbox
		return
	}

	// Search for unread messages
	criteria := imap.NewSearchCriteria()
	criteria.WithoutFlags = []string{imap.SeenFlag}

	ids, err := c.client.Search(criteria)
	if err != nil {
		c.logger.Error("Failed to search for emails", zap.Error(err))
		return
	}

	if len(ids) == 0 {
		// No unread messages
		return
	}

	c.logger.Info("Found new emails", zap.Int("count", len(ids)))

	// Create a sequence set for the unread messages
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(ids...)

	// Fetch the whole message body
	section := &imap.BodySectionName{}
	items := []imap.FetchItem{section.FetchItem()}

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.client.Fetch(seqSet, items, messages)
	}()

	for msg := range messages {
		// Get the message body
		r := msg.GetBody(section)
		if r == nil {
			c.logger.Error("Failed to get message body", zap.Uint32("uid", msg.Uid))
			continue
		}

		// Process the email using the handler
		if err := c.emailHandler(context.Background(), r); err != nil {
			c.logger.Error("Error processing email", zap.Error(err), zap.Uint32("uid", msg.Uid))
		} else {
			// Mark the message as read
			flagsToAdd := []interface{}{imap.SeenFlag}
			err = c.client.Store(seqSet, imap.AddFlags, flagsToAdd, nil)
			if err != nil {
				c.logger.Error("Failed to mark message as read", zap.Error(err), zap.Uint32("uid", msg.Uid))
			}
		}
	}

	if err := <-done; err != nil {
		c.logger.Error("Failed to fetch emails", zap.Error(err))
	}
}
