package service

import (
	"context"
	"crypto/sha256"
	"fmt"

	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"go.uber.org/zap"
)

// SubjectServiceDefault implements the SubjectService interface
type SubjectServiceDefault struct {
	BaseService
}

// Ensure SubjectServiceDefault implements the interface
var _ typesSvc.SubjectService = (*SubjectServiceDefault)(nil)

// NewSubjectService creates a new subject service
func NewSubjectService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &SubjectServiceDefault{}
	return svc, core.ContextOptions(
		core.ContextWithStartupFunc(func(ctx core.Context) error {
			svc.BaseService.InitializeBaseService(ctx, svc)
			return nil
		}),
	), nil
}

// ID returns the service identifier
func (s *SubjectServiceDefault) ID() string {
	return typesSvc.SUBJECT_SERVICE
}

// Create creates a new subject
func (s *SubjectServiceDefault) Create(subject *models.Subject) (*models.Subject, error) {
	if err := db.Create(context.Background(), s.ctx, s.db, subject); err != nil {
		s.logger.Error("Failed to create subject", zap.Error(err))
		return nil, db.HandleDBError(err, "Create", "Subject", 0)
	}
	return subject, nil
}

// GetByID retrieves a subject by ID
func (s *SubjectServiceDefault) GetByID(id uint) (*models.Subject, error) {
	var subject models.Subject
	if err := db.GetByID(context.Background(), s.ctx, s.db, id, &subject); err != nil {
		if db.IsRecordNotFound(err) {
			return nil, db.ErrRecordNotFound
		}
		s.logger.Error("Failed to get subject by ID", zap.Error(err), zap.Uint("id", id))
		return nil, db.HandleDBError(err, "GetByID", "Subject", id)
	}
	return &subject, nil
}

// FindOrCreate finds an existing subject or creates a new one
func (s *SubjectServiceDefault) FindOrCreate(identifier core.StorageHash, subjectType models.SubjectType) (*models.Subject, error) {
	var subject models.Subject
	properties := map[string]any{
		"identifier": identifier.Multihash(),
		"type":       subjectType,
	}

	err := db.GetByProperties(context.Background(), s.ctx, s.db, properties, &subject)
	if err != nil {
		if db.IsRecordNotFound(err) {
			// Create a new subject if not found
			subject = models.Subject{
				Identifier: identifier.Multihash(),
				Type:       subjectType,
			}
			if err := db.Create(context.Background(), s.ctx, s.db, &subject); err != nil {
				s.logger.Error("Failed to create subject", zap.Error(err), zap.Stringer("identifier", identifier), zap.String("type", string(subjectType)))
				return nil, db.HandleDBError(err, "Create", "Subject", 0)
			}
			return &subject, nil
		}

		s.logger.Error("Failed to get subject by properties", zap.Error(err), zap.Stringer("identifier", identifier), zap.String("type", string(subjectType)))
		return nil, fmt.Errorf("failed to get subject: %w", err)
	}

	return &subject, nil
}

// FindOrCreateByURL finds an existing subject by URL or creates a new one
func (s *SubjectServiceDefault) FindOrCreateByURL(url string, subjectType models.SubjectType) (*models.Subject, error) {
	var subject models.Subject
	properties := map[string]any{
		"source_url": url,
	}

	err := db.GetByProperties(context.Background(), s.ctx, s.db, properties, &subject)
	if err != nil {
		if db.IsRecordNotFound(err) {
			// Create a new subject if not found
			// Generate a dummy identifier since we're creating by URL
			// This is just a hash of the URL to satisfy the NOT NULL constraint
			h := sha256.Sum256([]byte(url))
			subject = models.Subject{
				SourceURL:  url,
				Type:       subjectType,
				Identifier: h[:],
			}
			if err := db.Create(context.Background(), s.ctx, s.db, &subject); err != nil {
				s.logger.Error("Failed to create subject", zap.Error(err), zap.String("url", url))
				return nil, db.HandleDBError(err, "Create", "Subject", 0)
			}
			return &subject, nil
		}

		s.logger.Error("Failed to get subject by properties", zap.Error(err), zap.String("url", url))
		return nil, fmt.Errorf("failed to get subject: %w", err)
	}

	return &subject, nil
}

// List returns a list of subjects with filtering and pagination
func (s *SubjectServiceDefault) List(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Subject, int64, error) {
	var subjects []models.Subject
	var total int64

	if err := db.List(context.Background(), s.ctx, s.db, filters, sorts, pagination, &subjects, &total); err != nil {
		s.logger.Error("Failed to list subjects", zap.Error(err))
		return nil, 0, db.HandleDBError(err, "List", "Subject", 0)
	}

	return subjects, total, nil
}
