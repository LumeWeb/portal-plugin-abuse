package email

import (
	"context"
	"crypto/tls"
	"io"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

// IMAPClientConn defines the interface for IMAP client connections
// This allows us to mock the client.Client from the go-imap library
type IMAPClientConn interface {
	// Login authenticates the client with the given credentials
	Login(username, password string) error

	// Logout logs out the client and closes the connection
	Logout() error

	// Select selects a mailbox for subsequent commands
	Select(mailbox string, readOnly bool) (*imap.MailboxStatus, error)

	// Search searches for messages matching the given criteria
	Search(criteria *imap.SearchCriteria) ([]uint32, error)

	// Fetch retrieves messages matching the given sequence set and items
	Fetch(seqSet *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error

	// Store updates message flags
	Store(seqSet *imap.SeqSet, item imap.StoreItem, flags []interface{}, ch chan *imap.Message) error
	
	// Noop sends a NOOP command to keep the connection alive
	Noop() error
}

// IMAPDialer defines the interface for connecting to an IMAP server
type IMAPDialer interface {
	// DialTLS connects to an IMAP server using TLS
	DialTLS(addr string, tlsConfig *tls.Config) (IMAPClientConn, error)
}

// EmailHandlerFunc defines a function to handle incoming emails
type EmailHandlerFunc func(ctx context.Context, data io.Reader) error

// DefaultIMAPDialer is the default implementation of IMAPDialer
type DefaultIMAPDialer struct{}

// DialTLS connects to an IMAP server using TLS
func (d *DefaultIMAPDialer) DialTLS(addr string, tlsConfig *tls.Config) (IMAPClientConn, error) {
	// Use the actual client.DialTLS function, but wrap the result in our interface
	conn, err := client.DialTLS(addr, tlsConfig)
	if err != nil {
		return nil, err
	}

	// Wrap the client.Client in our adapter
	return &IMAPClientAdapter{client: conn}, nil
}

// IMAPClientAdapter adapts client.Client to our IMAPClientConn interface
type IMAPClientAdapter struct {
	client *client.Client
}

// Login implements IMAPClientConn
func (a *IMAPClientAdapter) Login(username, password string) error {
	return a.client.Login(username, password)
}

// Logout implements IMAPClientConn
func (a *IMAPClientAdapter) Logout() error {
	return a.client.Logout()
}

// Select implements IMAPClientConn
func (a *IMAPClientAdapter) Select(mailbox string, readOnly bool) (*imap.MailboxStatus, error) {
	return a.client.Select(mailbox, readOnly)
}

// Search implements IMAPClientConn
func (a *IMAPClientAdapter) Search(criteria *imap.SearchCriteria) ([]uint32, error) {
	return a.client.Search(criteria)
}

// Fetch implements IMAPClientConn
func (a *IMAPClientAdapter) Fetch(seqSet *imap.SeqSet, items []imap.FetchItem, ch chan *imap.Message) error {
	return a.client.Fetch(seqSet, items, ch)
}

// Store implements IMAPClientConn
func (a *IMAPClientAdapter) Store(seqSet *imap.SeqSet, item imap.StoreItem, flags []interface{}, ch chan *imap.Message) error {
	return a.client.Store(seqSet, item, flags, ch)
}

// Noop implements IMAPClientConn
func (a *IMAPClientAdapter) Noop() error {
	return a.client.Noop()
}

// Package level variable for our IMAP dialer, which can be replaced in tests
var DefaultDialer IMAPDialer = &DefaultIMAPDialer{}
