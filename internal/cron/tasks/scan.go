package tasks

import (
	"context"
	"fmt"
	"github.com/samber/lo"
	"go.lumeweb.com/portal-plugin-abuse/internal/cron/define"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	svcTypes "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
)

// CronTaskScan handles scanning a file for abuse content
func CronTaskScan(args *define.CronTaskScanArgs, ctx core.Context) error {
	logger := ctx.Logger()
	logger.Info("Starting abuse scan task",
		zap.Uint("caseScanID", args.CaseScanID),
		zap.Uint("subjectID", args.SubjectID), zap.Uint("workflowID", args.RequestID))

	// Get the scan service
	scanSvc := core.GetService[svcTypes.ScanService](ctx, svcTypes.SCAN_SERVICE)
	if scanSvc == nil {
		logger.Error("Scan service not available")
		return fmt.Errorf("scan service not available")
	}

	// Get content scanner service from the portal core
	contentScannerSvc := core.GetService[core.ContentScannerService](ctx, core.CONTENT_SCANNER_SERVICE)
	if contentScannerSvc == nil {
		logger.Error("Content scanner service not available")
		// Update scan status to error
		updateErr := scanSvc.UpdateScanStatus(args.CaseScanID, models.ScanStatusError)
		if updateErr != nil {
			logger.Error("Failed to update scan status", zap.Error(updateErr))
		}
		return fmt.Errorf("content scanner service not available")
	}

	// Get the subject service
	subjectSvc := core.GetService[svcTypes.SubjectService](ctx, svcTypes.SUBJECT_SERVICE)
	if subjectSvc == nil {
		logger.Error("Subject service not available")
		return fmt.Errorf("subject service not available")
	}

	// Update scan status to scanning
	if err := scanSvc.UpdateScanStatus(args.CaseScanID, models.ScanStatusScanning); err != nil {
		logger.Error("Failed to update scan status", zap.Error(err))
		return err
	}

	subject, err := subjectSvc.GetByID(args.SubjectID)
	if err != nil {
		return err
	}

	hash := core.NewStorageHashFromRawMultihash(subject.Identifier)

	results, err := contentScannerSvc.ScanContent(ctx.GetContext(), hash)
	if err != nil {
		return err
	}

	// Process scan results first
	if err := processScanResults(scanSvc, args, results, ctx); err != nil {
		// If processing failed, ensure status is marked as error
		_ = scanSvc.UpdateScanStatus(args.CaseScanID, models.ScanStatusError)
		return err
	}
	return nil
}

// processScanResults processes the results of a content scan
func processScanResults(scanSvc svcTypes.ScanService, args *define.CronTaskScanArgs, results []*core.ScanResult, ctx core.Context) error {
	_logger := ctx.Logger()
	// Track if any scanners failed the content
	var finalStatus models.ScanStatus
	failed := lo.CountBy(results, func(item *core.ScanResult) bool {
		return !item.Passed
	})

	if failed > 0 {
		finalStatus = models.ScanStatusFlagged
	} else {
		finalStatus = models.ScanStatusClean
	}

	// Update scan status
	if err := scanSvc.UpdateScanStatus(args.CaseScanID, finalStatus); err != nil {
		_logger.Error("Failed to update scan status", zap.Error(err))
		return err
	}

	// Save detailed results
	if err := scanSvc.SaveScanResults(args.CaseScanID, results); err != nil {
		_logger.Error("Failed to save scan results", zap.Error(err))
		return err
	}

	_logger.Info("Scan task completed successfully",
		zap.Uint("scanID", args.CaseScanID),
		zap.String("status", string(finalStatus)))

	// Complete the workflow step
	if args.RequestID > 0 {
		workflowSvc := core.GetService[core.WorkflowService](ctx, core.WORKFLOW_SERVICE)
		if workflowSvc != nil {
			if err := workflowSvc.CompleteWorkflowStep(context.Background(), args.RequestID); err != nil {
				_logger.Error("Failed to complete workflow step",
					zap.Uint("requestID", args.RequestID),
					zap.Error(err))
			}
		}
	} else {
		_logger.Info("No workflow context - skipping completion (normal for direct scans)",
			zap.Uint("requestID", args.RequestID))
	}

	return nil
}
