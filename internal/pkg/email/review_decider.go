package email

import "go.lumeweb.com/portal-plugin-abuse/internal/db/models"

// ReviewDecider interface for deciding whether a case needs manual review
type ReviewDecider interface {
	NeedsReview(caseType models.CaseType, priority models.CasePriority, isNoReplyAddress bool) bool
}

// DefaultReviewDecider is a basic implementation of ReviewDecider
type DefaultReviewDecider struct {
	ReviewHighPriority     bool
	ReviewContentViolation bool
	ReviewNoReplyAddress   bool
}

// NewReviewDecider creates a new DefaultReviewDecider with default settings
func NewReviewDecider() *DefaultReviewDecider {
	return &DefaultReviewDecider{
		ReviewHighPriority:     true,
		ReviewContentViolation: true,
		ReviewNoReplyAddress:   false, // Disabled by default
	}
}

// NeedsReview determines if a case needs manual review based on the case type and priority
func (d *DefaultReviewDecider) NeedsReview(caseType models.CaseType, priority models.CasePriority, isNoReplyAddress bool) bool {
	if d.ReviewHighPriority && (priority == models.CasePriorityHigh || priority == models.CasePriorityCritical) {
		return true
	}

	if d.ReviewContentViolation && (caseType == models.CaseTypeIllegalOrHarmfulContent ||
		caseType == models.CaseTypeCopyrightViolation ||
		caseType == models.CaseTypePhishing ||
		caseType == models.CaseTypeResourceAbuse ||
		caseType == models.CaseTypeMalware ||
		caseType == models.CaseTypeHarassment) {
		return true
	}

	if d.ReviewNoReplyAddress && isNoReplyAddress {
		return true
	}

	return false
}
