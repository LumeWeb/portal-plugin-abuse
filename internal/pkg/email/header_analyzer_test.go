package email

import (
	"go.lumeweb.com/portal/core"
	"net/mail"
	"strings"
	"testing"

	"github.com/mnako/letters"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestDefaultHeaderAnalyzer_Analyze(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))

	analyzer := NewHeaderAnalyzer(core.NewLogger(nil, logger))

	testCases := []struct {
		name            string
		email           *letters.Email
		expectedIssues  bool
		expectedNoReply bool
		description     string
	}{
		{
			name: "Valid Email",
			email: &letters.Email{
				Headers: letters.Headers{
					From: []*mail.Address{{Address: "test@example.com"}},
				},
			},
			expectedIssues:  false,
			expectedNoReply: false,
			description:     "Should pass with a valid email address",
		},
		{
			name: "Invalid Email - Missing @",
			email: &letters.Email{
				Headers: letters.Headers{
					From: []*mail.Address{{Address: "invalid-email"}},
				},
			},
			expectedIssues:  true,
			expectedNoReply: false,
			description:     "Should fail with invalid email missing @",
		},
		{
			name: "Invalid Email - Missing Domain",
			email: &letters.Email{
				Headers: letters.Headers{
					From: []*mail.Address{{Address: "test@"}},
				},
			},
			expectedIssues:  true,
			expectedNoReply: false,
			description:     "Should fail with invalid email missing domain",
		},
		{
			name: "Invalid Email - Missing Local Part",
			email: &letters.Email{
				Headers: letters.Headers{
					From: []*mail.Address{{Address: "@example.com"}},
				},
			},
			expectedIssues:  true,
			expectedNoReply: false,
			description:     "Should fail with invalid email missing local part",
		},
		{
			name: "Invalid Email - Multiple @",
			email: &letters.Email{
				Headers: letters.Headers{
					From: []*mail.Address{{Address: "test@@example.com"}},
				},
			},
			expectedIssues:  true,
			expectedNoReply: false,
			description:     "Should fail with invalid email containing multiple @",
		},
		{
			name: "Missing From Header",
			email: &letters.Email{
				Headers: letters.Headers{},
			},
			expectedIssues:  true,
			expectedNoReply: false,
			description:     "Should fail with missing From header",
		},
		{
			name: "No-Reply Email",
			email: &letters.Email{
				Headers: letters.Headers{
					From: []*mail.Address{{Address: "no-reply@example.com"}},
				},
			},
			expectedIssues:  false,
			expectedNoReply: true,
			description:     "Should detect no-reply email",
		},
		{
			name: "Multiple From Addresses",
			email: &letters.Email{
				Headers: letters.Headers{
					From: []*mail.Address{{Address: "test@example.com"}, {Address: "test2@example.com"}},
				},
			},
			expectedIssues:  false, // This is not checked in the current implementation
			expectedNoReply: false,
			description:     "Should handle multiple From addresses (currently doesn't flag as error)",
		},
		{
			name: "Mixed Case No-Reply Email",
			email: &letters.Email{
				Headers: letters.Headers{
					From: []*mail.Address{{Address: "No-Reply@example.com"}},
				},
			},
			expectedIssues:  false,
			expectedNoReply: true,
			description:     "Should detect no-reply email with mixed case",
		},
		{
			name: "No-Reply Email with Display Name",
			email: &letters.Email{
				Headers: letters.Headers{
					From: []*mail.Address{{Name: "No Reply", Address: "noreply@example.com"}},
				},
			},
			expectedIssues:  false,
			expectedNoReply: true,
			description:     "Should detect no-reply email with display name",
		},
		{
			name: "No-Reply Email with Subdomain",
			email: &letters.Email{
				Headers: letters.Headers{
					From: []*mail.Address{{Address: "no-reply@sub.example.com"}},
				},
			},
			expectedIssues:  false,
			expectedNoReply: true,
			description:     "Should detect no-reply email with subdomain",
		},
		{
			name: "No-Reply Email with Plus Addressing",
			email: &letters.Email{
				Headers: letters.Headers{
					From: []*mail.Address{{Address: "no-reply+extra@example.com"}},
				},
			},
			expectedIssues:  false,
			expectedNoReply: true,
			description:     "Should detect no-reply email with plus addressing",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := analyzer.Analyze(tc.email)

			if tc.expectedIssues {
				assert.Error(t, result.Issues, tc.description)
			} else {
				assert.NoError(t, result.Issues, tc.description)
			}

			assert.Equal(t, tc.expectedNoReply, result.IsNoReplyAddress, tc.description)

			if result.Issues != nil {
				if tc.name == "Missing From Header" {
					assert.ErrorIs(t, result.Issues, ErrMissingFromHeader, "Should return ErrMissingFromHeader")
				}
			}
		})
	}
}

func TestDefaultHeaderAnalyzer_Analyze_NoReplyPatterns(t *testing.T) {
	logger := zaptest.NewLogger(t, zaptest.Level(zap.DebugLevel))
	analyzer := NewHeaderAnalyzer(core.NewLogger(nil, logger))

	for _, pattern := range NoReplyPatterns {
		t.Run("NoReplyPattern_"+pattern, func(t *testing.T) {
			emailAddress := pattern + "example.com"
			email := &letters.Email{
				Headers: letters.Headers{
					From: []*mail.Address{{Address: emailAddress}},
				},
			}

			result, _ := analyzer.Analyze(email)
			assert.True(t, result.IsNoReplyAddress, "Should detect '"+pattern+"' as no-reply pattern")
		})
	}

	t.Run("CaseInsensitive", func(t *testing.T) {
		email := &letters.Email{
			Headers: letters.Headers{
				From: []*mail.Address{{Address: strings.ToUpper("no-reply@example.com")}},
			},
		}
		result, _ := analyzer.Analyze(email)
		assert.True(t, result.IsNoReplyAddress, "Should be case insensitive")
	})
}
