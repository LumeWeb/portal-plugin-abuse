package models

import (
	"gorm.io/gorm"
)

type ProcessedEmail struct {
	gorm.Model
	MessageID string // Message-ID (if available)
	Hash      []byte // SHA256 hash of the raw email (fallback)
	Error     bool
}

func (ProcessedEmail) TableName() string {
	return "abuse_processed_emails"
}
