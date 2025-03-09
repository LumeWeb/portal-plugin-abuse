package service

import (
	"context"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"

	"go.lumeweb.com/portal/core"
)

// AbuseReportService defines the interface for the abuse report service
type AbuseReportService interface {
	core.Service

	// SubmitReport submits a new abuse report
	SubmitReport(ctx context.Context, caseData *models.Case) (*models.Case, error)

	// GetReportStatus retrieves the status of a report by confirmation number
	GetReportStatus(ctx context.Context, confirmationNumber string) (*models.Case, error)

	// MapAbuseCategoryToCaseType maps the abuse category to internal case type
	MapAbuseCategoryToCaseType(category AbuseCategory) string
}
