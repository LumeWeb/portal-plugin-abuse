package service

import (
	"context"
	"fmt"
	"github.com/samber/lo"
	"go.lumeweb.com/portal-plugin-abuse/internal"
	"go.lumeweb.com/portal-plugin-abuse/internal/util"
	"go.lumeweb.com/portal/event"
	"time"

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

// NewCaseService creates a new case service
func NewCaseService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &CaseServiceDefault{}
	return svc, []core.ContextBuilderOption{
		core.ContextWithStartupFunc(func(ctx core.Context) error {
			svc.BaseService.InitializeBaseService(ctx, svc)

			httpSvc := core.GetService[core.HTTPService](ctx, core.HTTP_SERVICE)
			if httpSvc == nil {
				return fmt.Errorf("http service not available")
			}
			svc.httpSvc = httpSvc

			core.Listen(ctx, event.EVENT_BOOT_COMPLETE, func(e *core.CoreEvent[event.BootCompleteEvent]) error {
				// Get required services
				reporterSvc := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE)
				if reporterSvc == nil {
					return fmt.Errorf("reporter service not available")
				}
				svc.reporterSvc = reporterSvc

				subjectSvc := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE)
				if subjectSvc == nil {
					return fmt.Errorf("subject service not available")
				}
				svc.subjectSvc = subjectSvc

				emailSvc := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE)
				if emailSvc == nil {
					return fmt.Errorf("email service not available")
				}
				svc.emailSvc = emailSvc

				tokenSvc := core.GetService[typesSvc.TokenService](ctx, typesSvc.TOKEN_SERVICE)
				if tokenSvc == nil {
					return fmt.Errorf("token service not available")
				}
				svc.tokenSvc = tokenSvc

				communicationSvc := core.GetService[typesSvc.CommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
				if communicationSvc == nil {
					return fmt.Errorf("communication service not available")
				}
				svc.communicationSvc = communicationSvc

				evidenceSvc := core.GetService[typesSvc.EvidenceService](ctx, typesSvc.EVIDENCE_SERVICE)
				if evidenceSvc == nil {
					return fmt.Errorf("evidence service not available")
				}
				svc.evidenceSvc = evidenceSvc

				blocklistSvc := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
				if blocklistSvc == nil {
					return fmt.Errorf("blocklist service not available")
				}
				svc.blocklistSvc = blocklistSvc

				return nil
			})

			return nil
		}),
	}, nil
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
	threadID := s.emailSvc.GenerateCaseThreadID(caseModel.ReferenceNumber)

	siteURL := s.httpSvc.APISubdomain(internal.PLUGIN_NAME, true)

	// Generate access token for the reporter (valid for 90 days)
	accessToken, _, err := s.tokenSvc.GenerateToken(caseID, reporter.ID, 90)
	if err != nil {
		s.logger.Error("Failed to generate access token", zap.Error(err), zap.Uint("caseID", caseID), zap.Uint("reporterID", reporter.ID))
	} else {
		// Send single case access email with all needed information
		templateData := core.MailerTemplateData{
			"CaseID":        caseModel.ReferenceNumber,
			"Reference":     caseModel.ReferenceNumber,
			"ReporterName":  reporter.Name,
			"ReporterEmail": reporter.Email,
			"SubjectType":   subject.Type,
			"SubjectHash":   subject.Identifier,
			"AccessURL":     fmt.Sprintf("%s/case/access?token=%s", siteURL, accessToken),
			"ExpiresIn":     "90 days",
			"PortalName":    s.ctx.Config().Config().Core.PortalName,
			"CreatedDate":   caseModel.CreatedAt.Format("January 2, 2006"),
			"CaseType":      string(caseModel.Type),
			"CaseStatus":    string(caseModel.Status),
			"ReplyTo":       threadID,
		}

		// Add high-priority warning if needed
		if caseModel.Priority == models.CasePriorityHigh {
			templateData["HighPriorityWarning"] = true
			templateData["PriorityReason"] = "This case has been marked as high priority due to the nature of the report"
		}

		// Validate template requirements
		requiredFields := []string{"CaseID", "ReporterName", "PortalName", "AccessURL", "CreatedDate"}
		if err := s.ValidateTemplateData("case_access", templateData, requiredFields); err != nil {
			s.logger.Error("Invalid template data for case access notification",
				zap.Uint("caseID", caseModel.ID),
				zap.Error(err))
			return fmt.Errorf("failed to validate template data: %w", err)
		}

		if err := s.emailSvc.SendTemplatedEmail(
			[]string{reporter.Email},
			"case_access",
			templateData,
		); err != nil {
			return fmt.Errorf("failed to send access notification: %w", err)
		}
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
	threadID := s.emailSvc.GenerateCaseThreadID(caseModel.ReferenceNumber)

	siteURL := core.GetService[core.HTTPService](s.ctx, core.HTTP_SERVICE).APISubdomain("admin", true)

	templateData := core.MailerTemplateData{
		"Reference":   caseModel.ReferenceNumber,
		"OldStatus":   oldStatus,
		"NewStatus":   newStatus,
		"CaseType":    caseModel.Type,
		"PortalName":  s.ctx.Config().Config().Core.PortalName,
		"UpdatedDate": time.Now().Format("January 2, 2006 15:04"),
		"CreatedDate": caseModel.CreatedAt.Format("January 2, 2006 15:04"),
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
		return nil, db.HandleDBError(err, "Validate", "Case", 0)
	}

	if err := db.Create(context.Background(), s.ctx, s.db, caseData); err != nil {
		s.logger.Error("Failed to create case", zap.Error(err))
		return nil, db.HandleDBError(err, "Create", "Case", 0)
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
		if db.IsRecordNotFound(err) {
			return nil, db.ErrRecordNotFound
		}
		s.logger.Error("Failed to get case by ID", zap.Error(err), zap.Uint("caseID", id))
		return nil, db.HandleDBError(err, "GetByID", "Case", id)
	}
	return &caseModel, nil
}

// List returns a list of cases with filtering, sorting and pagination
func (s *CaseServiceDefault) List(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Case, int64, error) {
	var cases []models.Case
	var total int64

	if err := db.List[models.Case](context.Background(), s.ctx, s.db, filters, sorts, pagination, &cases, &total); err != nil {
		s.logger.Error("Failed to list cases", zap.Error(err))
		return nil, 0, db.HandleDBError(err, "List", "Case", 0)
	}

	return cases, total, nil
}

// Update updates an existing case
func (s *CaseServiceDefault) Update(caseModel *models.Case) error {
	if err := caseModel.Validate(); err != nil {
		s.logger.Error("Invalid case data", zap.Error(err), zap.Uint("caseID", caseModel.ID))
		return db.HandleDBError(err, "Validate", "Case", caseModel.ID)
	}

	if err := db.Update(context.Background(), s.ctx, s.db, caseModel); err != nil {
		s.logger.Error("Failed to update case", zap.Error(err), zap.Uint("caseID", caseModel.ID))
		return db.HandleDBError(err, "Update", "Case", caseModel.ID)
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
		if db.IsRecordNotFound(err) {
			return nil, db.ErrRecordNotFound
		}
		s.logger.Error("Failed to fetch case by reference", zap.Error(err), zap.String("reference", reference))
		return nil, db.HandleDBError(err, "GetByReference", "Case", 0)
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
		return nil, db.ErrInvalidInput
	}

	return caseModel, nil
}

// GetAnalytics returns aggregated case metrics with filters
func (s *CaseServiceDefault) GetAnalytics(filters []queryutil.CrudFilter) (*typesSvc.CaseAnalytics, error) {
	var start, end time.Time
	now := time.Now()

	// Handle time range filters
	filters, err := util.ApplyTimeRangeFilters(filters, "created_at")
	if err != nil {
		return nil, err
	}

	// Default to last 30 days if no time range specified
	if queryutil.FindFilterWithOperator(filters, "created_at", queryutil.OpGte) == nil {
		filters = append(filters, queryutil.FieldGte("created_at", now.AddDate(0, 0, -30)))
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
	var statusResults []models.CaseStatusBreakdown
	var statusTotal int64
	err = db.List[models.CaseStatusBreakdown](context.Background(), s.ctx, s.db, filters, nil, queryutil.Pagination{}, &statusResults, &statusTotal)
	if err != nil {
		return nil, fmt.Errorf("failed to get status breakdown: %w", err)
	}
	for _, res := range statusResults {
		analytics.StatusBreakdown[res.Status] += res.Count
	}

	// Needs Review
	needsReviewCount, err := db.Count[models.Case](context.Background(), s.ctx, s.db, append(filters, queryutil.FieldEqual("needs_review", true)))
	if err != nil {
		return nil, fmt.Errorf("failed to get needs review count: %w", err)
	}
	analytics.NeedsReviewCount = needsReviewCount

	// Case Type Breakdown
	var typeResults []models.CaseTypeBreakdown
	var typeTotal int64
	err = db.List[models.CaseTypeBreakdown](context.Background(), s.ctx, s.db, filters, nil, queryutil.Pagination{}, &typeResults, &typeTotal)
	if err != nil {
		return nil, fmt.Errorf("failed to get case type breakdown: %w", err)
	}
	for _, res := range typeResults {
		analytics.CaseTypeBreakdown[res.Type] += res.Count
	}

	// Source Breakdown
	var sourceResults []models.CaseSourceBreakdown
	var sourceTotal int64
	err = db.List[models.CaseSourceBreakdown](context.Background(), s.ctx, s.db, filters, nil, queryutil.Pagination{}, &sourceResults, &sourceTotal)
	if err != nil {
		return nil, fmt.Errorf("failed to get source breakdown: %w", err)
	}
	for _, res := range sourceResults {
		analytics.SourceBreakdown[res.Source] += res.Count
	}
	// Get metrics from other services using new metrics methods
	commStats, err := s.communicationSvc.GetCommunicationMetrics(start, end)
	if err != nil {
		return nil, db.HandleDBError(err, "GetCommunicationMetrics", "Communication", 0)
	}
	analytics.CommsMetrics = *commStats

	evidenceStats, err := s.evidenceSvc.GetEvidenceMetrics(start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to get evidence metrics: %w", err)
	}
	analytics.EvidenceMetrics = *evidenceStats

	blockStats, err := s.blocklistSvc.GetBlocklistMetrics(filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get blocklist metrics: %w", err)
	}
	analytics.BlocklistMetrics = *blockStats

	// Get resolution trends from view
	var resolutionTrends []models.CaseDailyResolution
	filters = queryutil.Filters(
		queryutil.FieldGte("resolution_date", start),
		queryutil.FieldLte("resolution_date", end),
	)

	var total int64
	if err := db.List[models.CaseDailyResolution](
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
		date, err := trend.GetDate()
		if err != nil {
			s.logger.Warn("Failed to parse resolution date, skipping",
				zap.String("date", trend.ResolutionDate),
				zap.Error(err))
			continue
		}
		analytics.ResolutionTrends[date] = trend.ResolvedCount
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

	var statusDurations []models.CaseStatusDuration
	filters := queryutil.Filters(
		queryutil.FieldEqual("case_id", caseID),
	)

	var total int64
	err := db.List[models.CaseStatusDuration](
		context.Background(),
		s.ctx,
		s.db,
		filters,
		nil,
		queryutil.Pagination{},
		&statusDurations,
		&total,
	)
	if err != nil {
		s.logger.Error("Failed to fetch status durations",
			zap.Uint("caseID", caseID),
			zap.Error(err))
		return durations
	}

	for _, sd := range statusDurations {
		durations[sd.NewStatus] = sd.Duration()
	}

	return durations
}

func (s *CaseServiceDefault) Get7DayAnalytics() (*typesSvc.CaseAnalytics, error) {
	start := time.Now().AddDate(0, 0, -7)
	return s.GetAnalytics(queryutil.Filters(
		queryutil.FieldGte("created_at", start),
	))
}

func (s *CaseServiceDefault) Get30DayAnalytics() (*typesSvc.CaseAnalytics, error) {
	start := time.Now().AddDate(0, 0, -30)
	return s.GetAnalytics(queryutil.Filters(
		queryutil.FieldGte("created_at", start),
	))
}

func (s *CaseServiceDefault) Get24HourAnalytics() (*typesSvc.CaseAnalytics, error) {
	start := time.Now().Add(-24 * time.Hour)
	return s.GetAnalytics(queryutil.Filters(
		queryutil.FieldGte("created_at", start),
	))
}

func (s *CaseServiceDefault) GetTypeSourceMatrix(timeRange string, filters []queryutil.CrudFilter) ([]models.CaseTypeSourceBreakdown, error) {
	// Calculate time range
	start, end, err := util.ParseTimeRange(timeRange)
	if err != nil {
		return nil, util.ErrInvalidTimeRange
	}

	s.logger.Debug("GetTypeSourceMatrix time range",
		zap.String("timeRange", timeRange),
		zap.Time("start", start),
		zap.Time("end", end))

	s.logger.Debug("GetTypeSourceMatrix filters",
		zap.Any("filters", filters))

	var results []models.CaseTypeSourceBreakdown
	var total int64
	err = db.List[models.CaseTypeSourceBreakdown](
		context.Background(),
		s.ctx,
		s.db,
		filters,
		nil,
		queryutil.Pagination{},
		&results,
		&total,
	)

	if err != nil {
		s.logger.Error("Failed to get case type source matrix",
			zap.Error(err),
			zap.String("timeRange", timeRange))
		return nil, fmt.Errorf("failed to get case type source matrix: %w", err)
	}

	s.logger.Debug("GetTypeSourceMatrix results",
		zap.Int("count", len(results)),
		zap.Any("results", results))

	return results, nil
}

func (s *CaseServiceDefault) GetStatusFlowData(filters []queryutil.CrudFilter) (*typesSvc.StatusFlowGraph, error) {
	// Remove the time_range filter since it's a meta-filter and not a column
	filters = lo.Filter(filters, func(f queryutil.CrudFilter, _ int) bool {
		return f.GetField() != "time_range"
	})

	// Handle time range filters
	cleanedFilters, err := util.ApplyTimeRangeFilters(filters, "changed_at")
	if err != nil {
		return nil, err
	}
	filters = cleanedFilters

	var transitions []models.CaseStatusTransition
	var total int64

	if err := db.List[models.CaseStatusTransition](
		context.Background(),
		s.ctx,
		s.db,
		filters,
		nil,
		queryutil.Pagination{},
		&transitions,
		&total,
	); err != nil {
		return nil, fmt.Errorf("failed to get status transitions: %w", err)
	}

	// Filter out transitions with same from/to status
	filteredTransitions := lo.Filter(transitions, func(t models.CaseStatusTransition, _ int) bool {
		return t.FromStatus != t.ToStatus
	})

	// Get all unique statuses and convert to nodes
	nodes := lo.Map(
		lo.Uniq(
			lo.FlatMap(filteredTransitions, func(t models.CaseStatusTransition, _ int) []models.CaseStatus {
				return []models.CaseStatus{t.FromStatus, t.ToStatus}
			}),
		),
		func(status models.CaseStatus, _ int) typesSvc.StatusFlowNode {
			return typesSvc.StatusFlowNode{Name: string(status)}
		},
	)

	// Group and sum transitions
	type transitionKey struct{ From, To models.CaseStatus }
	links := lo.MapToSlice(
		lo.GroupBy(filteredTransitions, func(t models.CaseStatusTransition) transitionKey {
			return transitionKey{t.FromStatus, t.ToStatus}
		}),
		func(key transitionKey, group []models.CaseStatusTransition) typesSvc.StatusFlowLink {
			return typesSvc.StatusFlowLink{
				Source: string(key.From),
				Target: string(key.To),
				Value:  lo.SumBy(group, func(t models.CaseStatusTransition) int64 { return t.TransitionCount }),
			}
		},
	)

	response := &typesSvc.StatusFlowGraph{
		Nodes: nodes,
		Links: links,
	}

	return response, nil
}

func (s *CaseServiceDefault) Search(ctx context.Context, query string, filters []queryutil.CrudFilter, pagination queryutil.Pagination) ([]models.Case, int64, error) {
	var cases []models.Case
	var total int64

	err := db.List[models.Case](context.Background(), s.ctx, s.db, filters, nil, pagination, &cases, &total, db.WithSearchQuery[models.Case](query))

	if err != nil {
		s.logger.Error("Search failed", zap.Error(err))
		return nil, 0, db.HandleDBError(err, "Search", "Case", 0)
	}

	return cases, total, nil
}

func (s *CaseServiceDefault) GetTimeSeriesMetrics(metric string, timeRange string, filters []queryutil.CrudFilter) ([]int64, error) {
	// Calculate time range
	_, _, err := util.ParseTimeRange(timeRange)
	if err != nil {
		return nil, util.ErrInvalidTimeRange
	}

	var results []models.CaseDailyMetrics
	var total int64
	err = db.List[models.CaseDailyMetrics](
		context.Background(),
		s.ctx,
		s.db,
		filters,
		[]queryutil.Sort{{Field: "metric_date", Order: queryutil.OrderAsc}},
		queryutil.Pagination{}, // No pagination needed for time series
		&results,
		&total,
		func(_db *gorm.DB) *gorm.DB {
			switch metric {
			case "open_cases":
				return _db.Select("metric_date, open_cases")
			case "new_cases":
				return _db.Select("metric_date, new_cases")
			case "resolved_cases":
				return _db.Select("metric_date, resolved_cases")
			default:
				return _db
			}
		},
	)

	if err != nil {
		return nil, db.HandleDBError(err, "List", "CaseDailyMetrics", 0)
	}

	// Extract requested metric
	data := make([]int64, len(results))
	for i, r := range results {
		switch metric {
		case "open_cases":
			data[i] = r.OpenCases
		case "new_cases":
			data[i] = r.NewCases
		case "resolved_cases":
			data[i] = r.ResolvedCases
		default:
			return nil, typesSvc.ErrInvalidMetric
		}
	}

	return data, nil
}
