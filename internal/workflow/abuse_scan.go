package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go.lumeweb.com/portal-plugin-abuse/internal/cron/define"
	svcTypes "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/service"
	"go.lumeweb.com/queryutil"
	"go.uber.org/zap"
	"time"

	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/db/models"
)

const (
	AbuseScanWorkflowName = "abuse.scan"
	scanOperationType     = "abuse.scan.execute"
)

type AbuseScanWorkflowInitData struct {
	CaseScanID uint `json:"case_scan_id"`
	SubjectID  uint `json:"subject_id"`
}

type AbuseScanWorkflow struct {
	ctx         core.Context
	logger      *core.Logger
	coordinator core.WorkflowCoordinator
	cron        core.CronService
	coreScanSvc core.ContentScannerService
	scanSvc     svcTypes.ScanService
}

func NewAbuseScanWorkflow(ctx core.Context, coordinator core.WorkflowCoordinator, cron core.CronService, coreScanSvc core.ContentScannerService, scanSvc svcTypes.ScanService) *AbuseScanWorkflow {
	return &AbuseScanWorkflow{
		ctx:         ctx,
		logger:      ctx.Logger(),
		coordinator: coordinator,
		cron:        cron,
		coreScanSvc: coreScanSvc,
		scanSvc:     scanSvc,
	}
}

func (w *AbuseScanWorkflow) Register() error {
	return w.coordinator.RegisterWorkflow(AbuseScanWorkflowName, []core.OperationStep{
		{
			Operation: scanOperationType,
			Handler: &scanOperationHandler{
				ctx:         w.ctx,
				logger:      w.logger,
				cron:        w.cron,
				coreScanSvc: w.coreScanSvc,
			},
			FailureBehavior: core.RetryStep,
		},
	})
}

type scanOperationHandler struct {
	ctx         core.Context
	logger      *core.Logger
	cron        core.CronService
	coreScanSvc core.ContentScannerService
	scanSvc     svcTypes.ScanService
}

func (h *scanOperationHandler) ValidateRequest(ctx context.Context, req *models.Request) error {
	// Noop
	return nil
}

func (h *scanOperationHandler) Execute(_ context.Context, req *models.Request) error {
	var metaData service.WorkflowMetadata
	err := json.Unmarshal(req.Metadata, &metaData)
	if err != nil {
		return err
	}

	initData, ok := (metaData.InitialData).(*AbuseScanWorkflowInitData)
	if !ok {
		return errors.New("invalid init data")
	}

	// Schedule scan as a cron job with workflow instance ID
	err = h.cron.CreateJobIfNotExists(
		define.CronTaskScanName,
		define.CronTaskScanArgs{
			RequestID:  req.ID,
			CaseScanID: initData.CaseScanID,
			SubjectID:  initData.SubjectID,
		},
	)
	if err != nil {
		h.logger.Error("Failed to schedule scan job",
			zap.Error(err),
			zap.Uint("request_id", req.ID),
		)
		return fmt.Errorf("failed to schedule scan: %w", err)
	}

	return nil
}

func (h *scanOperationHandler) GetStatus(ctx context.Context, req *models.Request) (core.RequestStatus, error) {
	initData, err := getInitData(req)
	if err != nil {
		return core.RequestStatus{}, err
	}
	// Check cron job status
	exists, job := h.cron.JobExists(
		define.CronTaskScanName,
		define.CronTaskScanArgs{
			RequestID:  req.ID,
			CaseScanID: initData.CaseScanID,
			SubjectID:  initData.SubjectID,
		},
	)

	if !exists {
		return core.RequestStatus{
			State:           string(models.RequestStatusFailed),
			ProgressPercent: 0,
			Message:         "Scan job not found",
		}, nil
	}

	// Get scan results through the scan service
	results, _, err := h.scanSvc.GetScanResults(initData.CaseScanID, nil, nil, queryutil.Pagination{})
	if err != nil {
		return core.RequestStatus{}, fmt.Errorf("failed to get scan results: %w", err)
	}

	status := core.RequestStatus{
		State:           string(cronStatusToRequestStatus(job.State)),
		ProgressPercent: calculateProgress(results, h.coreScanSvc),
		Message:         "Scan in progress",
		UpdatedAt:       time.Now(),
	}

	return status, nil
}

func (h *scanOperationHandler) Cleanup(ctx context.Context, req *models.Request) error {
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

func getInitData(req *models.Request) (*AbuseScanWorkflowInitData, error) {
	var metaData service.WorkflowMetadata
	err := json.Unmarshal(req.Metadata, &metaData)
	if err != nil {
		return nil, err
	}

	initData, ok := (metaData.InitialData).(*AbuseScanWorkflowInitData)
	if !ok {
		return nil, errors.New("invalid init data")
	}

	return initData, nil
}

func cronStatusToRequestStatus(status models.CronJobState) models.RequestStatusType {
	switch status {
	case models.CronJobStateQueued:
		return models.RequestStatusPending
	case models.CronJobStateProcessing:
		return models.RequestStatusProcessing
	case models.CronJobStateCompleted:
		return models.RequestStatusCompleted
	case models.CronJobStateFailed:
		return models.RequestStatusFailed
	default:
		return models.RequestStatusFailed // Default to failed for unknown states
	}
}
