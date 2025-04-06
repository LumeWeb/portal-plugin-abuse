package service

import (
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"time"
)

// CommunicationService defines the interface for communication management operations
type CommunicationService interface {
	core.Service

	// Create adds a new communication
	Create(comm *models.Communication) (*models.Communication, error)

	// GetByID retrieves a communication by ID
	GetByID(id uint) (*models.Communication, error)

	// GetByThreadID retrieves a communication by thread ID
	GetByThreadID(threadID string) (*models.Communication, error)

	// ListByCaseID retrieves all communications for a case with filtering, sorting and pagination
	ListByCaseID(caseID uint, filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Communication, int64, error)

	// GetCommMetrics gets communication metrics within a date range
	GetCommunicationMetrics(start, end time.Time) (*CommAnalytics, error)

	// GetCommunicationTimeline retrieves a timeline of communication counts per hour over a specified time range with optional filters.
	GetCommunicationTimeline(timeRange string, filters []queryutil.CrudFilter) ([]models.CommunicationHourlyCount, error)
}
