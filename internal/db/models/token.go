package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Token represents an access token for reporters to submit evidence or communicate about a case
type Token struct {
	gorm.Model
	CaseID     uint
	ReporterID uint
	Token      []byte
	ExpiresAt  *time.Time `gorm:"index"`
	RevokedAt  *time.Time `gorm:"index"`
	LastUsedAt *time.Time `gorm:"index"`
}

// TableName specifies the database table name
func (Token) TableName() string {
	return "abuse_tokens"
}

// Validate checks if the token is valid
func (t *Token) Validate() error {
	if t.RevokedAt != nil && !t.RevokedAt.IsZero() {
		return fmt.Errorf("token revoked")
	}

	if t.ExpiresAt != nil && t.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("token expired")
	}

	if len(t.Token) == 0 {
		return fmt.Errorf("token value cannot be empty")
	}

	return nil
}

// BeforeCreate sets default values
func (t *Token) BeforeCreate(tx *gorm.DB) error {
	if len(t.Token) == 0 {
		return fmt.Errorf("token value must be generated before creation")
	}
	return nil
}
