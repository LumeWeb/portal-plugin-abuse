package email

import (
	"errors"
	"github.com/mnako/letters"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
)

var _ Classifier = (*DefaultClassifier)(nil)

// Classifier interface for classifying abuse reports
type Classifier interface {
	ClassifyEmail(email *letters.Email) *ClassificationResult
}

// ClassificationResult contains the classification outcome
type ClassificationResult struct {
	CaseType    models.CaseType
	Priority    models.CasePriority
	Score       ContentScore
	NeedsReview bool
}

// DefaultClassifier is a basic implementation of Classifier
type DefaultClassifier struct {
	logger             *core.Logger
	contentScorer      ContentScorer
	headerAnalyzer     HeaderAnalyzer
	priorityDeterminer PriorityDeterminer
	reviewDecider      ReviewDecider
}

// Classify classifies an abuse report based on its content, headers, and attachments
func (c *DefaultClassifier) ClassifyEmail(email *letters.Email) *ClassificationResult {
	// Extract text content for content scoring
	text := email.Text
	if text == "" {
		text = email.HTML
	}

	// Score the content
	scores := c.contentScorer.Score(text)

	// Analyze headers
	headerResult, err := c.headerAnalyzer.Analyze(email)
	if err != nil {
		c.logger.Warn("Header analysis failed", zap.Error(err))
	}

	// Determine priority
	priority := c.priorityDeterminer.DeterminePriority(scores.ContentScores)

	// Determine if review is needed
	needsReview := c.reviewDecider.NeedsReview(determineHighestCaseType(scores.ContentScores), priority, headerResult.IsNoReplyAddress)

	// Create the classification result
	result := &ClassificationResult{
		CaseType:    determineHighestCaseType(scores.ContentScores),
		Priority:    priority,
		Score:       scores.ContentScores,
		NeedsReview: needsReview,
	}

	c.logger.Debug("Classification result",
		zap.String("case_type", string(result.CaseType)),
		zap.String("priority", string(result.Priority)),
		zap.Bool("needs_review", result.NeedsReview),
	)

	return result
}

// Helper function to determine the highest scoring case type
func determineHighestCaseType(scores ContentScore) models.CaseType {
	highestType := models.CaseTypeOther
	highestScore := 0

	for caseType, score := range scores {
		if score > highestScore || (score == highestScore && models.CaseTypePriority[caseType] > models.CaseTypePriority[highestType]) {
			highestType = caseType
			highestScore = score
		}
	}

	return highestType
}

// NewClassifier creates a new classifier with default dependencies
func NewClassifier(
	logger *core.Logger,
	contentScorer ContentScorer,
	headerAnalyzer HeaderAnalyzer,
	priorityDeterminer PriorityDeterminer,
	reviewDecider ReviewDecider,
) (*DefaultClassifier, error) {
	if logger == nil {
		return nil, errors.New("logger cannot be nil")
	}
	if contentScorer == nil {
		return nil, errors.New("contentScorer cannot be nil")
	}
	if headerAnalyzer == nil {
		return nil, errors.New("headerAnalyzer cannot be nil")
	}
	if priorityDeterminer == nil {
		return nil, errors.New("priorityDeterminer cannot be nil")
	}
	if reviewDecider == nil {
		return nil, errors.New("reviewDecider cannot be nil")
	}

	return &DefaultClassifier{
		logger:             logger,
		contentScorer:      contentScorer,
		headerAnalyzer:     headerAnalyzer,
		priorityDeterminer: priorityDeterminer,
		reviewDecider:      reviewDecider,
	}, nil
}
