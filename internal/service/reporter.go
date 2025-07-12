package service

import (
	"context"
	"errors"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"go.uber.org/zap"
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
	return svc, core.ContextOptions(
		core.ContextWithStartupFunc(func(ctx core.Context) error {
			svc.BaseService.InitializeBaseService(ctx, svc)
			return nil
		}),
	), nil
}

// ID returns the service identifier
func (s *ReporterServiceDefault) ID() string {
	return typesSvc.REPORTER_SERVICE
}

// Create creates a new reporter
func (s *ReporterServiceDefault) Create(reporter *models.Reporter) (*models.Reporter, error) {
	if err := reporter.Validate(); err != nil {
		return nil, err
	}

	if err := db.Create(context.Background(), s.ctx, s.db, reporter); err != nil {
		return nil, db.HandleDBError(err, "Create", "Reporter", 0)
	}

	return reporter, nil
}

// GetByID retrieves a reporter by ID
func (s *ReporterServiceDefault) GetByID(id uint) (*models.Reporter, error) {
	var reporter models.Reporter
	if err := db.GetByID(context.Background(), s.ctx, s.db, id, &reporter); err != nil {
		if db.IsRecordNotFound(err) {
			return nil, db.ErrRecordNotFound
		}
		s.logger.Error("Failed to get reporter by ID", zap.Error(err), zap.Uint("id", id))
		return nil, db.HandleDBError(err, "GetByID", "Reporter", id)
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
		if db.IsRecordNotFound(err) {
			return nil, db.ErrRecordNotFound
		}
		s.logger.Error("Failed to fetch reporter by email", zap.Error(err), zap.String("email", email))
		return nil, db.HandleDBError(err, "GetByEmail", "Reporter", 0)
	}

	return reporter, nil
}

// List returns a list of reporters with filtering and pagination
func (s *ReporterServiceDefault) List(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Reporter, int64, error) {
	var reporters []models.Reporter
	var total int64

	if err := db.List(context.Background(), s.ctx, s.db, filters, sorts, pagination, &reporters, &total); err != nil {
		s.logger.Error("Failed to list reporters", zap.Error(err))
		return nil, 0, db.HandleDBError(err, "List", "Reporter", 0)
	}

	return reporters, total, nil
}

// Update updates a reporter
func (s *ReporterServiceDefault) Update(reporter *models.Reporter) error {
	result := s.db.Model(reporter).Updates(reporter)
	if result.Error != nil {
		return db.HandleDBError(result.Error, "Update", "Reporter", reporter.ID)
	}

	// Check if any rows were actually updated
	if result.RowsAffected == 0 {
		return db.ErrRecordNotFound
	}

	return nil
}

// GetTrustStatus checks if reporter is trusted based on case history
func (s *ReporterServiceDefault) GetTrustStatus(reporter *models.Reporter) (models.ReporterTrustStatus, error) {
	if reporter == nil || reporter.ID == 0 {
		return models.ReporterNew, nil
	}

	var resolvedCases int64
	err := s.db.Model(&models.Case{}).
		Where("reporter_id = ? AND status IN (?)",
			reporter.ID,
			[]models.CaseStatus{models.CaseStatusClosed, models.CaseStatusResolved}).
		Count(&resolvedCases).
		Error
	if err != nil {
		return models.ReporterNew, db.HandleDBError(err, "Count", "Case", 0)
	}

	if resolvedCases > 0 {
		return models.ReporterTrusted, nil
	}
	return models.ReporterUntrusted, nil
}

// IsTrusted is a convenience wrapper around GetTrustStatus
func (s *ReporterServiceDefault) IsTrusted(reporter *models.Reporter) (bool, error) {
	status, err := s.GetTrustStatus(reporter)
	return status == models.ReporterTrusted, err
}
