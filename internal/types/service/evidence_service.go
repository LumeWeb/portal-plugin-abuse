package service

import (
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"gorm.io/datatypes"
	"io"
	"time"
)

const EVIDENCE_SERVICE = "evidence"

// EvidenceService defines the interface for evidence management
type EvidenceService interface {
	core.Service

	// CreateFromData creates an evidence record from raw file data
	CreateFromData(r io.ReadCloser, model *models.Evidence) (*models.Evidence, error)

	// CreateFromHash creates an evidence record using a pre-stored hash
	CreateFromHash(caseID uint, submitterID uint, storageHash string, fileName string, contentType string, fileSize int64, source models.EvidenceSource, description string, metadata datatypes.JSON) (*models.Evidence, error)

	// GetByCaseID gets all evidence for a case
	GetByCaseID(caseID uint, pagination queryutil.Pagination) ([]models.Evidence, int64, error)

	// GetByID gets evidence by ID
	GetByID(id uint) (*models.Evidence, error)

	// GetContent retrieves the evidence content
	GetContent(id uint) (io.ReadCloser, string, error)

	// GetEvidenceMetrics gets evidence metrics within a date range
	GetEvidenceMetrics(start, end time.Time) (*EvidenceAnalytics, error)
}
