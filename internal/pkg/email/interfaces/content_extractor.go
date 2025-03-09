package interfaces

import (
	"github.com/mnako/letters"
)

// ContentExtractor defines the interface for extracting content from emails
type ContentExtractor interface {
	// ExtractURLs extracts URLs from an email
	ExtractURLs(email *letters.Email) []string

	// ExtractHashes extracts content hashes from an email
	ExtractHashes(email *letters.Email) []string
}
