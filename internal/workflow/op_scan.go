package workflow

import (
	"context"
	"errors"
	"fmt"
	"github.com/samber/lo"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	pluginModels "go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	svcTypes "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/db/models"
	"go.lumeweb.com/queryutil"
	"go.uber.org/zap"
	"time"
)

type ScanOperationHandler struct {
	core.OperationHelper
	ctx         core.Context
	logger      *core.Logger
	cron        core.CronService
	coreScanSvc core.ContentScannerService
	scanSvc     svcTypes.ScanService
}

func (h *ScanOperationHandler) ValidateRequest(_ context.Context, _ *models.Request) error {

	// Noop
	return nil
}

func (h *ScanOperationHandler) Execute(_ context.Context, req *models.Request) error {
	logger := h.Logger()

	var wfData AbuseScanWorkflowData

	err := h.StructuredWorkflowData(req.ID, &wfData)
	if err != nil {
		return err
	}

	logger.Info("Starting abuse scan task",
		zap.Uint("caseScanID", wfData.CaseScanID),
		zap.Uint("subjectID", wfData.SubjectID), zap.Uint("requestID", req.ID))

	// Get the scan service
	scanSvc := core.GetService[svcTypes.ScanService](h.Context(), svcTypes.SCAN_SERVICE)
	if scanSvc == nil {
		logger.Error("Scan service not available")
		return fmt.Errorf("scan service not available")
	}

	// Get content scanner service from the portal core
	contentScannerSvc := core.GetService[core.ContentScannerService](h.Context(), core.CONTENT_SCANNER_SERVICE)
	if contentScannerSvc == nil {
		logger.Error("Content scanner service not available")
		// Update scan status to error
		updateErr := scanSvc.UpdateScanStatus(wfData.CaseScanID, pluginModels.ScanStatusError)
		if updateErr != nil {
			logger.Error("Failed to update scan status", zap.Error(updateErr))
		}
		return fmt.Errorf("content scanner service not available")
	}

	// Get the subject service
	subjectSvc := core.GetService[svcTypes.SubjectService](h.Context(), svcTypes.SUBJECT_SERVICE)
	if subjectSvc == nil {
		logger.Error("Subject service not available")
		// Update scan status to error
		updateErr := scanSvc.UpdateScanStatus(wfData.CaseScanID, pluginModels.ScanStatusError)
		if updateErr != nil {
			logger.Error("Failed to update scan status", zap.Error(updateErr))
		}
		return fmt.Errorf("subject service not available")
	}

	// Update scan status to scanning
	if err = scanSvc.UpdateScanStatus(wfData.CaseScanID, pluginModels.ScanStatusScanning); err != nil {
		logger.Error("Failed to update scan status", zap.Error(err))
		return err
	}

	subject, err := subjectSvc.GetByID(wfData.SubjectID)
	if err != nil {
		return err
	}

	hash := core.NewStorageHashFromRawMultihash(subject.Identifier)

	results, err := contentScannerSvc.ScanContent(h.Context(), hash)
	if err != nil {
		return err
	}

	// Process scan results first
	if err := processScanResults(scanSvc, req.ID, &wfData, results, h.Context()); err != nil {
		// If processing failed, ensure status is marked as error
		_ = scanSvc.UpdateScanStatus(wfData.CaseScanID, pluginModels.ScanStatusError)
		return err
	}
	return nil
}

func (h *ScanOperationHandler) GetStatus(_ context.Context, req *models.Request) (*core.RequestStatus, error) {
	var status core.RequestStatus

	status.UpdatedAt = time.Now()
	status.Message = "Scan not started"

	var wfData AbuseScanWorkflowData
	err := h.StructuredWorkflowData(req.ID, &wfData)
	if err != nil {
		return nil, err
	}

	// Get scan results through the scan service
	results, _, err := h.scanSvc.GetScanResults(wfData.CaseScanID, nil, nil, queryutil.Pagination{})
	if err != nil {
		if !errors.Is(err, db.ErrRecordNotFound) {
			return nil, fmt.Errorf("failed to get scan results: %w", err)
		}
		return &status, nil
	}

	status.ProgressPercent = calculateProgress(results, h.coreScanSvc)
	status.Message = "Scan in progress"

	return &status, nil
}

func (h *ScanOperationHandler) Cleanup(ctx context.Context, req *models.Request) error {
	return nil
}

func calculateProgress(results []*core.ScanResult, scannerService core.ContentScannerService) float64 {
	if len(results) == 0 {
		return 0
	}

	completed := len(results)
	total := len(scannerService.RegisteredScanners())
	return float64(completed) / float64(total)
}

// processScanResults processes the results of a content scan
func processScanResults(scanSvc svcTypes.ScanService, reqId uint, data *AbuseScanWorkflowData, results []*core.ScanResult, ctx core.Context) error {
	_logger := ctx.Logger()
	// Track if any scanners failed the content
	var finalStatus pluginModels.ScanStatus
	failed := lo.CountBy(results, func(item *core.ScanResult) bool {
		return !item.Passed
	})

	if failed > 0 {
		finalStatus = pluginModels.ScanStatusFlagged
	} else {
		finalStatus = pluginModels.ScanStatusClean
	}

	// Update scan status
	if err := scanSvc.UpdateScanStatus(data.CaseScanID, finalStatus); err != nil {
		_logger.Error("Failed to update scan status", zap.Error(err))
		return err
	}

	// Save detailed results
	if err := scanSvc.SaveScanResults(data.CaseScanID, results); err != nil {
		_logger.Error("Failed to save scan results", zap.Error(err))
		return err
	}

	_logger.Info("Scan task completed successfully",
		zap.Uint("scanID", data.CaseScanID),
		zap.String("status", string(finalStatus)))

	// Complete the workflow step
	if reqId > 0 {
		workflowSvc := core.GetService[core.WorkflowService](ctx, core.WORKFLOW_SERVICE)
		if workflowSvc != nil {
			if err := workflowSvc.CompleteWorkflowStep(context.Background(), reqId); err != nil {
				_logger.Error("Failed to complete workflow step",
					zap.Uint("requestID", reqId),
					zap.Error(err))
			}
		}
	} else {
		_logger.Info("No workflow context - skipping completion (normal for direct scans)",
			zap.Uint("requestID", reqId))
	}

	return nil
}

const AbuseScanOperationName = "abuse.content.scan"

func NewAbuseScanOperation(ctx core.Context) core.Operation {
	return core.NewOperation(
		AbuseScanOperationName,
		"",
		&ScanOperationHandler{
			OperationHelper: core.NewOperationHelper(ctx),
			ctx:             ctx,
			logger:          ctx.Logger(),
			scanSvc:         core.GetService[svcTypes.ScanService](ctx, svcTypes.SCAN_SERVICE),
			coreScanSvc:     core.GetService[core.ContentScannerService](ctx, core.CONTENT_SCANNER_SERVICE),
		},
	)
}
