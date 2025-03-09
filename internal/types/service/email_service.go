package service

import (
	"context"
	"io"

	"go.lumeweb.com/portal/core"
)

// EmailService handles all email-related operations
type EmailService interface {
	core.Service

	// SendTemplatedEmail sends an email using a registered template to all recipients
	SendTemplatedEmail(to []string, templateName string, data core.MailerTemplateData) error

	// ProcessIncomingEmail processes an incoming email and adds it to a case
	ProcessIncomingEmail(ctx context.Context, rawEmail io.Reader) error

	// GenerateCaseThreadID generates a unique thread ID for a case
	GenerateCaseThreadID(caseID uint, referenceNumber string) string
}
