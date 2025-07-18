package models

import "time"

type ProcessedEmail struct {
	MessageID   string // Message-ID (if available)
	Hash        []byte // SHA256 hash of the raw email (fallback)
	ProcessedAt time.Time
	Error       bool
}

func (ProcessedEmail) TableName() string {
	return "abuse_processed_emails"
}
