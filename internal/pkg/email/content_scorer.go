package email

import (
	"go.lumeweb.com/portal/core"
	"regexp"
	"strings"
	"sync"

	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.uber.org/zap"
)

type SeverityScoreType string

const (
	ScoreTypeUrgency   SeverityScoreType = "urgency"
	ScoreTypeSensitive SeverityScoreType = "sensitive"
	ScoreTypeThreat    SeverityScoreType = "threat"
)

type ContentScore = map[models.CaseType]int
type SeverityScore = map[SeverityScoreType]int

// ScoringResults contains all scoring results
type ScoringResults struct {
	ContentScores  ContentScore
	SeverityScores SeverityScore
}

// ContentScorerFunc is a type alias for scoring functions
// ContentScorer interface for scoring content against abuse categories
type ContentScorer interface {
	Score(text string) ScoringResults
}

var _ ContentScorer = (*DictionaryContentScorer)(nil)

// DictionaryContentScorer is a basic implementation of ContentScorer using dictionaries
type DictionaryContentScorer struct {
	logger     *core.Logger
	terms      map[models.CaseType]map[string]int // Map of CaseType to term dictionaries
	regexCache map[string]*regexp.Regexp
	cacheMutex sync.RWMutex // Protects regexCache access
}

// NewDictionaryContentScorer creates a new DictionaryContentScorer
func NewDictionaryContentScorer(logger *core.Logger) *DictionaryContentScorer {
	// Initialize the term dictionaries
	terms := map[models.CaseType]map[string]int{
		models.CaseTypeIllegalOrHarmfulContent: IllegalOrHarmfulTerms,
		models.CaseTypePhishing:                PhishingTerms,
		models.CaseTypeCopyrightViolation:      CopyrightTerms,
		models.CaseTypeResourceAbuse:           ResourceAbuseTerms,
		models.CaseTypeHarassment:              HarassmentTerms,
		models.CaseTypeSpam:                    SpamTerms,
		models.CaseTypeMalware:                 MalwareTerms,
	}

	return &DictionaryContentScorer{
		logger:     logger,
		terms:      terms,
		regexCache: make(map[string]*regexp.Regexp),
	}
}

// Score scores the given text and attachments and returns all scoring results
func (s *DictionaryContentScorer) Score(text string) ScoringResults {
	text = strings.ToLower(text)
	contentScores := make(map[models.CaseType]int)

	// Pre-process text by removing exclusion patterns
	for _, rule := range ExclusionRules {
		if regex := s.getRegex(rule.Pattern); regex != nil {
			text = regex.ReplaceAllString(text, "")
		}
	}

	// Score all categories
	for caseType, termDict := range s.terms {
		contentScores[caseType] = scoreDictionary(s.logger, text, termDict, string(caseType))
	}

	// Calculate severity scores from content scores
	scores := map[SeverityScoreType]int{
		ScoreTypeUrgency:   contentScores[models.CaseTypeSpam],
		ScoreTypeSensitive: contentScores[models.CaseTypeHarassment],
		ScoreTypeThreat:    contentScores[models.CaseTypeMalware],
	}

	return ScoringResults{
		ContentScores:  contentScores,
		SeverityScores: scores,
	}
}

func scoreDictionary(logger *core.Logger, text string, terms map[string]int, scoreType string) int {
	// Limit text size to prevent performance issues
	const maxTextSize = 100000 // 100KB
	if len(text) > maxTextSize {
		logger.Warn("Text too large for scoring, truncating",
			zap.String("scoreType", scoreType),
			zap.Int("originalSize", len(text)))
		text = text[:maxTextSize]
	}

	score := 0
	text = strings.ToLower(text)
	words := strings.Fields(text)

	for _, word := range words {
		cleanWord := strings.Trim(word, ".,/#!%^&*;:{}=-_`()?\n")
		if cleanWord == "" {
			continue
		}

		// Skip if word is a stop word
		if isStopWord(cleanWord) {
			logger.Debug("Skipping stop word", zap.String("word", cleanWord))
			continue
		}

		for term, weight := range terms {
			term = strings.ToLower(term)
			if cleanWord == term {
				logger.Debug("Matched "+scoreType+" term",
					zap.String("term", term),
					zap.Int("weight", weight))
				score += weight
			}
		}
	}
	return score
}
func isStopWord(word string) bool {
	if _, ok := StopWordsMap[word]; ok {
		return true
	}

	return false
}

// getRegex gets or creates a compiled regex pattern from the cache
func (s *DictionaryContentScorer) getRegex(pattern string) *regexp.Regexp {
	// First try with read lock
	s.cacheMutex.RLock()
	regex, ok := s.regexCache[pattern]
	s.cacheMutex.RUnlock()

	if ok {
		return regex
	}

	// Not found, acquire write lock
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// Double check in case another goroutine added it while we waited for lock
	if regex, ok := s.regexCache[pattern]; ok {
		return regex
	}

	// Compile new pattern and add to cache
	regex, err := regexp.Compile(pattern)
	if err != nil {
		s.logger.Error("Failed to compile regex pattern", zap.String("pattern", pattern), zap.Error(err))
		return nil
	}

	s.regexCache[pattern] = regex
	return regex
}
