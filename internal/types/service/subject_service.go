package service

import (
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
)

// SubjectService defines the interface for subject tracking operations
type SubjectService interface {
	core.Service

	// Create creates a new subject or returns an existing one
	Create(subject *models.Subject) (*models.Subject, error)

	// GetByID retrieves a subject by ID
	GetByID(id uint) (*models.Subject, error)

	// FindOrCreate finds an existing subject or creates a new one
	FindOrCreate(identifier string, subjectType models.SubjectType) (*models.Subject, error)

	// List returns a list of subjects with filtering and pagination
	List(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Subject, int64, error)
}
