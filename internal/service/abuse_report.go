package service

import (
	"context"
	"fmt"

	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
)

// AbuseReportServiceDefault implements the AbuseReportService interface
type AbuseReportServiceDefault struct {
	BaseService
	caseService     typesSvc.CaseService
	reporterService typesSvc.ReporterService
	subjectService  typesSvc.SubjectService
}

// Ensure AbuseReportServiceDefault implements the interface
var _ typesSvc.AbuseReportService = (*AbuseReportServiceDefault)(nil)

// NewAbuseReportService creates a new abuse report service
func NewAbuseReportService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &AbuseReportServiceDefault{}
	return svc, core.ContextOptions(
		core.ContextWithStartupFunc(func(ctx core.Context) error {
			svc.BaseService.InitializeBaseService(ctx, svc)

			// Get required services
			caseService := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
			if caseService == nil {
				return fmt.Errorf("case service not available")
			}
			svc.caseService = caseService

			reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE)
			if reporterService == nil {
				return fmt.Errorf("reporter service not available")
			}
			svc.reporterService = reporterService

			subjectService := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE)
			if subjectService == nil {
				return fmt.Errorf("subject service not available")
			}
			svc.subjectService = subjectService

			return nil
		}),
	), nil
}

// ID returns the service identifier
func (s *AbuseReportServiceDefault) ID() string {
	return typesSvc.ABUSE_REPORT_SERVICE
}

// SubmitReport submits a new abuse report
func (s *AbuseReportServiceDefault) SubmitReport(_ context.Context, caseData *models.Case) (*models.Case, error) {
	// Create reporter
	reporter := &models.Reporter{
		Email: caseData.Reporter.Email,
		Name:  caseData.Reporter.Name,
	}

	// Check if reporter already exists
	existingReporter, err := s.reporterService.GetByEmail(reporter.Email)
	if err != nil {
		if !db.IsRecordNotFound(err) {
			s.logger.Error("Failed to get reporter by email", zap.Error(err), zap.String("email", reporter.Email))
			return nil, fmt.Errorf("failed to get reporter by email: %w", err)
		}
	}

	if existingReporter != nil {
		reporter = existingReporter
	} else {
		// Create new reporter
		createdReporter, err := s.reporterService.Create(reporter)
		if err != nil {
			s.logger.Error("Failed to create reporter", zap.Error(err), zap.String("email", reporter.Email))
			return nil, db.HandleDBError(err, "Create", "Reporter", 0)
		}
		reporter = createdReporter
	}

	// Create subject
	subject := &models.Subject{
		Identifier: caseData.Subject.Identifier,
		Type:       caseData.Subject.Type,
		SourceURL:  caseData.Subject.SourceURL,
	}

	createdSubject, err := s.subjectService.Create(subject)
	if err != nil {
		s.logger.Error("Failed to create subject", zap.Error(err), zap.Stringer("identifier", subject.Identifier), zap.String("type", string(subject.Type)))
		return nil, db.HandleDBError(err, "Create", "Subject", 0)
	}
	subject = createdSubject

	caseData.ReporterID = reporter.ID
	caseData.SubjectID = subject.ID
	caseData.Reporter = models.Reporter{}
	caseData.Subject = models.Subject{}

	// Create case through service to ensure notifications
	createdCase, err := s.caseService.Create(caseData)
	if err != nil {
		s.logger.Error("Failed to create case", zap.Error(err), zap.Uint("reporterID", caseData.ReporterID), zap.Uint("subjectID", caseData.SubjectID))
		return nil, db.HandleDBError(err, "Create", "Case", 0)
	}
	caseData = createdCase

	return caseData, nil
}

// GetReportStatus retrieves the status of a report by confirmation number
func (s *AbuseReportServiceDefault) GetReportStatus(_ context.Context, confirmationNumber string) (*models.Case, error) {
	return s.getCaseByReference(confirmationNumber)
}

// getCaseByReference retrieves a case by its reference number
func (s *AbuseReportServiceDefault) getCaseByReference(referenceNumber string) (*models.Case, error) {
	var caseModel models.Case
	err := db.GetByProperty(context.Background(), s.ctx, s.db, "reference_number", referenceNumber, &caseModel)
	if err != nil {
		if db.IsRecordNotFound(err) {
			return nil, db.ErrRecordNotFound
		}
		s.logger.Error("Failed to get case by reference number", zap.Error(err), zap.String("referenceNumber", referenceNumber))
		return nil, db.HandleDBError(err, "getCaseByReference", "Case", 0)
	}
	return &caseModel, nil
}
