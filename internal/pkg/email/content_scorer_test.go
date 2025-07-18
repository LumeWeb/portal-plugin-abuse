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
		{
			name: "Reliable Site",
			text: "Hello,\nWe have received an abuse complaint for the IP address\n185.150.190.66. Please see the details of the complaint below. All\nabuse complaints must be addressed and fully resolved within 24\nhours. Once an abuse complaint has been resolved, we require an\nupdate through our support ticketing system at\nhttp://support.reliablesite.net/Main/frmTicket.aspx?ticketnumber=27A-2AEF5A4C-0017&email=takedown-response%2b36676557%40netcraft.com&h=FC1F2C8422E55B42217AA90A3FEACB26\n- please do not reply by e-mail. If there is no resolution within\n24 hours, the IP in question will be null routed.\n=======================================================\nHello,\nWe have discovered a phishing attack located on your network:\nhxxp://web3portal[.]com/3AGolXJz5LVgHDPJmU4GGMDBuOSRPKK8QxVRB-TpCfzj4g\n[185.150.190.66]\nhxxps://web3portal[.]com/3AGolXJz5LVgHDPJmU4GGMDBuOSRPKK8QxVRB-TpCfzj4g\n[185.150.190.66]\nThis attack targets our customer, Microsoft, website URL\nhttps://www.microsoft.com/.\nWould it be possible to have the fraudulent content, and any\nother associated fraudulent content, taken down as soon as you\nare able to?\nAdditionally, please keep the fraudulent content safe so that\nour customer and law enforcement agencies can investigate this\nincident further once the site is offline.\nMore information about the detected issue is provided at\nhttps://incident.netcraft.com/bf29544e4a0b/\nMany thanks,\nNetcraft\nPhone: +44(0)1225 447500\nFax: +44(0)1225 448600\nNetcraft Issue Number: 37040580\nTo contact us about updates regarding this attack, please\nrespond to this email. Please note: replies to this address will\nbe logged, but aren't always read. If you believe you have\nreceived this email in error, or you require further support,\nplease contact: takedown@netcraft.com.\nThis mail can be parsed with x-arf tools. Visit\nhttp://www.xarf.org/ for more information about x-arf.\n=======================================================\n\nAbuse Department\nReliableSite.Net LLC\n",
			expected: map[models.CaseType]int{
				models.CaseTypeIllegalOrHarmfulContent: 0,
				models.CaseTypePhishing:                10,
				models.CaseTypeCopyrightViolation:      0,
				models.CaseTypeResourceAbuse:           20,
				models.CaseTypeHarassment:              0,
				models.CaseTypeSpam:                    30,
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

func TestFindDuplicateTerms(t *testing.T) {
	// Aggregate all term maps into a single slice for easier iteration.
	termMaps := map[string]map[string]int{
		"SpamTerms":             SpamTerms,
		"HarassmentTerms":       HarassmentTerms,
		"IllegalOrHarmfulTerms": IllegalOrHarmfulTerms,
		"PhishingTerms":         PhishingTerms,
		"CopyrightTerms":        CopyrightTerms,
		"ResourceAbuseTerms":    ResourceAbuseTerms,
		"MalwareTerms":          MalwareTerms,
		"UrgencyPatterns":       UrgencyPatterns,
		"SensitivePatterns":     SensitivePatterns,
		"ThreatPatterns":        ThreatPatterns,
	}

	termData := make(map[string][]string)

	// Iterate through each term map and populate termData.
	for mapName, termMap := range termMaps {
		for term := range termMap {
			term = strings.ToLower(term) // Normalize to lowercase for comparison
			if _, ok := termData[term]; ok {
				termData[term] = append(termData[term], mapName)
			} else {
				termData[term] = []string{mapName}
			}
		}
	}

	// Identify duplicate terms.
	duplicates := make(map[string][]string)
	for term, dicts := range termData {
		if len(dicts) > 1 {
			duplicates[term] = dicts
		}
	}

	// Assert that there are no duplicates (or handle them as needed).
	if len(duplicates) > 0 {
		for term, dicts := range duplicates {
			t.Logf("Duplicate term: \"%s\" found in dictionaries: %v\n", term, dicts)
		}
		assert.Fail(t, "Duplicate terms found in term dictionaries")
	}
}
