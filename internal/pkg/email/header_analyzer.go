package email

import (
	"errors"
	"go.lumeweb.com/portal/core"
	mail "net/mail"
	"strings"

	"github.com/mnako/letters"
	"go.uber.org/zap"
)

// HeaderAnalyzer interface for analyzing email headers
type HeaderAnalyzer interface {
	Analyze(email *letters.Email) (HeaderAnalysisResult, error)
}

// HeaderAnalysisResult contains the results of header analysis
type HeaderAnalysisResult struct {
	Issues           error
	IsNoReplyAddress bool
}

// DefaultHeaderAnalyzer is a basic implementation of HeaderAnalyzer
type DefaultHeaderAnalyzer struct {
	logger *core.Logger
}

// NewHeaderAnalyzer creates a new DefaultHeaderAnalyzer
func NewHeaderAnalyzer(logger *core.Logger) *DefaultHeaderAnalyzer {
	return &DefaultHeaderAnalyzer{
		logger: logger,
	}
}

// Analyze examines email headers for potential issues
func (c *DefaultHeaderAnalyzer) Analyze(email *letters.Email) (HeaderAnalysisResult, error) {
	var errs []error
	isNoReplyAddress := false

	fromAddresses := email.Headers.From
	if len(fromAddresses) == 0 {
		errs = append(errs, ErrMissingFromHeader)
		c.logger.Debug("Missing From header - suspicious")
	}

	if len(fromAddresses) > 0 {
		primaryFrom := fromAddresses[0]
		if primaryFrom != nil {
			addr := primaryFrom.Address
			if _, err := mail.ParseAddress(addr); err != nil {
				c.logger.Debug("Invalid email address format", zap.String("address", addr), zap.Error(err))
				return HeaderAnalysisResult{
					Issues:           errors.Join(append(errs, ErrInvalidEmailFormat)...),
					IsNoReplyAddress: false,
				}, nil
			}
			fromAddress := strings.ToLower(addr)
			parts := strings.Split(fromAddress, "@")
			localPart := parts[0]
			// Handle plus addressing by taking the part before "+"
			baseLocalPart := strings.Split(localPart, "+")[0]
			domainPart := parts[1]
			if strings.Contains(localPart, "+") {
				c.logger.Debug("Processing plus-addressed email",
					zap.String("full_address", fromAddress),
					zap.String("local_part", localPart),
					zap.String("base_local_part", baseLocalPart))
			}
			for _, pattern := range NoReplyPatterns {
				// Split pattern into local and domain parts if it contains @
				if strings.Contains(pattern, "@") {
					patternParts := strings.Split(pattern, "@")
					if len(patternParts) != 2 {
						continue
					}

					// If pattern specifies a domain (not empty after @), it must match exactly
					if patternParts[1] != "" && patternParts[1] != domainPart {
						continue
					}

					// Check if local part matches pattern (with or without plus addressing)
					if strings.HasPrefix(localPart, patternParts[0]) || strings.HasPrefix(baseLocalPart, patternParts[0]) {
						isNoReplyAddress = true
						c.logger.Debug("Detected no-reply address pattern",
							zap.String("pattern", pattern),
							zap.String("address", fromAddress))
						break
					}
				} else {
					// For patterns without @, check against both local part variants
					if strings.HasPrefix(baseLocalPart, pattern) || strings.HasPrefix(localPart, pattern) {
						isNoReplyAddress = true
						c.logger.Debug("Detected no-reply address pattern (local part match)",
							zap.String("pattern", pattern),
							zap.String("address", fromAddress),
							zap.String("base_local_part", baseLocalPart),
							zap.String("local_part", localPart))
						break
					}
				}
			}
		}
	}

	return HeaderAnalysisResult{
		Issues:           errors.Join(errs...),
		IsNoReplyAddress: isNoReplyAddress,
	}, nil
}
