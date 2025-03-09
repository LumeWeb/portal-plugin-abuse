package email

import (
	"crypto/tls"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"go.lumeweb.com/portal-plugin-abuse/internal/pkg/email/interfaces"
)

// IMAPClientConn is an alias from the interfaces package
type IMAPClientConn = interfaces.IMAPClientConn

// IMAPDialer is an alias from the interfaces package
type IMAPDialer = interfaces.IMAPDialer

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

// Package level variable for our IMAP dialer, which can be replaced in tests
var DefaultDialer IMAPDialer = &DefaultIMAPDialer{}
