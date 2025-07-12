package email

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap/zaptest"
)

func TestDictionaryContentScorer_Score(t *testing.T) {
	logger := zaptest.NewLogger(t)
	scorer := NewDictionaryContentScorer(core.NewLogger(nil, logger))

	testCases := []struct {
		name     string
		text     string
		expected map[models.CaseType]int
	}{
		{
			name: "Empty text",
			text: "",
			expected: map[models.CaseType]int{
				models.CaseTypeIllegalOrHarmfulContent: 0,
				models.CaseTypePhishing:                0,
				models.CaseTypeCopyrightViolation:      0,
				models.CaseTypeResourceAbuse:           0,
				models.CaseTypeHarassment:              0,
				models.CaseTypeSpam:                    0,
				models.CaseTypeMalware:                 0,
			},
		},
		{
			name: "Spam text",
			text: "This is spam and unsolicited bulk email.",
			expected: map[models.CaseType]int{
				models.CaseTypeIllegalOrHarmfulContent: 0,
				models.CaseTypePhishing:                0,
				models.CaseTypeCopyrightViolation:      0,
				models.CaseTypeResourceAbuse:           0,
				models.CaseTypeHarassment:              0,
				models.CaseTypeSpam:                    21,
				models.CaseTypeMalware:                 0,
			},
		},
		{
			name: "Harassment text",
			text: "This is harassment and abusive content.",
			expected: map[models.CaseType]int{
				models.CaseTypeIllegalOrHarmfulContent: 0,
				models.CaseTypePhishing:                0,
				models.CaseTypeCopyrightViolation:      0,
				models.CaseTypeResourceAbuse:           0,
				models.CaseTypeHarassment:              14,
				models.CaseTypeSpam:                    0,
				models.CaseTypeMalware:                 0,
			},
		},
		{
			name: "Copyright violation text",
			text: "This is a copyright infringement and unauthorized use of content.",
			expected: map[models.CaseType]int{
				models.CaseTypeIllegalOrHarmfulContent: 0,
				models.CaseTypePhishing:                0,
				models.CaseTypeCopyrightViolation:      12,
				models.CaseTypeResourceAbuse:           7,
				models.CaseTypeHarassment:              0,
				models.CaseTypeSpam:                    0,
				models.CaseTypeMalware:                 0,
			},
		},
		{
			name: "Mixed content",
			text: "This is spam and harassment and copyright infringement.",
			expected: map[models.CaseType]int{
				models.CaseTypeIllegalOrHarmfulContent: 0,
				models.CaseTypePhishing:                0,
				models.CaseTypeCopyrightViolation:      12,
				models.CaseTypeResourceAbuse:           0,
				models.CaseTypeHarassment:              7,
				models.CaseTypeSpam:                    7,
				models.CaseTypeMalware:                 0,
			},
		},
		{
			name: "Case insensitive",
			text: "This is SPAM and Harassment.",
			expected: map[models.CaseType]int{
				models.CaseTypeIllegalOrHarmfulContent: 0,
				models.CaseTypePhishing:                0,
				models.CaseTypeCopyrightViolation:      0,
				models.CaseTypeResourceAbuse:           0,
				models.CaseTypeHarassment:              7,
				models.CaseTypeSpam:                    7,
				models.CaseTypeMalware:                 0,
			},
		},
		{
			name: "Partial word",
			text: "This is spamming.",
			expected: map[models.CaseType]int{
				models.CaseTypeIllegalOrHarmfulContent: 0,
				models.CaseTypePhishing:                0,
				models.CaseTypeCopyrightViolation:      0,
				models.CaseTypeResourceAbuse:           0,
				models.CaseTypeHarassment:              0,
				models.CaseTypeSpam:                    7,
				models.CaseTypeMalware:                 0,
			},
		},
		{
			name: "Multiple lines",
			text: "This is spam.\nThis is harassment.",
			expected: map[models.CaseType]int{
				models.CaseTypeIllegalOrHarmfulContent: 0,
				models.CaseTypePhishing:                0,
				models.CaseTypeCopyrightViolation:      0,
				models.CaseTypeResourceAbuse:           0,
				models.CaseTypeHarassment:              7,
				models.CaseTypeSpam:                    7,
				models.CaseTypeMalware:                 0,
			},
		},
		// Will match despite the text.
		{
			name: "With stop words",
			text: "This is not spam.",
			expected: map[models.CaseType]int{
				models.CaseTypeIllegalOrHarmfulContent: 0,
				models.CaseTypePhishing:                0,
				models.CaseTypeCopyrightViolation:      0,
				models.CaseTypeResourceAbuse:           0,
				models.CaseTypeHarassment:              0,
				models.CaseTypeSpam:                    7,
				models.CaseTypeMalware:                 0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := scorer.Score(tc.text)
			assert.Equal(t, tc.expected, actual.ContentScores)
		})
	}
}

func TestNewDictionaryContentScorer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	scorer := NewDictionaryContentScorer(core.NewLogger(nil, logger))

	assert.NotNil(t, scorer.terms)
	assert.NotNil(t, scorer.regexCache)
	assert.NotNil(t, scorer.logger)

	// Check if the terms are initialized
	assert.NotEmpty(t, scorer.terms[models.CaseTypeSpam])
	assert.NotEmpty(t, scorer.terms[models.CaseTypeHarassment])
	assert.NotEmpty(t, scorer.terms[models.CaseTypeCopyrightViolation])
	assert.NotEmpty(t, scorer.terms[models.CaseTypeResourceAbuse])
	assert.NotEmpty(t, scorer.terms[models.CaseTypePhishing])
	assert.NotEmpty(t, scorer.terms[models.CaseTypeIllegalOrHarmfulContent])
	assert.NotEmpty(t, scorer.terms[models.CaseTypeMalware])
}

func TestDictionaryContentScorer_Score_NoPanic(t *testing.T) {
	logger := zaptest.NewLogger(t)
	scorer := NewDictionaryContentScorer(core.NewLogger(nil, logger))

	// Test with a long string to ensure no panic occurs
	longString := strings.Repeat("a", 10000)
	assert.NotPanics(t, func() {
		scorer.Score(longString)
	})
}
