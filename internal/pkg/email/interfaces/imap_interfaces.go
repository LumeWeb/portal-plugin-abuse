package interfaces

import (
	"context"
	"crypto/tls"
	"io"

	"github.com/emersion/go-imap"
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
}

// IMAPDialer defines the interface for connecting to an IMAP server
type IMAPDialer interface {
	// DialTLS connects to an IMAP server using TLS
	DialTLS(addr string, tlsConfig *tls.Config) (IMAPClientConn, error)
}

// EmailHandlerFunc defines a function to handle incoming emails
type EmailHandlerFunc func(ctx context.Context, data io.Reader) error
