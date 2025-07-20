package internal

import (
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
)

const PLUGIN_NAME = "abuse"

// CaseTypeToBlockReason maps case types to appropriate block reasons
func CaseTypeToBlockReason(caseType models.CaseType) models.BlockReason {
	switch caseType {
	case models.CaseTypeSpam:
		return models.BlockReasonSpam
	case models.CaseTypeHarassment:
		return models.BlockReasonHarassment
	case models.CaseTypeIllegalOrHarmfulContent:
		return models.BlockReasonSystemPolicy // TODO: Define more specific illegal content categories
	case models.CaseTypeMalware:
		return models.BlockReasonMalware
	case models.CaseTypePhishing:
		return models.BlockReasonMalware
	case models.CaseTypeCopyrightViolation:
		return models.BlockReasonCopyright
	case models.CaseTypeResourceAbuse:
		return models.BlockReasonSystemPolicy
	case models.CaseTypeOther:
		return models.BlockReasonSystemPolicy
	default:
		return models.BlockReasonSystemPolicy
	}
}

// CasePriorityToBlockSeverity maps case priorities to block severity levels
func CasePriorityToBlockSeverity(priority models.CasePriority) models.BlockSeverity {
	switch priority {
	case models.CasePriorityCritical:
		return models.BlockSeverityCritical
	case models.CasePriorityHigh:
		return models.BlockSeverityHigh
	case models.CasePriorityMedium:
		return models.BlockSeverityMedium
	case models.CasePriorityLow:
		return models.BlockSeverityLow
	default:
		return models.BlockSeverityMedium
	}
}
