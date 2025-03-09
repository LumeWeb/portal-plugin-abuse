package service

import (
	"context"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"time"
)

// CaseService defines the interface for case management operations
type CommAnalytics struct {
	AvgResponseTime  time.Duration
	MaxResponseTime  time.Duration
	CommsPerCase     map[uint]int64  // Number of communications per case
}

type EvidenceAnalytics struct {
	FilesPerCase    map[uint]int64
	AvgFilesPerCase float64
}

type BlocklistAnalytics struct {
	TotalBlocks      int64
	BlocksByReason   map[models.BlockReason]int64
	BlocksBySeverity map[models.BlockSeverity]int64
}

type CaseAnalytics struct {
	TotalCases            int64
	OpenCases             int64 
	NewCasesInRange       int64
	StatusBreakdown       map[models.CaseStatus]int64
	CaseTypeBreakdown     map[models.CaseType]int64
	SourceBreakdown       map[models.ReportSource]int64
	NeedsReviewCount      int64
	CommsMetrics          CommAnalytics
	EvidenceMetrics       EvidenceAnalytics
	BlocklistMetrics      BlocklistAnalytics
	ResolutionTrends      map[time.Time]int64
	TotalResolved         int64
	TotalResolutionSeconds float64
	AvgResolutionSeconds  float64
	StatusDurations       map[models.CaseStatus]time.Duration
	AvgStatusDurations    map[models.CaseStatus]time.Duration
}

type CaseService interface {
	core.Service

	// Create creates a new case
	Create(caseData *models.Case) (*models.Case, error)

	// GetByID retrieves a case by its ID
	GetByID(id uint) (*models.Case, error)

	// GetAnalytics returns aggregated case metrics with filters
	GetAnalytics(filters []queryutil.Filter) (*CaseAnalytics, error)

	// Get7DayAnalytics returns analytics for the last 7 days
	Get7DayAnalytics() (*CaseAnalytics, error)

	// Get30DayAnalytics returns analytics for the last 30 days
	Get30DayAnalytics() (*CaseAnalytics, error)

	// List returns a list of cases with filtering, sorting and pagination
	List(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Case, int64, error)

	// Update updates an existing case
	Update(caseModel *models.Case) error

	// UpdateStatus updates the status of a case
	UpdateStatus(id uint, status models.CaseStatus) error

	// GetCaseByReference retrieves a case by its reference number
	GetCaseByReference(reference string) (*models.Case, error)

	// GetPublicCase gets a case for public access
	GetPublicCase(reference string, reporterID uint) (*models.Case, error)

	// Search performs a more advanced search on cases
	Search(ctx context.Context, query string, filters []queryutil.Filter, pagination queryutil.Pagination) ([]models.Case, int64, error)

	// LinkSubject associates a subject with a case
	LinkSubject(caseID, subjectID uint) error

	// SendCreationNotification sends an email notification when a case is created
	SendCreationNotification(caseID uint) error

	// SendStatusUpdateNotification sends an email notification when a case status is updated
	SendStatusUpdateNotification(caseID uint, oldStatus, newStatus models.CaseStatus) error
}
