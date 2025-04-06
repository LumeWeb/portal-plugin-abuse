package service

import (
	"context"
	"encoding/json"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal-plugin-abuse/internal/workflow"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	coreMocks "go.lumeweb.com/portal/core/testing/mocks"
	"go.lumeweb.com/queryutil"
	"gorm.io/gorm"
	"testing"
)

func TestScanService_CreateScanRequest(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		scanService := core.GetService[typesSvc.ScanService](ctx, typesSvc.SCAN_SERVICE)
		assert.NotNil(tb, scanService)

		caseID := uint(1)
		subjectID := uint(2)

		// Mock services
		mockCaseService := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		require.NotNil(tb, mockCaseService)
		mockWorkflowService := core.GetService[*coreMocks.MockWorkflowService](ctx, core.WORKFLOW_SERVICE)
		require.NotNil(tb, mockWorkflowService)

		// Set up mock expectations
		mockCaseService.On("GetByID", caseID).Return(&models.Case{
			Model:     gorm.Model{ID: caseID},
			SubjectID: subjectID,
		}, nil).Once()

		mockWorkflowService.On("StartWorkflow", ctx.GetContext(), workflow.AbuseScanWorkflowName, mock.Anything).Return(nil, nil).Once()

		// Act
		err := scanService.CreateScanRequest(caseID)

		// Assert
		assert.NoError(tb, err)
		mockCaseService.AssertExpectations(t)
		mockWorkflowService.AssertExpectations(t)
	},
		coreTesting.WithService(typesSvc.SCAN_SERVICE, NewScanService))
}

func TestScanService_GetScansForCase(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		scanService := core.GetService[typesSvc.ScanService](ctx, typesSvc.SCAN_SERVICE)
		assert.NotNil(tb, scanService)

		caseID := uint(1)
		pagination := queryutil.DefaultPagination

		// Create test data directly in database
		scan1 := &models.CaseScan{
			CaseID:    caseID,
			SubjectID: 1,
			Status:    models.ScanStatusPending,
		}
		err := ctx.DB().Create(scan1).Error
		require.NoError(tb, err)

		scan2 := &models.CaseScan{
			CaseID:    caseID,
			SubjectID: 2,
			Status:    models.ScanStatusClean,
		}
		err = ctx.DB().Create(scan2).Error
		require.NoError(tb, err)

		// Act
		scans, total, err := scanService.GetScansForCase(caseID, pagination)

		// Assert
		assert.NoError(tb, err)
		assert.Equal(tb, int64(2), total)
		assert.Len(tb, scans, 2)
		assert.Equal(tb, caseID, scans[0].CaseID)
		assert.Equal(tb, caseID, scans[1].CaseID)
	},
		coreTesting.WithService(typesSvc.SCAN_SERVICE, NewScanService),
	)
}

func TestScanService_GetScanById(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		scanService := core.GetService[typesSvc.ScanService](ctx, typesSvc.SCAN_SERVICE)
		assert.NotNil(tb, scanService)

		scanID := uint(1)

		// Create test data directly in database
		expectedScan := &models.CaseScan{
			Model:     gorm.Model{ID: scanID},
			CaseID:    1,
			SubjectID: 1,
			Status:    models.ScanStatusPending,
		}
		err := ctx.DB().Create(expectedScan).Error
		require.NoError(tb, err)

		// Act
		scan, err := scanService.GetScanById(scanID)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, scan)
		assert.Equal(tb, scanID, scan.ID)
	},
		coreTesting.WithService(typesSvc.SCAN_SERVICE, NewScanService),
	)
}

func TestScanService_SaveScanResults(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		scanService := core.GetService[typesSvc.ScanService](ctx, typesSvc.SCAN_SERVICE)
		assert.NotNil(tb, scanService)

		scanID := uint(1)
		results := []*core.ScanResult{
			{
				ID:     1,
				Passed: true,
			},
			{
				ID:     2,
				Passed: false,
			},
		}

		// Create test data directly in database
		initialScan := &models.CaseScan{
			Model:     gorm.Model{ID: scanID},
			CaseID:    1,
			SubjectID: 1,
			Status:    models.ScanStatusPending,
		}
		err := ctx.DB().Create(initialScan).Error
		require.NoError(tb, err)

		// Act
		err = scanService.SaveScanResults(scanID, results)

		// Assert
		assert.NoError(tb, err)

		// Verify that the scan results are saved in the database
		var updatedScan models.CaseScan
		err = ctx.DB().First(&updatedScan, scanID).Error
		require.NoError(tb, err)

		var scanResultIDs []uint64
		err = json.Unmarshal(updatedScan.ScanResults, &scanResultIDs)
		require.NoError(tb, err)

		expectedScanResultIDs := lo.Map(results, func(item *core.ScanResult, _ int) uint64 {
			return item.ID
		})

		assert.Equal(tb, expectedScanResultIDs, scanResultIDs)
	},
		coreTesting.WithService(typesSvc.SCAN_SERVICE, NewScanService),
	)
}

func TestScanService_GetScanResults(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		scanService := core.GetService[typesSvc.ScanService](ctx, typesSvc.SCAN_SERVICE)
		assert.NotNil(tb, scanService)

		scanID := uint(1)
		filters := []queryutil.CrudFilter{}
		sorts := []queryutil.Sort{}
		pagination := queryutil.DefaultPagination

		// Mock core scanner service
		mockCoreScannerService := core.GetService[*coreMocks.MockContentScannerService](ctx, core.CONTENT_SCANNER_SERVICE)
		require.NotNil(tb, mockCoreScannerService)

		// Create test data directly in database
		scanResults := []*core.ScanResult{
			{
				ID:     1,
				Passed: true,
			},
			{
				ID:     2,
				Passed: false,
			},
		}

		scanResultIDs := lo.Map(scanResults, func(item *core.ScanResult, _ int) uint64 {
			return item.ID
		})

		scanResultsJSON, err := json.Marshal(scanResultIDs)
		require.NoError(tb, err)

		initialScan := &models.CaseScan{
			Model:       gorm.Model{ID: scanID},
			CaseID:      1,
			SubjectID:   1,
			Status:      models.ScanStatusPending,
			ScanResults: scanResultsJSON,
		}
		err = ctx.DB().Create(initialScan).Error
		require.NoError(tb, err)

		// Set up mock expectations
		// Convert core.ScanResult to *core.ScanResult for mock return
		mockCoreScannerService.On("GetScanResultById", context.Background(), uint(1)).Return(&core.ScanResult{
			ID:     1,
			Passed: false,
		}, nil).Once()
		mockCoreScannerService.On("GetScanResultById", context.Background(), uint(2)).Return(scanResults[1], nil).Once()

		// Act
		retrievedResults, total, err := scanService.GetScanResults(scanID, filters, sorts, pagination)

		// Assert
		assert.NoError(tb, err)
		assert.Equal(tb, int64(2), total)
		assert.Len(tb, retrievedResults, 2)
		assert.Equal(tb, scanResults[0].ID, retrievedResults[0].ID)
		assert.Equal(tb, scanResults[1].ID, retrievedResults[1].ID)

		mockCoreScannerService.AssertExpectations(t)
	},
		coreTesting.WithService(typesSvc.SCAN_SERVICE, NewScanService),
	)
}

func TestScanService_GetScanResults_ScanNotFound(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		scanService := core.GetService[typesSvc.ScanService](ctx, typesSvc.SCAN_SERVICE)
		assert.NotNil(tb, scanService)

		scanID := uint(999) // Non-existent scan ID
		filters := []queryutil.CrudFilter{}
		sorts := []queryutil.Sort{}
		pagination := queryutil.DefaultPagination

		// Act
		_, _, err := scanService.GetScanResults(scanID, filters, sorts, pagination)

		// Assert
		assert.Error(tb, err)
		assert.ErrorIs(tb, err, db.ErrRecordNotFound)
	},
		coreTesting.WithService(typesSvc.SCAN_SERVICE, NewScanService),
		coreTesting.WithMockServiceFactory(core.CONTENT_SCANNER_SERVICE, coreMocks.NewMockContentScannerService),
	)
}

func TestScanService_UpdateScanStatus(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		scanService := core.GetService[typesSvc.ScanService](ctx, typesSvc.SCAN_SERVICE)
		assert.NotNil(tb, scanService)

		scanID := uint(1)
		newStatus := models.ScanStatusFlagged

		// Mock services
		mockCaseService := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		require.NotNil(tb, mockCaseService)
		mockReporterService := core.GetService[*mocks.MockReporterService](ctx, typesSvc.REPORTER_SERVICE)
		require.NotNil(tb, mockReporterService)
		mockEmailService := core.GetService[*mocks.MockEmailService](ctx, typesSvc.EMAIL_SERVICE)
		require.NotNil(tb, mockEmailService)

		// Create test data directly in database
		initialScan := &models.CaseScan{
			Model:     gorm.Model{ID: scanID},
			CaseID:    1,
			SubjectID: 1,
			Status:    models.ScanStatusPending,
		}
		err := ctx.DB().Create(initialScan).Error
		require.NoError(tb, err)

		// Mock core scanner service
		mockCoreScannerService := core.GetService[*coreMocks.MockContentScannerService](ctx, core.CONTENT_SCANNER_SERVICE)
		require.NotNil(tb, mockCoreScannerService)

		scanResults := []*core.ScanResult{
			{
				ID:     1,
				Passed: false,
			},
		}

		scanResultIDs := lo.Map(scanResults, func(item *core.ScanResult, _ int) uint64 {
			return item.ID
		})

		scanResultsJSON, err := json.Marshal(scanResultIDs)
		require.NoError(tb, err)

		initialScan.ScanResults = scanResultsJSON
		err = ctx.DB().Save(initialScan).Error
		require.NoError(tb, err)

		mockCoreScannerService.On("GetScanResultById", context.Background(), uint(1)).Return(scanResults[0], nil).Once()

		// Act
		err = scanService.UpdateScanStatus(scanID, newStatus)

		// Assert
		assert.NoError(tb, err)

		// Verify that the scan status is updated in the database
		var updatedScan models.CaseScan
		err = ctx.DB().First(&updatedScan, scanID).Error
		require.NoError(tb, err)

		assert.Equal(tb, newStatus, updatedScan.Status)

		mockCaseService.AssertExpectations(t)
		mockReporterService.AssertExpectations(t)
		mockEmailService.AssertExpectations(t)
		mockCoreScannerService.AssertExpectations(t)
	},
		coreTesting.WithService(typesSvc.SCAN_SERVICE, NewScanService),
	)
}

func TestScanService_UpdateScanStatus_ScanNotFound(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		scanService := core.GetService[typesSvc.ScanService](ctx, typesSvc.SCAN_SERVICE)
		assert.NotNil(tb, scanService)

		scanID := uint(999) // Non-existent scan ID
		newStatus := models.ScanStatusFlagged

		// Act
		err := scanService.UpdateScanStatus(scanID, newStatus)

		// Assert
		assert.Error(tb, err)
		assert.ErrorIs(tb, err, db.ErrRecordNotFound)
	},
		coreTesting.WithService(typesSvc.SCAN_SERVICE, NewScanService),
	)
}
