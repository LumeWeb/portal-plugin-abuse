package service

import (
	"context"
	"errors"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"time"
)

// CaseService defines the interface for case management operations
type CommAnalytics struct {
	AvgResponseTime time.Duration
	MaxResponseTime time.Duration
	CommsPerCase    map[uint]int64 // Number of communications per case
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

type StatusFlowGraph struct {
	Nodes []StatusFlowNode `json:"nodes"`
	Links []StatusFlowLink `json:"links"`
}

type StatusFlowNode struct {
	Name string `json:"name"`
}

type StatusFlowLink struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Value  int64  `json:"value"`
}

type CaseAnalytics struct {
	TotalCases             int64
	OpenCases              int64
	NewCasesInRange        int64
	StatusBreakdown        map[models.CaseStatus]int64
	CaseTypeBreakdown      map[models.CaseType]int64
	SourceBreakdown        map[models.ReportSource]int64
	NeedsReviewCount       int64
	CommsMetrics           CommAnalytics
	EvidenceMetrics        EvidenceAnalytics
	BlocklistMetrics       BlocklistAnalytics
	ResolutionTrends       map[time.Time]int64
	TotalResolved          int64
	TotalResolutionSeconds float64
	AvgResolutionSeconds   float64
	StatusDurations        map[models.CaseStatus]time.Duration
	AvgStatusDurations     map[models.CaseStatus]time.Duration
}

var (
	ErrMetricRequired    = errors.New("metric parameter is required")
	ErrTimeRangeRequired = errors.New("timeRange parameter is required")
	ErrInvalidMetric     = errors.New("invalid metric")
)

type CaseService interface {
	core.Service

	// Create creates a new case
	Create(caseData *models.Case) (*models.Case, error)

	// GetByID retrieves a case by its ID
	GetByID(id uint) (*models.Case, error)

	// GetStatusFlowData returns status transition data for Sankey visualization
	GetStatusFlowData(filters []queryutil.CrudFilter) (*StatusFlowGraph, error)

	// GetAnalytics returns aggregated case metrics with filters
	GetAnalytics(filters []queryutil.CrudFilter) (*CaseAnalytics, error)

	// Get7DayAnalytics returns analytics for the last 7 days
	Get7DayAnalytics() (*CaseAnalytics, error)

	// Get30DayAnalytics returns analytics for the last 30 days
	Get30DayAnalytics() (*CaseAnalytics, error)

	// Get24HourAnalytics returns analytics for the last 24 hours
	Get24HourAnalytics() (*CaseAnalytics, error)

	// GetTimeSeriesMetrics returns time-series metrics for cases
	// Parameters:
	//   metric: The metric to retrieve (open_cases, new_cases, resolved_cases)
	//   timeRange: The time range (7d, 30d, 90d)
	//   filters: Optional filters to apply
	GetTimeSeriesMetrics(metric string, timeRange string, filters []queryutil.CrudFilter) ([]int64, error)

	// List returns a list of cases with filtering, sorting and pagination
	List(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Case, int64, error)

	// Update updates an existing case
	Update(caseModel *models.Case) error

	// UpdateStatus updates the status of a case
	UpdateStatus(id uint, status models.CaseStatus) error

	// GetCaseByReference retrieves a case by its reference number
	GetCaseByReference(reference string) (*models.Case, error)

	// GetPublicCase gets a case for public access
	GetPublicCase(reference string, reporterID uint) (*models.Case, error)

	// Search performs a more advanced search on cases
	Search(ctx context.Context, query string, filters []queryutil.CrudFilter, pagination queryutil.Pagination) ([]models.Case, int64, error)

	// SendCreationNotification sends an email notification when a case is created
	SendCreationNotification(caseID uint) error

	// SendStatusUpdateNotification sends an email notification when a case status is updated
	SendStatusUpdateNotification(caseID uint, oldStatus, newStatus models.CaseStatus) error

	// GetTypeSourceMatrix returns a breakdown of case counts by type and source over a time range
	GetTypeSourceMatrix(timeRange string, filters []queryutil.CrudFilter) ([]models.CaseTypeSourceBreakdown, error)
}
