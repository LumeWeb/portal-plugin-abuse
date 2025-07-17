package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/samber/lo"
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
var _ core.Configurable = (*ScanServiceDefault)(nil)

// NewScanService creates a new scan service instance
func NewScanService() (core.Service, []core.ContextBuilderOption, error) {
	svc := &ScanServiceDefault{}

	options := core.ContextOptions(
		core.ContextWithStartupFunc(func(ctx core.Context) error {
			svc.BaseService.InitializeBaseService(ctx, svc)

			// Get case service dependency
			caseSvc := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
			if caseSvc == nil {
				return fmt.Errorf("case service not available")
			}
			svc.caseSvc = caseSvc

			coreScanSvc := core.GetService[core.ContentScannerService](ctx, core.CONTENT_SCANNER_SERVICE)
			if coreScanSvc == nil {
				return fmt.Errorf("content scanner service not available")
			}
			svc.coreScanSvc = coreScanSvc

			subjectSvc := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE)
			if subjectSvc == nil {
				return fmt.Errorf("subject service not available")
			}
			svc.subjectSvc = subjectSvc

			reporterSvc := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE)
			if reporterSvc == nil {
				return fmt.Errorf("reporter service not available")
			}
			svc.reporterSvc = reporterSvc

			emailSvc := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE)
			if emailSvc == nil {
				return fmt.Errorf("email service not available")
			}
			svc.emailSvc = emailSvc

			// Register abuse scan workflow
			coordinator := core.GetService[core.WorkflowService](ctx, core.WORKFLOW_SERVICE)
			if coordinator == nil {
				return fmt.Errorf("workflow coordinator service not available")
			}

			cronSvc := core.GetService[core.CronService](ctx, core.CRON_SERVICE)
			if cronSvc == nil {
				return fmt.Errorf("cron service not available")
			}

			storageSvc := core.GetService[core.StorageService](ctx, core.STORAGE_SERVICE)
			if storageSvc == nil {
				return fmt.Errorf("storage service not available")
			}

			uploadSvc := core.GetService[core.UploadService](ctx, core.UPLOAD_SERVICE)
			if uploadSvc == nil {
				return fmt.Errorf("upload service not available")
			}

			svc.workflowSvc = coordinator

			cfg := core.GetServiceConfig[*config.ScanConfig](ctx, typesSvc.SCAN_SERVICE)

			clamScanner := scanner.NewClamScanner(ctx, storageSvc, uploadSvc, cfg.ClamAV.Network, cfg.ClamAV.Address, cfg.ClamAV.MaxWorkers)
			if err := coreScanSvc.RegisterScanner(clamScanner); err != nil {
				ctx.Logger().Error("Failed to register ClamAV scanner - continuing without it", zap.Error(err))
			}

			return nil
		}),
	)

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
		return db.HandleDBError(err, "GetByID", "Case", caseID)
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
		return db.HandleDBError(err, "Create", "CaseScan", 0)
	}

	_, err = s.workflowSvc.StartWorkflow(s.ctx.GetContext(), workflow.AbuseScanWorkflowName, core.WithWorkflowStructData(workflow.AbuseScanWorkflowData{
		CaseScanID: scan.ID,
		SubjectID:  scan.SubjectID,
	}, "json"))

	return err
}

// GetScansForCase gets all scans for a case
func (s *ScanServiceDefault) GetScansForCase(caseID uint, pagination queryutil.Pagination) ([]models.CaseScan, int64, error) {
	var scans []models.CaseScan
	var total int64

	filters := []queryutil.CrudFilter{
		queryutil.Equal("case_id", caseID),
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
		return nil, db.HandleDBError(err, "GetScanById", "CaseScan", scanID)
	}

	return &scan, nil
}

func (s *ScanServiceDefault) SaveScanResults(scanID uint, results []*core.ScanResult) error {
	// Get existing scan record
	var scan models.CaseScan
	if err := db.GetByID(context.Background(), s.ctx, s.db, scanID, &scan); err != nil {
		s.logger.Error("Failed to get scan for saving results", zap.Error(err), zap.Uint("scanID", scanID))
		return db.HandleDBError(err, "GetByID", "CaseScan", scanID)
	}

	// Extract scan result IDs from core results
	scanResultIDs := lo.Map(results, func(item *core.ScanResult, _ int) uint64 {
		return item.ID
	})

	// Marshal IDs to JSON for storage
	resultsJSON, err := json.Marshal(scanResultIDs)
	if err != nil {
		s.logger.Error("Failed to marshal scan results", zap.Error(err), zap.Uint("scanID", scanID))
		return db.HandleDBError(err, "Marshal", "CaseScan", scanID)
	}

	// Update scan record with results
	scan.ScanResults = resultsJSON
	if err := db.Update(context.Background(), s.ctx, s.db, &scan); err != nil {
		s.logger.Error("Failed to update scan results", zap.Error(err), zap.Uint("scanID", scanID))
		return db.HandleDBError(err, "Update", "CaseScan", scanID)
	}

	return nil
}

// GetScanResults retrieves results for a specific scan
func (s *ScanServiceDefault) GetScanResults(scanID uint, filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]*core.ScanResult, int64, error) {
	// Get the scan record to get storage key
	var scan models.CaseScan
	if err := db.GetByID(context.Background(), s.ctx, s.db, scanID, &scan); err != nil {
		s.logger.Error("Failed to get scan", zap.Error(err), zap.Uint("scanID", scanID))
		// Return ErrRecordNotFound if the scan itself is not found
		if errors.Is(err, db.ErrRecordNotFound) {
			return nil, 0, db.ErrRecordNotFound // Propagate the specific error
		}
		return nil, 0, db.HandleDBError(err, "GetScanById", "CaseScan", scanID)
	}

	var scanResultIDs []uint
	if err := json.Unmarshal(scan.ScanResults, &scanResultIDs); err != nil {
		s.logger.Error("Failed to unmarshal scan results", zap.Error(err), zap.Uint("scanID", scanID))
		return nil, 0, db.HandleDBError(err, "Unmarshal", "CaseScan", scanID)
	}

	// Get core scanner service
	scannerSvc := core.GetService[core.ContentScannerService](s.ctx, core.CONTENT_SCANNER_SERVICE)
	if scannerSvc == nil {
		s.logger.Error("Content scanner service not available")
		return nil, 0, errors.New("content scanner service not available") // Return a standard error
	}

	// Fetch all core scan results based on IDs
	// This is the potentially inefficient part for large result sets
	allCoreResults := make([]*core.ScanResult, 0, len(scanResultIDs))
	for _, scanResultID := range scanResultIDs {
		coreResult, err := scannerSvc.GetScanResultById(context.Background(), scanResultID)
		if err != nil {
			// Log the error but continue if a single result fetch fails
			s.logger.Error("Failed to get scan result from core service", zap.Error(err), zap.Uint("scanID", scanID), zap.Uint("scanResultID", scanResultID))
			// Decide how to handle individual fetch errors: skip, return error, etc.
			// For now, we'll skip the failed result.
			continue
		}
		allCoreResults = append(allCoreResults, coreResult)
	}

	// --- Apply Filtering in Memory ---
	// queryutil.ApplyFilters is designed for GORM queries.
	// We need to manually filter the slice.
	// This requires implementing filtering logic based on the `filters` slice.
	// For simplicity in this example, we'll assume basic filtering on `core.ScanResult` fields.
	// You would need to implement `applyFiltersToScanResults` based on your DTO/Model fields.
	filteredResults := applyFiltersToScanResults(allCoreResults, filters)

	// --- Apply Sorting in Memory ---
	// queryutil.ApplySort is designed for GORM queries.
	// We need to manually sort the slice.
	// This requires implementing sorting logic based on the `sorts` slice.
	// You would need to implement `applySortToScanResults` based on your DTO/Model fields.
	sortedResults := applySortToScanResults(filteredResults, sorts)

	// --- Apply Pagination in Memory ---
	totalCount := int64(len(sortedResults))
	start := pagination.GetOffset()
	end := start + pagination.GetLimit()

	// Ensure start and end are within bounds
	if start < 0 {
		start = 0
	}
	if end > int(totalCount) || pagination.GetLimit() == 0 { // Handle limit 0 case (no limit)
		end = int(totalCount)
	}
	if start > int(totalCount) {
		start = int(totalCount) // Empty slice if start is beyond total
	}
	if end < start {
		end = start // Empty slice if end is before start
	}

	paginatedResults := sortedResults[start:end]

	return paginatedResults, totalCount, nil
}

// applyFiltersToScanResults is a placeholder function.
// You need to implement the actual filtering logic based on the `filters` slice
// and the fields available in the `core.ScanResult` struct.
func applyFiltersToScanResults(results []*core.ScanResult, filters []queryutil.CrudFilter) []*core.ScanResult {
	if len(filters) == 0 {
		return results // No filters to apply
	}

	// Example: Implement filtering logic here.
	// This is a simplified example and needs to be expanded
	// to handle different filter types (Equal, Like, etc.) and fields.
	filtered := make([]*core.ScanResult, 0, len(results))
	for _, result := range results {
		// Assume a simple filter check for demonstration
		// You would iterate through `filters` and check conditions
		// For example:
		// if queryutil.MatchesFilters(result, filters) { // You'd need a helper like this
		//     filtered = append(filtered, result)
		// }
		// Since we don't have a generic `MatchesFilters` helper for arbitrary structs,
		// you'd need to write specific logic based on expected filter fields.
		// For now, returning all results as a placeholder.
		filtered = append(filtered, result)
	}

	return filtered
}

// applySortToScanResults is a placeholder function.
// You need to implement the actual sorting logic based on the `sorts` slice
// and the fields available in the `core.ScanResult` struct.
func applySortToScanResults(results []*core.ScanResult, sorts []queryutil.Sort) []*core.ScanResult {
	if len(sorts) == 0 {
		return results // No sorting to apply
	}

	// Example: Implement sorting logic here.
	// This is a simplified example and needs to be expanded
	// to handle different sort fields and orders (asc/desc).
	// You would typically use `sort.Slice` or similar.
	// For now, returning results unsorted as a placeholder.

	return results
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
	siteURL := core.GetService[core.HTTPService](s.ctx, core.HTTP_SERVICE).APISubdomain("admin", true)

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
		// Handle DB errors specifically if needed, otherwise return the raw error
		return db.HandleDBError(err, "GetByID", "CaseScan", scanID)
	}

	// Only trigger notifications if status is changing to a terminal state
	if scan.Status != status && (status == models.ScanStatusFlagged || status == models.ScanStatusError) {
		// Call GetScanResults with empty filters, sorts, and pagination to get all results
		results, _, err := s.GetScanResults(scanID, nil, nil, queryutil.Pagination{})
		if err != nil {
			s.logger.Error("Failed to get scan results for notification",
				zap.Uint("scanID", scanID),
				zap.Error(err))
			// Continue processing the status update even if getting results for notification fails
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
	// Use db.Update with the context and logger
	if err := db.Update(context.Background(), s.ctx, s.db, &scan); err != nil {
		return db.HandleDBError(err, "Update", "CaseScan", scanID)
	}

	return nil
}
