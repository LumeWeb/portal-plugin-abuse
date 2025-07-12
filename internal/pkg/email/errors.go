package email

import "errors"

// Error variables for email processing
var (
	// Header-related errors
	ErrMissingFromHeader    = errors.New("missing From header")
	ErrInvalidEmailFormat   = errors.New("invalid email address format")
)
