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

var (
	ErrInvalidReporterEmail = errors.New("invalid reporter email")
)

// ReporterServiceDefault implements the ReporterService interface
type ReporterServiceDefault struct {
	BaseService
}

// Ensure ReporterServiceDefault implements the interface
var _ typesSvc.ReporterService = (*ReporterServiceDefault)(nil)

// NewReporterService creates a new reporter service
func NewReporterService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &ReporterServiceDefault{}

	options := []core.ContextBuilderOption{
		func(ctx core.Context) (core.Context, error) {
			svc.BaseService.InitializeBaseService(ctx, svc)
			return ctx, nil
		},
	}

	return svc, options, nil
}

// ID returns the service identifier
func (s *ReporterServiceDefault) ID() string {
	return typesSvc.REPORTER_SERVICE
}

// Create creates a new reporter
func (s *ReporterServiceDefault) Create(reporter *models.Reporter) (*models.Reporter, error) {
	if err := db.Create(context.Background(), s.ctx, s.db, reporter); err != nil {
		s.logger.Error("Failed to create reporter", zap.Error(err), zap.String("email", reporter.Email))
		return nil, fmt.Errorf("failed to create reporter: %w", err)
	}

	return reporter, nil
}

// GetByID retrieves a reporter by ID
func (s *ReporterServiceDefault) GetByID(id uint) (*models.Reporter, error) {
	var reporter models.Reporter
	if err := db.GetByID(context.Background(), s.ctx, s.db, id, &reporter); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("reporter not found")
		}
		s.logger.Error("Failed to get reporter by ID", zap.Error(err), zap.Uint("id", id))
		return nil, fmt.Errorf("failed to get reporter: %w", err)
	}
	return &reporter, nil
}

// GetByEmail retrieves a reporter by email
func (s *ReporterServiceDefault) GetByEmail(email string) (*models.Reporter, error) {
	verify, err := getEmailVerifier().Verify(email)
	if err != nil {
		return nil, err
	}
	if !verify.Syntax.Valid {
		return nil, ErrInvalidReporterEmail
	}

	reporter := &models.Reporter{}

	err = db.GetByProperty(context.Background(), s.ctx, s.db, "email", email, reporter)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("reporter not found")
		}
		s.logger.Error("Failed to fetch reporter by email", zap.Error(err), zap.String("email", email))
		return nil, fmt.Errorf("failed to fetch reporter: %w", err)
	}

	return reporter, nil
}

// List returns a list of reporters with filtering and pagination
func (s *ReporterServiceDefault) List(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Reporter, int64, error) {
	var reporters []models.Reporter
	var total int64

	if err := db.List(context.Background(), s.ctx, s.db, filters, sorts, pagination, &reporters, &total); err != nil {
		s.logger.Error("Failed to list reporters", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list reporters: %w", err)
	}

	return reporters, total, nil
}

// Update updates a reporter
func (s *ReporterServiceDefault) Update(reporter *models.Reporter) error {
	if err := db.Update(context.Background(), s.ctx, s.db, reporter); err != nil {
		s.logger.Error("Failed to update reporter", zap.Error(err), zap.Uint("reporterID", reporter.ID))
		return fmt.Errorf("failed to update reporter: %w", err)
	}

	return nil
}
