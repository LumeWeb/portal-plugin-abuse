package service

import (
	"go.lumeweb.com/portal/core"
	"time"
)

// TokenService defines the interface for token generation and validation
type TokenService interface {
	core.Service

	// GenerateToken creates a new access token for a reporter
	GenerateToken(caseID, reporterID uint, validDays int) (string, time.Time, error)

	// ValidateToken checks if a token is valid
	ValidateToken(token string) (caseID, reporterID uint, valid bool)

	// GetTokenParts returns the parts of a token for testing
	GetTokenParts(token string) []string
}
