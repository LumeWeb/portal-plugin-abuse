package service

import (
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
)

// ReporterService defines the interface for reporter management operations
type ReporterService interface {
	core.Service

	// Create creates a new reporter
	Create(reporter *models.Reporter) (*models.Reporter, error)

	// GetByID retrieves a reporter by ID
	GetByID(id uint) (*models.Reporter, error)

	// GetByEmail retrieves a reporter by email
	GetByEmail(email string) (*models.Reporter, error)

	// List returns a list of reporters with filtering and pagination
	List(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Reporter, int64, error)

	// Update updates a reporter
	Update(reporter *models.Reporter) error
}
