package service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/samber/lo"
	"go.lumeweb.com/portal-plugin-abuse/internal"
	"go.lumeweb.com/portal-plugin-abuse/internal/config"
	"go.lumeweb.com/portal-plugin-abuse/internal/pkg/scanner"
	"go.lumeweb.com/portal-plugin-abuse/internal/workflow"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"time"

	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/queryutil"
)

type ScanServiceDefault struct {
	BaseService
	caseSvc     typesSvc.CaseService
	workflowSvc core.WorkflowService
	coreScanSvc core.ContentScannerService
	subjectSvc  typesSvc.SubjectService
	reporterSvc typesSvc.ReporterService
	emailSvc    typesSvc.EmailService
}

func (s *ScanServiceDefault) Config() (any, error) {
	return &config.ScanConfig{}, nil
}

var _ typesSvc.ScanService = (*ScanServiceDefault)(nil)

// NewScanService creates a new scan service instance
func NewScanService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &ScanServiceDefault{}

	options := []core.ContextBuilderOption{
		func(ctx core.Context) (core.Context, error) {
			svc.BaseService.InitializeBaseService(ctx, svc)

			// Get case service dependency
			caseSvc := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
			if caseSvc == nil {
				return ctx, fmt.Errorf("case service not available")
			}
			svc.caseSvc = caseSvc

			coreScanSvc := core.GetService[core.ContentScannerService](ctx, core.CONTENT_SCANNER_SERVICE)
			if coreScanSvc == nil {
				return ctx, fmt.Errorf("content scanner service not available")
			}
			svc.coreScanSvc = coreScanSvc

			subjectSvc := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE)
			if subjectSvc == nil {
				return ctx, fmt.Errorf("subject service not available")
			}
			svc.subjectSvc = subjectSvc

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

			// Register abuse scan workflow
			coordinator := core.GetService[core.WorkflowService](ctx, core.WORKFLOW_SERVICE)
			if coordinator == nil {
				return ctx, fmt.Errorf("workflow coordinator service not available")
			}

			cronSvc := core.GetService[core.CronService](ctx, core.CRON_SERVICE)
			if cronSvc == nil {
				return ctx, fmt.Errorf("cron service not available")
			}

			abuseWorkflow := workflow.NewAbuseScanWorkflow(ctx, coordinator, cronSvc, coreScanSvc, svc)
			if err := abuseWorkflow.Register(); err != nil {
				return ctx, fmt.Errorf("failed to register abuse scan workflow: %w", err)
			}

			storageSvc := core.GetService[core.StorageService](ctx, core.STORAGE_SERVICE)
			if storageSvc == nil {
				return ctx, fmt.Errorf("storage service not available")
			}

			uploadSvc := core.GetService[core.UploadService](ctx, core.UPLOAD_SERVICE)
			if uploadSvc == nil {
				return ctx, fmt.Errorf("upload service not available")
			}

			svc.workflowSvc = coordinator

			cfg := ctx.Config().GetService(typesSvc.SCAN_SERVICE).(*config.ScanConfig)

			err := coreScanSvc.RegisterScanner(scanner.NewClamScanner(ctx, storageSvc, uploadSvc, cfg.ClamAV.Network, cfg.ClamAV.Address, cfg.ClamAV.MaxWorkers))
			if err != nil {
				return nil, err
			}

			return ctx, nil
		},
	}

	return svc, options, nil
}

func (s *ScanServiceDefault) ID() string {
	return typesSvc.SCAN_SERVICE
}

// CreateScanRequest creates a scan request for an existing subject
func (s *ScanServiceDefault) CreateScanRequest(caseID uint) error {
	// Verify case exists
	_case, err := s.caseSvc.GetByID(caseID)
	if err != nil {
		return fmt.Errorf("case not found: %w", err)
	}

	scan := &models.CaseScan{
		CaseID:       caseID,
		SubjectID:    _case.SubjectID,
		Status:       models.ScanStatusPending,
		ScheduledFor: time.Now(),
	}

	_ = scan

	// Save scan first before starting workflow
	if err := db.Create(context.Background(), s.ctx, s.db, scan); err != nil {
		return fmt.Errorf("failed to create scan record: %w", err)
	}

	_, err = s.workflowSvc.StartWorkflow(s.ctx.GetContext(), workflow.AbuseScanWorkflowName, &workflow.AbuseScanWorkflowInitData{
		CaseScanID: scan.ID,
		SubjectID:  scan.SubjectID,
	})

	return err
}

// GetScansForCase gets all scans for a case
func (s *ScanServiceDefault) GetScansForCase(caseID uint, pagination queryutil.Pagination) ([]models.CaseScan, int64, error) {
	var scans []models.CaseScan
	var total int64

	filters := []queryutil.Filter{
		{Field: "case_id", Operator: "=", Value: caseID},
	}

	err := db.List[models.CaseScan](
		context.Background(),
		s.ctx,
		s.db,
		filters,
		[]queryutil.Sort{{Field: "created_at", Order: queryutil.OrderDesc}},
		pagination,
		&scans,
		&total,
	)

	return scans, total, err
}

// GetScanById retrieves a scan by its ID along with total results count
func (s *ScanServiceDefault) GetScanById(scanID uint) (*models.CaseScan, error) {
	var scan models.CaseScan
	if err := db.GetByID(context.Background(), s.ctx, s.db, scanID, &scan); err != nil {
		return nil, err
	}

	return &scan, nil
}

func (s *ScanServiceDefault) SaveScanResults(scanID uint, results []*core.ScanResult) error {
	// Get existing scan record
	var scan models.CaseScan
	if err := db.GetByID(context.Background(), s.ctx, s.db, scanID, &scan); err != nil {
		s.logger.Error("Failed to get scan for saving results", zap.Error(err), zap.Uint("scanID", scanID))
		return fmt.Errorf("failed to get scan: %w", err)
	}

	// Extract scan result IDs from core results
	scanResultIDs := lo.Map(results, func(item *core.ScanResult, _ int) uint {
		return uint(item.ID)
	})

	// Marshal IDs to JSON for storage
	resultsJSON, err := json.Marshal(scanResultIDs)
	if err != nil {
		s.logger.Error("Failed to marshal scan results", zap.Error(err), zap.Uint("scanID", scanID))
		return fmt.Errorf("failed to marshal scan results: %w", err)
	}

	// Update scan record with results
	scan.ScanResults = resultsJSON
	if err := db.Update(context.Background(), s.ctx, s.db, &scan); err != nil {
		s.logger.Error("Failed to update scan results", zap.Error(err), zap.Uint("scanID", scanID))
		return fmt.Errorf("failed to update scan results: %w", err)
	}

	return nil
}

// GetScanResults retrieves results for a specific scan
func (s *ScanServiceDefault) GetScanResults(scanID uint) ([]*core.ScanResult, error) {
	// Get the scan record to get storage key
	var scan models.CaseScan
	if err := db.GetByID(context.Background(), s.ctx, s.db, scanID, &scan); err != nil {
		s.logger.Error("Failed to get scan", zap.Error(err), zap.Uint("scanID", scanID))
		return nil, fmt.Errorf("failed to get scan: %w", err)
	}

	var scanResultIDs []uint
	if err := json.Unmarshal(scan.ScanResults, &scanResultIDs); err != nil {
		s.logger.Error("Failed to unmarshal scan results", zap.Error(err), zap.Uint("scanID", scanID))
		return nil, fmt.Errorf("failed to unmarshal scan results: %w", err)
	}

	// Get core scanner service
	scannerSvc := core.GetService[core.ContentScannerService](s.ctx, core.CONTENT_SCANNER_SERVICE)
	if scannerSvc == nil {
		s.logger.Error("Content scanner service not available")
		return nil, fmt.Errorf("content scanner service not available")
	}

	subjectSvc := core.GetService[typesSvc.SubjectService](s.ctx, typesSvc.SUBJECT_SERVICE)
	if subjectSvc == nil {
		s.logger.Error("Subject service not available")
		return nil, fmt.Errorf("subject service not available")
	}

	coreResults := make([]*core.ScanResult, 0, len(scanResultIDs))
	for _, scanResultID := range scanResultIDs {
		coreResult, err := scannerSvc.GetScanResultById(context.Background(), scanResultID)
		if err != nil {
			s.logger.Error("Failed to get scan results from core service", zap.Error(err), zap.Uint("scanID", scanID), zap.Uint("scanResultID", scanResultID))
			return nil, fmt.Errorf("failed to get scan result from core service: %w", err)
		}
		coreResults = append(coreResults, coreResult)
	}

	return coreResults, nil
}

// UpdateScanStatus updates the status of a scan
func (s *ScanServiceDefault) notifyScanFailure(scanID uint, failedScans []*core.ScanResult) {
	// Get scan details
	scan, err := s.GetScanById(scanID)
	if err != nil {
		s.logger.Error("Failed to get scan for failure notification",
			zap.Uint("scanID", scanID),
			zap.Error(err))
		return
	}

	// Get associated case
	caseModel, err := s.caseSvc.GetByID(scan.CaseID)
	if err != nil {
		s.logger.Error("Failed to get case for scan failure notification",
			zap.Uint("caseID", scan.CaseID),
			zap.Error(err))
		return
	}

	// Get reporter
	reporter, err := s.reporterSvc.GetByID(caseModel.ReporterID)
	if err != nil {
		s.logger.Error("Failed to get reporter for scan failure notification",
			zap.Uint("reporterID", caseModel.ReporterID),
			zap.Error(err))
		return
	}

	// Prepare notification data
	siteURL := core.GetService[core.HTTPService](s.ctx, core.HTTP_SERVICE).APISubdomain(internal.PLUGIN_NAME, true)

	templateData := core.MailerTemplateData{
		"CaseID":       caseModel.ReferenceNumber,
		"ReporterName": reporter.Name,
		"PortalName":   s.ctx.Config().Config().Core.PortalName,
		"FailedCount":  len(failedScans),
		"ScanDate":     time.Now().Format("January 2, 2006 15:04"),
		"CaseURL":      fmt.Sprintf("%s/case/%s", siteURL, caseModel.ReferenceNumber),
		"CaseType":     string(caseModel.Type),
		"CaseStatus":   string(caseModel.Status),
	}

	// Validate template requirements
	requiredFields := []string{"CaseID", "ReporterName", "PortalName", "FailedCount", "ScanDate", "CaseURL"}
	if err := s.ValidateTemplateData("scan_failed", templateData, requiredFields); err != nil {
		s.logger.Error("Invalid template data for scan failure notification",
			zap.Uint("scanID", scanID),
			zap.Error(err))
		return
	}

	// Send notification
	err = s.emailSvc.SendTemplatedEmail(
		[]string{reporter.Email},
		"scan_failed",
		templateData,
	)

	if err != nil {
		s.logger.Error("Failed to send scan failure notification",
			zap.Uint("scanID", scanID),
			zap.Error(err))
	}
}

func (s *ScanServiceDefault) UpdateScanStatus(scanID uint, status models.ScanStatus) error {
	var scan models.CaseScan
	if err := db.GetByID(context.Background(), s.ctx, s.db, scanID, &scan); err != nil {
		return err
	}

	// Only trigger notifications if status is changing to a terminal state
	if scan.Status != status && (status == models.ScanStatusFlagged || status == models.ScanStatusError) {
		results, err := s.GetScanResults(scanID)
		if err != nil {
			s.logger.Error("Failed to get scan results for notification",
				zap.Uint("scanID", scanID),
				zap.Error(err))
		} else {
			failedScans := lo.Filter(results, func(item *core.ScanResult, _ int) bool {
				return !item.Passed
			})

			if len(failedScans) > 0 {
				go s.notifyScanFailure(scanID, failedScans)
			}
		}
	}

	scan.Status = status
	return db.Update(context.Background(), s.ctx, s.db, &scan)
}
