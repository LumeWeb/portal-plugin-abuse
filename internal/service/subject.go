package service

import (
	"context"
	"errors"
	"fmt"

	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"go.uber.org/zap"
	"gorm.io/gorm"
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

	options := []core.ContextBuilderOption{
		func(ctx core.Context) (core.Context, error) {
			svc.BaseService.InitializeBaseService(ctx, svc)
			return ctx, nil
		},
	}

	return svc, options, nil
}

// ID returns the service identifier
func (s *SubjectServiceDefault) ID() string {
	return typesSvc.SUBJECT_SERVICE
}

// Create creates a new subject
func (s *SubjectServiceDefault) Create(subject *models.Subject) (*models.Subject, error) {
	if err := db.Create(context.Background(), s.ctx, s.db, subject); err != nil {
		s.logger.Error("Failed to create subject", zap.Error(err))
		return nil, fmt.Errorf("failed to create subject: %w", err)
	}
	return subject, nil
}

// GetByID retrieves a subject by ID
func (s *SubjectServiceDefault) GetByID(id uint) (*models.Subject, error) {
	var subject models.Subject
	if err := db.GetByID(context.Background(), s.ctx, s.db, id, &subject); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("subject not found")
		}
		s.logger.Error("Failed to get subject by ID", zap.Error(err), zap.Uint("id", id))
		return nil, fmt.Errorf("failed to get subject: %w", err)
	}
	return &subject, nil
}

// FindOrCreate finds an existing subject or creates a new one
func (s *SubjectServiceDefault) FindOrCreate(identifier string, subjectType models.SubjectType) (*models.Subject, error) {
	var subject models.Subject
	properties := map[string]any{
		"identifier": identifier,
		"type":       subjectType,
	}

	hash, err := core.ParseStorageHash(identifier)
	if err != nil {
		return nil, err
	}

	err = db.GetByProperties(context.Background(), s.ctx, s.db, properties, &subject)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create a new subject if not found
			subject = models.Subject{
				Identifier: hash.Multihash(),
				Type:       subjectType,
			}
			if err := db.Create(context.Background(), s.ctx, s.db, &subject); err != nil {
				s.logger.Error("Failed to create subject", zap.Error(err), zap.String("identifier", identifier), zap.String("type", string(subjectType)))
				return nil, fmt.Errorf("failed to create subject: %w", err)
			}
			return &subject, nil
		}

		s.logger.Error("Failed to get subject by properties", zap.Error(err), zap.String("identifier", identifier), zap.String("type", string(subjectType)))
		return nil, fmt.Errorf("failed to get subject: %w", err)
	}

	return &subject, nil
}

// List returns a list of subjects with filtering and pagination
func (s *SubjectServiceDefault) List(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Subject, int64, error) {
	var subjects []models.Subject
	var total int64

	if err := db.List(context.Background(), s.ctx, s.db, filters, sorts, pagination, &subjects, &total); err != nil {
		s.logger.Error("Failed to list subjects", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list subjects: %w", err)
	}

	return subjects, total, nil
}
