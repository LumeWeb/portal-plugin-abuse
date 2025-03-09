package service

import (
	"context"
	"errors"
	"fmt"
	"go.lumeweb.com/portal-plugin-abuse/internal"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"time"
)

// CommunicationServiceDefault handles communication management operations
type CommunicationServiceDefault struct {
	BaseService
	caseSvc     typesSvc.CaseService
	reporterSvc typesSvc.ReporterService
	emailSvc    typesSvc.EmailService
}

// Ensure CommunicationServiceDefault implements the interface
var _ typesSvc.CommunicationService = (*CommunicationServiceDefault)(nil)

// ID returns the service identifier
func (s *CommunicationServiceDefault) ID() string {
	return typesSvc.COMMUNICATION_SERVICE
}

// NewCommunicationService creates a new communication service
func NewCommunicationService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &CommunicationServiceDefault{}

	options := []core.ContextBuilderOption{
		func(ctx core.Context) (core.Context, error) {
			svc.BaseService.InitializeBaseService(ctx, svc)

			// Get required services
			caseSvc := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
			if caseSvc == nil {
				return ctx, fmt.Errorf("case service not available")
			}
			svc.caseSvc = caseSvc

			reporterSvc := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE)
			if reporterSvc == nil {
				return ctx, fmt.Errorf("reporter service not available")
			}
			svc.reporterSvc = reporterSvc

			emailSvc := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE)
			if emailSvc == nil {
				return ctx, fmt.Errorf("email service not available")
			}
			svc.emailSvc = emailSvc

			return ctx, nil
		},
	}

	return svc, options, nil
}

// Create adds a new communication
func (s *CommunicationServiceDefault) Create(comm *models.Communication) (*models.Communication, error) {
	if err := db.Create(context.Background(), s.ctx, s.db, comm); err != nil {
		s.logger.Error("Failed to create communication", zap.Error(err))
		return nil, fmt.Errorf("failed to create communication: %w", err)
	}

	// Send notifications async for user-facing comments
	if comm.Direction == models.CommunicationDirectionIncoming {
		go s.notifyNewComment(comm)
	}

	return comm, nil
}

func (s *CommunicationServiceDefault) notifyNewComment(comm *models.Communication) {
	caseModel, err := s.caseSvc.GetByID(comm.CaseID)
	if err != nil {
		s.logger.Error("Failed to get case for comment notification",
			zap.Uint("caseID", comm.CaseID),
			zap.Error(err))
		return
	}

	reporter, err := s.reporterSvc.GetByID(caseModel.ReporterID)
	if err != nil {
		s.logger.Error("Failed to get reporter for comment notification",
			zap.Uint("reporterID", caseModel.ReporterID),
			zap.Error(err))
		return
	}

	siteURL := core.GetService[core.HTTPService](s.ctx, core.HTTP_SERVICE).APISubdomain(internal.PLUGIN_NAME, true)

	templateData := core.MailerTemplateData{
		"CaseID":         caseModel.ReferenceNumber,
		"ReporterName":   reporter.Name,
		"PortalName":     s.ctx.Config().Config().Core.PortalName,
		"CommentContent": comm.Content,
		"CommentDate":    comm.CreatedAt.Format("January 2, 2006 15:04"),
		"CaseURL":        fmt.Sprintf("%s/case/%s", siteURL, caseModel.ReferenceNumber),
		"CaseType":       string(caseModel.Type),
		"CaseStatus":     string(caseModel.Status),
	}

	// Validate template requirements
	requiredFields := []string{"CaseID", "ReporterName", "PortalName", "CommentContent", "CommentDate", "CaseURL"}
	if err := s.ValidateTemplateData("case_comment_added", templateData, requiredFields); err != nil {
		s.logger.Error("Invalid template data for comment notification",
			zap.Uint("caseID", comm.CaseID),
			zap.Error(err))
		return
	}

	err = s.emailSvc.SendTemplatedEmail(
		[]string{reporter.Email},
		"case_comment_added",
		templateData,
	)

	if err != nil {
		s.logger.Error("Failed to send comment notification",
			zap.Uint("caseID", comm.CaseID),
			zap.Uint("commID", comm.ID),
			zap.Error(err))
	}
}

// GetByID retrieves a communication by ID
func (s *CommunicationServiceDefault) GetByID(id uint) (*models.Communication, error) {
	var communication models.Communication
	if err := db.GetByID(context.Background(), s.ctx, s.db, id, &communication); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("communication not found")
		}
		s.logger.Error("Failed to get communication by ID", zap.Error(err), zap.Uint("communicationID", id))
		return nil, fmt.Errorf("failed to get communication: %w", err)
	}
	return &communication, nil
}

// GetByThreadID retrieves a communication by thread ID
func (s *CommunicationServiceDefault) GetByThreadID(threadID string) (*models.Communication, error) {
	var communication models.Communication
	err := db.GetByProperty(context.Background(), s.ctx, s.db, "thread_id", threadID, &communication)
	if err != nil {
		if errors.Is(err, fmt.Errorf("record not found")) {
			return nil, fmt.Errorf("communication not found")
		}
		s.logger.Error("Failed to fetch communication by thread ID", zap.Error(err), zap.String("threadID", threadID))
		return nil, fmt.Errorf("failed to fetch communication by thread ID: %w", err)
	}
	return &communication, nil
}

// ListByCaseID retrieves all communications for a case with filtering, sorting and pagination
func (s *CommunicationServiceDefault) ListByCaseID(caseID uint, filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Communication, int64, error) {
	var communications []models.Communication
	var total int64

	// Prepend case_id filter to ensure scope
	caseFilter := queryutil.Filter{Field: "case_id", Operator: queryutil.OperatorEquals, Value: caseID}
	combinedFilters := append([]queryutil.Filter{caseFilter}, filters...)

	if err := db.List(context.Background(), s.ctx, s.db, combinedFilters, sorts, pagination, &communications, &total); err != nil {
		s.logger.Error("Failed to list communications by case ID", zap.Error(err), zap.Uint("caseID", caseID))
		return nil, 0, fmt.Errorf("failed to list communications: %w", err)
	}

	return communications, total, nil
}

// Update updates an existing communication
func (s *CommunicationServiceDefault) Update(communication *models.Communication) error {
	if err := db.Update(context.Background(), s.ctx, s.db, communication); err != nil {
		s.logger.Error("Failed to update communication", zap.Error(err), zap.Uint("communicationID", communication.ID))
		return fmt.Errorf("failed to update communication: %w", err)
	}

	return nil
}

// Delete deletes a communication
func (s *CommunicationServiceDefault) GetCommunicationMetrics(start, end time.Time) (*typesSvc.CommAnalytics, error) {
	analytics := &typesSvc.CommAnalytics{
		CommsPerCase: make(map[uint]int64),
	}

	// Get comms per case
	var counts []struct {
		CaseID uint
		Count  int64
	}

	query := s.db.Model(&models.Communication{})
	if !start.IsZero() {
		query = query.Where("created_at >= ?", start)
	}
	if !end.IsZero() {
		query = query.Where("created_at <= ?", end)
	}

	err := query.Select("case_id, count(*) as count").
		Group("case_id").
		Scan(&counts).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get comms per case: %w", err)
	}

	for _, c := range counts {
		analytics.CommsPerCase[c.CaseID] = c.Count
	}

	// Calculate response times
	var responseStats []struct {
		MinCreatedAt time.Time
		MaxCreatedAt time.Time
		CaseID       uint
	}

	err = s.db.Model(&models.Communication{}).
		Select("case_id, MIN(created_at) as min_created_at, MAX(created_at) as max_created_at").
		Where("direction = ?", models.CommunicationDirectionIncoming).
		Group("case_id").
		Scan(&responseStats).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get response times: %w", err)
	}

	var totalDuration time.Duration
	var maxDuration time.Duration
	for _, stat := range responseStats {
		duration := stat.MaxCreatedAt.Sub(stat.MinCreatedAt)
		totalDuration += duration
		if duration > maxDuration {
			maxDuration = duration
		}
	}

	if len(responseStats) > 0 {
		analytics.AvgResponseTime = totalDuration / time.Duration(len(responseStats))
	}
	analytics.MaxResponseTime = maxDuration

	return analytics, nil
}

func (s *CommunicationServiceDefault) Delete(id uint) error {
	var communication models.Communication
	if err := db.Delete(context.Background(), s.ctx, s.db, id, &communication); err != nil {
		s.logger.Error("Failed to delete communication", zap.Error(err), zap.Uint("communicationID", id))
		return fmt.Errorf("failed to delete communication: %w", err)
	}

	return nil
}
