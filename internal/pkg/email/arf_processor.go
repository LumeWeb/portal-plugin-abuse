package email

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"

	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
)

// ARFProcessor processes Abuse Reporting Format emails as defined in RFC 5965
// https://www.rfc-editor.org/rfc/rfc5965.txt
type ARFProcessor struct {
	ctx              core.Context
	logger           *core.Logger
	contentExtractor ContentExtractor
}

// ARFReport represents a parsed ARF report
type ARFReport struct {
	// Feedback type (RFC 5965 Section 3.2)
	FeedbackType string

	// User-Agent of the feedback generating party (RFC 5965 Section 3.2)
	UserAgent string

	// Arrival date of the original message (RFC 5965 Section 3.2)
	ArrivalDate string

	// Source IP address of the message (RFC 5965 Section 3.2)
	SourceIP string

	// Reporter's address (RFC 5965 Section 3.4)
	ReporterEmail string

	// Original message headers
	OriginalMessageHeaders map[string]string

	// Original message subject
	OriginalSubject string

	// Original message recipient
	OriginalRecipient string

	// Original message from address
	OriginalFrom string

	// Feedback raw text
	FeedbackText string

	// Machine readable part
	MachineReadable map[string]string
}

// NewARFProcessor creates a new ARF processor
func NewARFProcessor(ctx core.Context, contentExtractor ContentExtractor) *ARFProcessor {
	return &ARFProcessor{
		ctx:              ctx,
		logger:           ctx.NamedLogger("arf-processor"),
		contentExtractor: contentExtractor,
	}
}

// IsARF determines if an email is in ARF format
// ARF emails are multipart/report emails with report-type=feedback-report as specified in RFC 5965
func (p *ARFProcessor) IsARF(rawEmail io.Reader) (bool, *bytes.Buffer) {
	// Create a buffer that preserves the original email content
	var buf bytes.Buffer
	teeReader := io.TeeReader(rawEmail, &buf)

	// Parse just enough to check headers
	msg, err := mail.ReadMessage(teeReader)
	if err != nil {
		return false, &buf
	}

	// Check content type
	contentType := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false, &buf
	}

	// Read remaining data into buffer to ensure complete capture
	io.Copy(io.Discard, teeReader) // Ensure entire message is read into buffer

	return strings.HasPrefix(mediaType, "multipart/report") &&
		params["report-type"] == "feedback-report", &buf
}

// Process processes an ARF formatted email and returns the parsed report
func (p *ARFProcessor) Process(ctx context.Context, rawEmail io.Reader) (*ARFReport, error) {
	// Check if this is an ARF email and get the buffered content
	isARF, buf := p.IsARF(rawEmail)
	if !isARF {
		return nil, fmt.Errorf("not an ARF email")
	}

	// Parse the ARF report
	arfReport, err := p.ParseARF(buf)
	if err != nil {
		p.logger.Error("Failed to parse ARF report", zap.Error(err))
		return nil, fmt.Errorf("failed to parse ARF report: %w", err)
	}

	return arfReport, nil
}

// ParseARF parses an ARF formatted email
func (p *ARFProcessor) ParseARF(rawEmail io.Reader) (*ARFReport, error) {
	// Parse the email message
	msg, err := mail.ReadMessage(rawEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to parse email: %w", err)
	}

	// Initialize ARF report
	report := &ARFReport{
		OriginalMessageHeaders: make(map[string]string),
		MachineReadable:        make(map[string]string),
	}

	// Set reporter email from the From header
	from := msg.Header.Get("From")
	if addr, err := mail.ParseAddress(from); err == nil {
		report.ReporterEmail = addr.Address
	}

	// Parse media type and boundary
	contentType := msg.Header.Get("Content-Type")
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("failed to parse content type: %w", err)
	}

	boundary, ok := params["boundary"]
	if !ok {
		return nil, fmt.Errorf("no boundary found in multipart message")
	}

	// Create multipart reader
	mr := multipart.NewReader(msg.Body, boundary)

	// Parse each part according to RFC 5965 Section 3.1
	// ARF has 3 parts:
	// 1. Human readable report
	// 2. Machine readable feedback report
	// 3. Original message or message portion

	// Part 1: Human readable report
	part1, err := mr.NextPart()
	if err != nil {
		return nil, fmt.Errorf("failed to get human readable part: %w", err)
	}

	humanReadable, err := io.ReadAll(part1)
	if err != nil {
		return nil, fmt.Errorf("failed to read human readable part: %w", err)
	}
	report.FeedbackText = string(humanReadable)

	// Part 2: Machine readable feedback report
	part2, err := mr.NextPart()
	if err != nil {
		return nil, fmt.Errorf("failed to get machine readable part: %w", err)
	}

	machineReadable, err := io.ReadAll(part2)
	if err != nil {
		return nil, fmt.Errorf("failed to read machine readable part: %w", err)
	}

	// Parse machine readable fields following the header format
	p.parseMachineReadable(report, string(machineReadable))

	// Part 3: Original message or message portion
	part3, err := mr.NextPart()
	if err != nil {
		// Third part is optional in some implementations
		p.logger.Warn("Failed to get original message part, continuing anyway", zap.Error(err))
	} else {
		// Try to parse the original message
		// Debug: Read the content of part3 first
		originalContent, _ := io.ReadAll(part3)
		originalContentStr := string(originalContent)
		p.logger.Debug("Original message content", zap.String("content", originalContentStr))

		// Try to extract the subject directly from the content
		if originalContentStr != "" {
			lines := strings.Split(originalContentStr, "\n")
			for _, line := range lines {
				if strings.HasPrefix(strings.ToLower(line), "subject:") {
					subjectParts := strings.SplitN(line, ":", 2)
					if len(subjectParts) == 2 {
						report.OriginalSubject = strings.TrimSpace(subjectParts[1])
						break
					}
				}
			}
		}

		// Reset the reader with a new reader from the content
		part3Reader := bytes.NewReader(originalContent)
		originalMsg, err := mail.ReadMessage(part3Reader)
		if err != nil {
			p.logger.Warn("Failed to parse original message", zap.Error(err))
		} else {
			// Extract headers from original message
			for headerName, headerValues := range originalMsg.Header {
				if len(headerValues) > 0 {
					report.OriginalMessageHeaders[headerName] = headerValues[0]
				}
			}

			// Set specific fields from the original message - overriding any we found in our manual parsing
			if subject := originalMsg.Header.Get("Subject"); subject != "" {
				report.OriginalSubject = subject
			}

			// Get From address
			if from := originalMsg.Header.Get("From"); from != "" {
				if addr, err := mail.ParseAddress(from); err == nil {
					report.OriginalFrom = addr.Address
				} else {
					report.OriginalFrom = from
				}
			}

			// Get To address
			if to := originalMsg.Header.Get("To"); to != "" {
				if addr, err := mail.ParseAddress(to); err == nil {
					report.OriginalRecipient = addr.Address
				} else {
					report.OriginalRecipient = to
				}
			}
		}
	}

	return report, nil
}

// parseMachineReadable parses the machine readable feedback report
// Format is described in RFC 5965 Section 3.2
func (p *ARFProcessor) parseMachineReadable(report *ARFReport, content string) {
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split at the first colon
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Store all fields in the machine readable map
		report.MachineReadable[key] = value

		// Set specific fields from the machine readable part
		switch strings.ToLower(key) {
		case "feedback-type":
			report.FeedbackType = value
		case "user-agent":
			report.UserAgent = value
		case "arrival-date":
			report.ArrivalDate = value
		case "received-date":
			// Historic field, treat as Arrival-Date per RFC 5965
			if report.ArrivalDate == "" {
				report.ArrivalDate = value
			}
		case "source-ip":
			report.SourceIP = value
		}
	}
}
