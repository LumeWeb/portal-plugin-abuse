package service

import (
	"context"
	"errors"
	"fmt"
	"go.lumeweb.com/portal-plugin-abuse/internal"
	"time"

	"go.lumeweb.com/portal-plugin-abuse/internal/config"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CaseServiceDefault handles business logic for case management
type CaseServiceDefault struct {
	BaseService
	reporterSvc      typesSvc.ReporterService
	subjectSvc       typesSvc.SubjectService
	emailSvc         typesSvc.EmailService
	tokenSvc         typesSvc.TokenService
	communicationSvc typesSvc.CommunicationService
	evidenceSvc      typesSvc.EvidenceService
	blocklistSvc     typesSvc.BlockListService
	httpSvc          core.HTTPService
}

// Ensure CaseServiceDefault implements the interface
var _ typesSvc.CaseService = (*CaseServiceDefault)(nil)

// ID returns the service identifier
func (s *CaseServiceDefault) ID() string {
	return typesSvc.CASE_SERVICE
}

// Config returns the configuration for this service
func (s *CaseServiceDefault) Config() (any, error) {
	return nil, nil
}

// UpdateConfig updates the configuration for this service
func (s *CaseServiceDefault) UpdateConfig(config any) error {
	// CaseService doesn't use configuration currently
	return nil
}

// NewCaseService creates a new case service
func NewCaseService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &CaseServiceDefault{}

	options := []core.ContextBuilderOption{
		func(ctx core.Context) (core.Context, error) {
			svc.BaseService.InitializeBaseService(ctx, svc)

			// Get required services
			reporterSvc := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE)
			if reporterSvc == nil {
				return ctx, fmt.Errorf("reporter service not available")
			}
			svc.reporterSvc = reporterSvc

			subjectSvc := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE)
			if subjectSvc == nil {
				return ctx, fmt.Errorf("subject service not available")
			}
			svc.subjectSvc = subjectSvc

			emailSvc := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE)
			if emailSvc == nil {
				return ctx, fmt.Errorf("email service not available")
			}
			svc.emailSvc = emailSvc

			tokenSvc := core.GetService[typesSvc.TokenService](ctx, typesSvc.TOKEN_SERVICE)
			if tokenSvc == nil {
				return ctx, fmt.Errorf("token service not available")
			}
			svc.tokenSvc = tokenSvc

			communicationSvc := core.GetService[typesSvc.CommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
			if communicationSvc == nil {
				return ctx, fmt.Errorf("communication service not available")
			}
			svc.communicationSvc = communicationSvc

			evidenceSvc := core.GetService[typesSvc.EvidenceService](ctx, typesSvc.EVIDENCE_SERVICE)
			if evidenceSvc == nil {
				return ctx, fmt.Errorf("evidence service not available")
			}
			svc.evidenceSvc = evidenceSvc

			blocklistSvc := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
			if blocklistSvc == nil {
				return ctx, fmt.Errorf("blocklist service not available")
			}
			svc.blocklistSvc = blocklistSvc

			httpSvc := core.GetService[core.HTTPService](ctx, core.HTTP_SERVICE)
			if httpSvc == nil {
				return ctx, fmt.Errorf("http service not available")
			}
			svc.httpSvc = httpSvc

			return ctx, nil
		},
	}

	return svc, options, nil
}

// SendCreationNotification sends an email notification when a case is created
func (s *CaseServiceDefault) SendCreationNotification(caseID uint) error {
	// Get the case
	caseModel, err := s.GetByID(caseID)
	if err != nil {
		s.logger.Error("Failed to get case", zap.Error(err), zap.Uint("caseID", caseID))
		return fmt.Errorf("failed to get case for notification: %w", err)
	}

	reporter, err := s.reporterSvc.GetByID(caseModel.ReporterID)
	if err != nil {
		s.logger.Error("Failed to get reporter", zap.Error(err), zap.Uint("reporterID", caseModel.ReporterID))
		return fmt.Errorf("failed to get reporter for notification: %w", err)
	}

	subject, err := s.subjectSvc.GetByID(caseModel.SubjectID)
	if err != nil {
		s.logger.Error("Failed to get subject", zap.Error(err), zap.Uint("subjectID", caseModel.SubjectID))
		return fmt.Errorf("failed to get subject for notification: %w", err)
	}

	// Generate thread ID for replies
	threadID := s.emailSvc.GenerateCaseThreadID(caseID, caseModel.ReferenceNumber)

	siteURL := s.httpSvc.APISubdomain(internal.PLUGIN_NAME, true)

	// Generate access token for the reporter (valid for 90 days)
	accessToken, err := s.tokenSvc.GenerateToken(caseID, reporter.ID, 90)
	if err != nil {
		s.logger.Error("Failed to generate access token", zap.Error(err), zap.Uint("caseID", caseID), zap.Uint("reporterID", reporter.ID))
	} else {
		// Send access email with tokenized URL
		accessTemplateData := core.MailerTemplateData{
			"CaseID":       caseModel.ReferenceNumber,
			"ReporterName": reporter.Name,
			"PortalName":   s.ctx.Config().Config().Core.PortalName,
			"AccessURL":    fmt.Sprintf("%s/access?token=%s", siteURL, accessToken),
			"ExpiresIn":    "90 days",
			"CreatedDate":  caseModel.CreatedAt.Format("January 2, 2006"),
			"CaseType":     string(caseModel.Type),
			"CaseStatus":   string(caseModel.Status),
		}

		if err := s.emailSvc.SendTemplatedEmail(
			[]string{reporter.Email},
			"case_access",
			accessTemplateData,
		); err != nil {
			s.logger.Error("Failed to send case access email", zap.Error(err))
		}
	}

	// Send main creation notification
	templateData := core.MailerTemplateData{
		"CaseID":        caseModel.ReferenceNumber,
		"Reference":     caseModel.ReferenceNumber,
		"ReporterName":  reporter.Name,
		"ReporterEmail": reporter.Email,
		"SubjectType":   subject.Type,
		"SubjectHash":   subject.Identifier,
		"CaseURL":       fmt.Sprintf("%s/case/%s", siteURL, caseModel.ReferenceNumber),
		"PortalName":    s.ctx.Config().Config().Core.PortalName,
		"CreatedDate":   caseModel.CreatedAt.Format("January 2, 2006"),
		"ReplyTo":       threadID,
	}

	// Add high-priority warning if needed
	if caseModel.Priority == models.CasePriorityHigh {
		templateData["HighPriorityWarning"] = true
		templateData["PriorityReason"] = "This case has been marked as high priority due to the nature of the report"
	}

	// Validate template requirements
	requiredFields := []string{"CaseID", "ReporterName", "PortalName", "CaseURL", "CreatedDate"}
	if err := s.ValidateTemplateData("case_created", templateData, requiredFields); err != nil {
		s.logger.Error("Invalid template data for case creation notification",
			zap.Uint("caseID", caseModel.ID),
			zap.Error(err))
		return fmt.Errorf("failed to validate template data: %w", err)
	}

	if err := s.emailSvc.SendTemplatedEmail(
		[]string{reporter.Email},
		"case_created",
		templateData,
	); err != nil {
		return fmt.Errorf("failed to send creation notification: %w", err)
	}

	comm := &models.Communication{
		CaseID:    caseID,
		SenderID:  0, // System sender
		Type:      models.CommunicationTypeEmail,
		Direction: models.CommunicationDirectionOutgoing,
		Content:   "Case creation notification sent to reporter",
		ThreadID:  threadID,
	}

	_, err = s.communicationSvc.Create(comm)
	if err != nil {
		s.logger.Error("Failed to create communication record", zap.Error(err), zap.Uint("caseID", caseID))
		// Not a critical error, continue
	}

	return nil
}

// LinkSubject associates a subject with a case
func (s *CaseServiceDefault) LinkSubject(caseID, subjectID uint) error {
	_, err := s.GetByID(caseID)
	if err != nil {
		return fmt.Errorf("failed to get case: %w", err)
	}

	// Create association through CaseScan model
	scan := &models.CaseScan{
		CaseID:    caseID,
		SubjectID: subjectID,
		Status:    models.ScanStatusPending,
	}

	if err := db.Create(context.Background(), s.ctx, s.db, scan); err != nil {
		s.logger.Error("Failed to link subject to case",
			zap.Uint("case_id", caseID),
			zap.Uint("subject_id", subjectID),
			zap.Error(err))
		return fmt.Errorf("failed to link subject: %w", err)
	}

	return nil
}

// SendStatusUpdateNotification sends an email notification when a case status is updated
func (s *CaseServiceDefault) SendStatusUpdateNotification(caseID uint, oldStatus, newStatus models.CaseStatus) error {
	// Get the case
	caseModel, err := s.GetByID(caseID)
	if err != nil {
		s.logger.Error("Failed to get case", zap.Error(err), zap.Uint("caseID", caseID))
		return fmt.Errorf("failed to get case for status update: %w", err)
	}

	reporter, err := s.reporterSvc.GetByID(caseModel.ReporterID)
	if err != nil {
		s.logger.Error("Failed to get reporter", zap.Error(err), zap.Uint("reporterID", caseModel.ReporterID))
		return fmt.Errorf("failed to get reporter for status update: %w", err)
	}

	// Generate thread ID for replies
	threadID := s.emailSvc.GenerateCaseThreadID(caseID, caseModel.ReferenceNumber)

	// Template data
	emailConfig, ok := s.ctx.Config().GetService(typesSvc.EMAIL_SERVICE).(*config.EmailConfig)
	siteURL := "https://example.com" // Default fallback
	if ok && emailConfig.SiteURL != "" {
		siteURL = emailConfig.SiteURL
	}

	templateData := core.MailerTemplateData{
		"CaseID":      caseModel.ID,
		"Reference":   caseModel.ReferenceNumber,
		"OldStatus":   oldStatus,
		"NewStatus":   newStatus,
		"PortalName":  s.ctx.Config().Config().Core.PortalName,
		"UpdatedDate": time.Now().Format("January 2, 2006 15:04"),
		"DetailsURL":  fmt.Sprintf("%s/case/%s", siteURL, caseModel.ReferenceNumber),
		"ReplyTo":     threadID,
	}

	err = s.emailSvc.SendTemplatedEmail(
		[]string{reporter.Email},
		"case_status_updated",
		templateData,
	)

	if err != nil {
		s.logger.Error("Failed to send templated email", zap.Error(err), zap.Strings("to", []string{reporter.Email}), zap.String("templateName", "case_status_updated"))
		return fmt.Errorf("failed to send status update notification: %w", err)
	}

	comm := &models.Communication{
		CaseID:    caseID,
		SenderID:  0, // System sender
		Type:      models.CommunicationTypeEmail,
		Direction: models.CommunicationDirectionOutgoing,
		Content:   fmt.Sprintf("Case status update notification sent to reporter. Status changed from %s to %s", oldStatus, newStatus),
		ThreadID:  threadID,
	}

	_, err = s.communicationSvc.Create(comm)
	if err != nil {
		s.logger.Error("Failed to create communication record", zap.Error(err), zap.Uint("caseID", caseID))
		// Not a critical error, continue
	}

	return nil
}

// Create creates a new case and sends notifications
func (s *CaseServiceDefault) Create(caseData *models.Case) (*models.Case, error) {
	if err := caseData.Validate(); err != nil {
		s.logger.Error("Invalid case data", zap.Error(err))
		return nil, fmt.Errorf("invalid case data: %w", err)
	}

	if err := db.Create(context.Background(), s.ctx, s.db, caseData); err != nil {
		s.logger.Error("Failed to create case", zap.Error(err))
		return nil, fmt.Errorf("failed to create case: %w", err)
	}

	// Send creation notification asynchronously
	go func() {
		if err := s.SendCreationNotification(caseData.ID); err != nil {
			s.logger.Error("Failed to send creation notification",
				zap.Uint("caseID", caseData.ID),
				zap.Error(err))
		}
	}()

	return caseData, nil
}

// GetByID retrieves a case by its ID
func (s *CaseServiceDefault) GetByID(id uint) (*models.Case, error) {
	var caseModel models.Case
	if err := db.GetByID(context.Background(), s.ctx, s.db, id, &caseModel, func(_db *gorm.DB) *gorm.DB {
		return _db.Preload("CaseScan")
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("case not found")
		}
		s.logger.Error("Failed to get case by ID", zap.Error(err), zap.Uint("caseID", id))
		return nil, fmt.Errorf("failed to fetch case: %w", err)
	}
	return &caseModel, nil
}

// List returns a list of cases with filtering, sorting and pagination
func (s *CaseServiceDefault) List(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Case, int64, error) {
	var cases []models.Case
	var total int64

	if err := db.List[models.Case](context.Background(), s.ctx, s.db, filters, sorts, pagination, &cases, &total); err != nil {
		s.logger.Error("Failed to list cases", zap.Error(err))
		return nil, 0, fmt.Errorf("failed to list cases: %w", err)
	}

	return cases, total, nil
}

// Update updates an existing case
func (s *CaseServiceDefault) Update(caseModel *models.Case) error {
	if err := caseModel.Validate(); err != nil {
		s.logger.Error("Invalid case data", zap.Error(err), zap.Uint("caseID", caseModel.ID))
		return fmt.Errorf("invalid case data: %w", err)
	}

	if err := db.Update(context.Background(), s.ctx, s.db, caseModel); err != nil {
		s.logger.Error("Failed to update case", zap.Error(err), zap.Uint("caseID", caseModel.ID))
		return fmt.Errorf("failed to update case: %w", err)
	}

	return nil
}

// UpdateStatus updates the status of a case and sends notifications
func (s *CaseServiceDefault) UpdateStatus(id uint, status models.CaseStatus) error {
	caseModel, err := s.GetByID(id)
	if err != nil {
		return err
	}

	oldStatus := caseModel.Status

	// Only update and notify if status actually changed
	if oldStatus == status {
		s.logger.Debug("Skipping status update - no change",
			zap.Uint("caseID", id),
			zap.String("status", string(status)))
		return nil
	}

	// Record status change history
	historyEntry := models.CaseStatusHistory{
		CaseID:    id,
		OldStatus: oldStatus,
		NewStatus: status,
		ChangedAt: time.Now(),
		ChangedBy: 0, // 0 indicates system-generated change
	}

	if err := db.Create(context.Background(), s.ctx, s.db, &historyEntry); err != nil {
		s.logger.Error("Failed to record status history",
			zap.Uint("caseID", id),
			zap.String("oldStatus", string(oldStatus)),
			zap.String("newStatus", string(status)),
			zap.Error(err))
	}

	caseModel.Status = status
	if err := s.Update(caseModel); err != nil {
		return err
	}

	// Send status update notification asynchronously
	go func() {
		if err := s.SendStatusUpdateNotification(id, oldStatus, status); err != nil {
			s.logger.Error("Failed to send status update notification",
				zap.Uint("caseID", id),
				zap.String("oldStatus", string(oldStatus)),
				zap.String("newStatus", string(status)),
				zap.Error(err))
		}
	}()

	return nil
}

// GetCaseByReference retrieves a case by its reference number
func (s *CaseServiceDefault) GetCaseByReference(reference string) (*models.Case, error) {
	var caseModel models.Case
	err := db.GetByProperty(context.Background(), s.ctx, s.db, "reference_number", reference, &caseModel)

	if err != nil {
		if errors.Is(err, fmt.Errorf("record not found")) {
			return nil, fmt.Errorf("case not found")
		}
		s.logger.Error("Failed to fetch case by reference", zap.Error(err), zap.String("reference", reference))
		return nil, fmt.Errorf("failed to fetch case: %w", err)
	}
	return &caseModel, nil
}

// GetPublicCase gets a case for public access
func (s *CaseServiceDefault) GetPublicCase(reference string, reporterID uint) (*models.Case, error) {
	caseModel, err := s.GetCaseByReference(reference)
	if err != nil {
		return nil, err
	}

	// Verify that the reporter ID matches
	if caseModel.ReporterID != reporterID {
		s.logger.Warn("Unauthorized access attempt", zap.String("reference", reference), zap.Uint("reporterID", reporterID))
		return nil, fmt.Errorf("unauthorized access")
	}

	return caseModel, nil
}

func (s *CaseServiceDefault) extractDateRange(filters []queryutil.Filter) (time.Time, time.Time, error) {
	var start, end time.Time
	now := time.Now()

	// Default to last 30 days if no filters
	end = now
	start = now.AddDate(0, 0, -30)

	// Look for date range in filters
	for _, filter := range filters {
		if filter.Field == "created_at" {
			switch filter.Operator {
			case queryutil.OperatorGTE:
				if t, ok := filter.Value.(time.Time); ok {
					start = t
				}
			case queryutil.OperatorLTE:
				if t, ok := filter.Value.(time.Time); ok {
					end = t
				}
			}
		}
	}

	// Validate date range
	if start.After(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("start date cannot be after end date")
	}

	return start, end, nil
}

// GetAnalytics returns aggregated case metrics with filters
func (s *CaseServiceDefault) GetAnalytics(filters []queryutil.Filter) (*typesSvc.CaseAnalytics, error) {
	// Extract date range first
	start, end, err := s.extractDateRange(filters)
	if err != nil {
		return nil, fmt.Errorf("invalid date range: %w", err)
	}

	// Initialize analytics with proper map allocations
	analytics := &typesSvc.CaseAnalytics{
		StatusBreakdown:   make(map[models.CaseStatus]int64),
		CaseTypeBreakdown: make(map[models.CaseType]int64),
		SourceBreakdown:   make(map[models.ReportSource]int64),
		CommsMetrics: typesSvc.CommAnalytics{
			CommsPerCase: make(map[uint]int64),
		},
		StatusDurations:    make(map[models.CaseStatus]time.Duration),
		AvgStatusDurations: make(map[models.CaseStatus]time.Duration),
	}

	// Total Cases
	totalCases, err := db.Count[models.Case](context.Background(), s.ctx, s.db, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get total cases: %w", err)
	}
	analytics.TotalCases = totalCases

	// Open Cases
	// Get open cases (not resolved or closed)
	openCases, err := db.Count[models.Case](context.Background(), s.ctx, s.db, filters,
		func(_db *gorm.DB) *gorm.DB {
			return _db.Where("status NOT IN (?)", []models.CaseStatus{
				models.CaseStatusResolved,
				models.CaseStatusClosed,
			})
		})
	if err != nil {
		return nil, fmt.Errorf("failed to get open cases: %w", err)
	}
	analytics.OpenCases = openCases

	// Status Breakdown
	type CaseStatusAggregate struct {
		Status models.CaseStatus
		Count  int64
	}
	var statusResults []CaseStatusAggregate
	err = db.ListAggregate[CaseStatusAggregate](context.Background(), s.ctx, s.db, filters, nil, queryutil.Pagination{}, &statusResults,
		db.WithDBSelect("status, count(*) as count"),
		db.WithDBGroupBy("status"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get status breakdown: %w", err)
	}
	for _, res := range statusResults {
		analytics.StatusBreakdown[res.Status] = res.Count
	}

	// Needs Review
	needsReviewCount, err := db.Count[models.Case](context.Background(), s.ctx, s.db, append(filters, queryutil.Filter{
		Field:    "needs_review",
		Operator: queryutil.OperatorEquals,
		Value:    true,
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to get needs review count: %w", err)
	}
	analytics.NeedsReviewCount = needsReviewCount

	// Case Type Breakdown
	type CaseTypeAggregate struct {
		Type  models.CaseType
		Count int64
	}
	var typeResults []CaseTypeAggregate
	err = db.ListAggregate[CaseTypeAggregate](context.Background(), s.ctx, s.db, filters, nil, queryutil.Pagination{}, &typeResults,
		db.WithDBSelect("type, count(*) as count"),
		db.WithDBGroupBy("type"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get case type breakdown: %w", err)
	}
	for _, res := range typeResults {
		analytics.CaseTypeBreakdown[res.Type] = res.Count
	}

	// Source Breakdown
	type SourceAggregate struct {
		Source models.ReportSource
		Count  int64
	}
	var sourceResults []SourceAggregate
	err = db.ListAggregate[SourceAggregate](context.Background(), s.ctx, s.db, filters, nil, queryutil.Pagination{}, &sourceResults,
		db.WithDBSelect("source, count(*) as count"),
		db.WithDBGroupBy("source"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get source breakdown: %w", err)
	}
	for _, res := range sourceResults {
		analytics.SourceBreakdown[res.Source] = res.Count
	}
	// Get metrics from other services using new metrics methods
	commStats, err := s.communicationSvc.GetCommunicationMetrics(start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get communication metrics: %w", err)
	}
	analytics.CommsMetrics = *commStats

	evidenceStats, err := s.evidenceSvc.GetEvidenceMetrics(start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get evidence metrics: %w", err)
	}
	analytics.EvidenceMetrics = *evidenceStats

	blockStats, err := s.blocklistSvc.GetBlocklistMetrics(start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get blocklist metrics: %w", err)
	}
	analytics.BlocklistMetrics = *blockStats

	// Get resolution trends from view
	var resolutionTrends []models.DailyResolution
	filters = []queryutil.Filter{
		{Field: "resolution_date", Operator: queryutil.OperatorGTE, Value: start},
		{Field: "resolution_date", Operator: queryutil.OperatorLTE, Value: end},
	}

	var total int64
	if err := db.List[models.DailyResolution](
		context.Background(),
		s.ctx,
		s.db,
		filters,
		nil,
		queryutil.Pagination{},
		&resolutionTrends,
		&total,
	); err != nil {
		return nil, fmt.Errorf("failed to get resolution trends: %w", err)
	}

	// Convert to map format with time.Time keys
	analytics.ResolutionTrends = make(map[time.Time]int64)
	for _, trend := range resolutionTrends {
		analytics.ResolutionTrends[trend.ResolutionDate] = trend.ResolvedCount
		analytics.TotalResolved += trend.ResolvedCount
		analytics.TotalResolutionSeconds += trend.AvgResolutionSeconds * float64(trend.ResolvedCount)
	}

	if len(resolutionTrends) > 0 {
		analytics.AvgResolutionSeconds = analytics.TotalResolutionSeconds / float64(analytics.TotalResolved)
	}

	return analytics, nil
}

func (s *CaseServiceDefault) calculateStatusDurations(caseID uint) map[models.CaseStatus]time.Duration {
	durations := make(map[models.CaseStatus]time.Duration)

	var history []models.CaseStatusHistory
	filters := []queryutil.Filter{
		{Field: "case_id", Operator: queryutil.OperatorEquals, Value: caseID},
	}
	sorts := []queryutil.Sort{
		{Field: "changed_at", Order: queryutil.OrderAsc},
	}

	var total int64
	err := db.List[models.CaseStatusHistory](
		context.Background(),
		s.ctx,
		s.db,
		filters,
		sorts,
		queryutil.Pagination{},
		&history,
		&total,
	)
	if err != nil {
		s.logger.Error("Failed to fetch status history",
			zap.Uint("caseID", caseID),
			zap.Error(err))
		return durations
	}

	var prevEntry models.CaseStatusHistory
	for _, entry := range history {
		if prevEntry.ID != 0 {
			duration := entry.ChangedAt.Sub(prevEntry.ChangedAt)
			durations[prevEntry.NewStatus] += duration
		}
		prevEntry = entry
	}

	// Add current status duration if case is still open
	if prevEntry.ID != 0 && prevEntry.NewStatus != models.CaseStatusResolved && prevEntry.NewStatus != models.CaseStatusClosed {
		durations[prevEntry.NewStatus] += time.Since(prevEntry.ChangedAt)
	}

	return durations
}

func (s *CaseServiceDefault) Get7DayAnalytics() (*typesSvc.CaseAnalytics, error) {
	start := time.Now().AddDate(0, 0, -7)
	return s.GetAnalytics([]queryutil.Filter{
		{Field: "created_at", Operator: queryutil.OperatorGTE, Value: start},
	})
}

func (s *CaseServiceDefault) Get30DayAnalytics() (*typesSvc.CaseAnalytics, error) {
	start := time.Now().AddDate(0, 0, -30)
	return s.GetAnalytics([]queryutil.Filter{
		{Field: "created_at", Operator: queryutil.OperatorGTE, Value: start},
	})
}

func (s *CaseServiceDefault) Search(ctx context.Context, query string, filters []queryutil.Filter, pagination queryutil.Pagination) ([]models.Case, int64, error) {
	var cases []models.Case
	var total int64

	err := db.List[models.Case](context.Background(), s.ctx, s.db, filters, nil, pagination, &cases, &total, db.WithSearchQuery[models.Case](query))

	if err != nil {
		s.logger.Error("Search failed", zap.Error(err))
		return nil, 0, fmt.Errorf("search failed: %w", err)
	}

	return cases, total, nil
}
