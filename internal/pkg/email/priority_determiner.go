package email

import (
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
)

var _ PriorityDeterminer = (*DefaultPriorityDeterminer)(nil)

// PriorityDeterminer interface for determining the priority of a case
type PriorityDeterminer interface {
	DeterminePriority(contentScores ContentScore) models.CasePriority
}

// DefaultPriorityDeterminer is a basic implementation of PriorityDeterminer
type DefaultPriorityDeterminer struct {
	PriorityScoreLow    int
	PriorityScoreMedium int
	PriorityScoreHigh   int
}

// NewPriorityDeterminer creates a new DefaultPriorityDeterminer with default thresholds
func NewPriorityDeterminer() *DefaultPriorityDeterminer {
	return &DefaultPriorityDeterminer{
		PriorityScoreLow:    5,
		PriorityScoreMedium: 10,
		PriorityScoreHigh:   15,
	}
}

// DeterminePriority determines the priority level based on the content scores and attachment score
func (d *DefaultPriorityDeterminer) DeterminePriority(contentScores map[models.CaseType]int) models.CasePriority {
	// Calculate a combined score based on content and attachments
	combinedScore := 0
	for _, score := range contentScores {
		combinedScore += score
	}

	// Determine priority based on thresholds
	if combinedScore >= d.PriorityScoreHigh {
		return models.CasePriorityHigh
	} else if combinedScore >= d.PriorityScoreMedium {
		return models.CasePriorityMedium
	} else if combinedScore > d.PriorityScoreLow {
		return models.CasePriorityLow
	}

	return models.CasePriorityLow // Default to low priority
}
