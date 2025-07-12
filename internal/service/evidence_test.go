package service

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	coreMocks "go.lumeweb.com/portal/core/testing/mocks"
	"go.lumeweb.com/queryutil"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
)

func TestEvidenceService_CreateFromData(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		evidenceService := core.GetService[typesSvc.EvidenceService](ctx, typesSvc.EVIDENCE_SERVICE).(*EvidenceServiceDefault)
		assert.NotNil(tb, evidenceService)

		mockStorageService := core.GetService[core.StorageService](ctx, core.STORAGE_SERVICE).(*coreMocks.MockStorageService)
		assert.NotNil(tb, mockStorageService)

		mockCaseService := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, mockCaseService)

		mockReporterService := core.GetService[*mocks.MockReporterService](ctx, typesSvc.REPORTER_SERVICE)
		assert.NotNil(tb, mockReporterService)

		mockEmailService := core.GetService[*mocks.MockEmailService](ctx, typesSvc.EMAIL_SERVICE)
		assert.NotNil(tb, mockEmailService)

		caseID := uint(1)
		model := &models.Evidence{
			CaseID:      caseID,
			SubmitterID: uint(2),
			FileName:    "test.txt",
			ContentType: "text/plain",
			FileSize:    1024,
			Source:      models.EvidenceSourceWebUpload,
			Description: "Test evidence",
			Metadata:    datatypes.JSON(`{"key": "value"}`),
		}

		r := io.NopCloser(bytes.NewBufferString("test data"))

		// Mock storage service expectations
		objectKey := fmt.Sprintf("cases/%d/evidence/%d/%s",
			model.CaseID,
			1, // First record will have ID=1
			sanitizeFileName(model.FileName),
		)

		mockCaseModel := &models.Case{
			Model:           gorm.Model{ID: caseID},
			ReferenceNumber: "REF123",
			Type:            models.CaseTypeSpam,
			Status:          models.CaseStatusNew,
			Priority:        models.CasePriorityMedium,
			Description:     "Test case description",
			Source:          models.ReportSourceWebForm,
			ReporterID:      1,
			SubjectID:       1, // Must match the subject created below
			ContentHash:     "testhash",
		}

		mockReporter := &models.Reporter{
			Model: gorm.Model{ID: 1},
			Name:  "Test Reporter",
			Email: "test@example.com",
		}

		mockStorageService.On("S3MultipartUpload", mock.Anything, r, abuseBucketName, mock.AnythingOfType("string"), uint64(model.FileSize)).Return(nil)
		// Setup expected calls in correct order
		mockCaseService.On("GetByID", caseID).Return(mockCaseModel, nil)
		mockReporterService.On("GetByID", mockCaseModel.ReporterID).Return(mockReporter, nil)

		waitForChan := make(chan bool)
		mockEmailService.EXPECT().SendTemplatedEmail(mock.Anything, mock.Anything, mock.Anything).Run(func(to []string, templateName string, data core.MailerTemplateData) {
			waitForChan <- true
		}).Return(nil)

		// Act
		createdEvidence, err := evidenceService.CreateFromData(r, model)

		waitForAsync(t, waitForChan, func() {
			// Assert
			require.NoError(tb, err)
			assert.NotNil(tb, createdEvidence)
			assert.Equal(tb, objectKey, createdEvidence.StoragePath)
		})
	},
		coreTesting.WithService(typesSvc.EVIDENCE_SERVICE, NewEvidenceService),
	)
}

func TestEvidenceService_CreateFromHash(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		evidenceService := core.GetService[typesSvc.EvidenceService](ctx, typesSvc.EVIDENCE_SERVICE).(*EvidenceServiceDefault)
		assert.NotNil(tb, evidenceService)

		caseID := uint(1)
		submitterID := uint(2)
		storagePath := "test/path/to/file"
		fileName := "file.txt"
		contentType := "text/plain"
		fileSize := int64(1024)
		source := models.EvidenceSourceAPI
		description := "Test evidence from hash"
		metadata := datatypes.JSON(`{"key": "value"}`)

		// Act
		evidence, err := evidenceService.CreateFromHash(caseID, submitterID, storagePath, fileName, contentType, fileSize, source, description, metadata)

		// Assert
		require.NoError(tb, err)
		assert.NotNil(tb, evidence)
		assert.Equal(tb, caseID, evidence.CaseID)
		assert.Equal(tb, submitterID, evidence.SubmitterID)
		assert.Equal(tb, storagePath, evidence.StoragePath)
		assert.Equal(tb, fileName, evidence.FileName)
		assert.Equal(tb, contentType, evidence.ContentType)
		assert.Equal(tb, fileSize, evidence.FileSize)
		assert.Equal(tb, source, evidence.Source)
		assert.Equal(tb, description, evidence.Description)
		assert.Equal(tb, metadata, evidence.Metadata)
	},
		coreTesting.WithService(typesSvc.EVIDENCE_SERVICE, NewEvidenceService),
	)
}

func TestEvidenceService_GetByID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		evidenceService := core.GetService[typesSvc.EvidenceService](ctx, typesSvc.EVIDENCE_SERVICE).(*EvidenceServiceDefault)
		assert.NotNil(tb, evidenceService)

		evidenceID := uint(1)
		expectedEvidence := &models.Evidence{
			Model:       gorm.Model{ID: evidenceID},
			CaseID:      1,
			SubmitterID: 2,
			FileName:    "test.txt",
			ContentType: "text/plain",
			FileSize:    1024,
			StoragePath: "test/path",
			Source:      models.EvidenceSourceWebUpload,
			Description: "Test evidence",
			Metadata:    datatypes.JSON(`{"key": "value"}`),
		}

		// Manually add the data
		err := ctx.DB().Create(expectedEvidence).Error
		require.NoError(tb, err)

		// Act
		retrievedEvidence, err := evidenceService.GetByID(evidenceID)

		// Assert
		require.NoError(tb, err)
		assert.NotNil(tb, retrievedEvidence)
		assert.Equal(tb, expectedEvidence.ID, retrievedEvidence.ID)
		assert.Equal(tb, expectedEvidence.FileName, retrievedEvidence.FileName)
	},
		coreTesting.WithService(typesSvc.EVIDENCE_SERVICE, NewEvidenceService),
	)
}

func TestEvidenceService_List(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		evidenceService := core.GetService[typesSvc.EvidenceService](ctx, typesSvc.EVIDENCE_SERVICE).(*EvidenceServiceDefault)
		assert.NotNil(tb, evidenceService)

		evidence1 := &models.Evidence{
			Model:       gorm.Model{ID: 1},
			CaseID:      1,
			SubmitterID: 2,
			FileName:    "test1.txt",
			ContentType: "text/plain",
			FileSize:    1024,
			StoragePath: "test/path1",
			Source:      models.EvidenceSourceWebUpload,
			Description: "Test evidence 1",
			Metadata:    datatypes.JSON(`{"key": "value1"}`),
		}
		evidence2 := &models.Evidence{
			Model:       gorm.Model{ID: 2},
			CaseID:      2,
			SubmitterID: 3,
			FileName:    "test2.txt",
			ContentType: "text/plain",
			FileSize:    2048,
			StoragePath: "test/path2",
			Source:      models.EvidenceSourceAPI,
			Description: "Test evidence 2",
			Metadata:    datatypes.JSON(`{"key": "value2"}`),
		}

		// Manually add the data
		err := ctx.DB().Create(evidence1).Error
		require.NoError(tb, err)
		err = ctx.DB().Create(evidence2).Error
		require.NoError(tb, err)

		// Act
		filters := []queryutil.CrudFilter{}
		sorts := []queryutil.Sort{}
		pagination := queryutil.DefaultPagination
		retrievedEvidence, total, err := evidenceService.List(filters, sorts, pagination)

		// Assert
		require.NoError(tb, err)
		assert.Equal(tb, int64(2), total)
		assert.Len(tb, retrievedEvidence, 2)
		// Results should be ordered by created_at descending (newest first)
		assert.Equal(tb, evidence2.FileName, retrievedEvidence[0].FileName)
		assert.Equal(tb, evidence1.FileName, retrievedEvidence[1].FileName)
	},
		coreTesting.WithService(typesSvc.EVIDENCE_SERVICE, NewEvidenceService),
	)
}

func TestEvidenceService_Update(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		evidenceService := core.GetService[typesSvc.EvidenceService](ctx, typesSvc.EVIDENCE_SERVICE).(*EvidenceServiceDefault)
		assert.NotNil(tb, evidenceService)

		evidenceID := uint(1)
		initialEvidence := &models.Evidence{
			Model:       gorm.Model{ID: evidenceID},
			CaseID:      1,
			SubmitterID: 2,
			FileName:    "test.txt",
			ContentType: "text/plain",
			FileSize:    1024,
			StoragePath: "test/path",
			Source:      models.EvidenceSourceWebUpload,
			Description: "Test evidence",
			Metadata:    datatypes.JSON(`{"key": "value"}`),
		}

		// Manually add the data
		err := ctx.DB().Create(initialEvidence).Error
		require.NoError(tb, err)

		updatedEvidence := &models.Evidence{
			Model:       gorm.Model{ID: evidenceID},
			CaseID:      2, // Updated CaseID
			SubmitterID: 3, // Updated SubmitterID
			FileName:    "updated.txt",
			ContentType: "image/jpeg",
			FileSize:    2048,
			StoragePath: "updated/path",
			Source:      models.EvidenceSourceAPI,
			Description: "Updated evidence",
			Metadata:    datatypes.JSON(`{"key": "updated_value"}`),
		}

		// Act
		err = evidenceService.Update(updatedEvidence)

		// Assert
		require.NoError(tb, err)

		// Retrieve the updated evidence from the database
		var retrievedEvidence models.Evidence
		err = ctx.DB().First(&retrievedEvidence, evidenceID).Error
		require.NoError(tb, err)

		// Assert that the retrieved evidence matches the updated evidence
		assert.Equal(tb, updatedEvidence.CaseID, retrievedEvidence.CaseID)
		assert.Equal(tb, updatedEvidence.SubmitterID, retrievedEvidence.SubmitterID)
		assert.Equal(tb, updatedEvidence.FileName, retrievedEvidence.FileName)
		assert.Equal(tb, updatedEvidence.ContentType, retrievedEvidence.ContentType)
		assert.Equal(tb, updatedEvidence.FileSize, retrievedEvidence.FileSize)
		assert.Equal(tb, updatedEvidence.StoragePath, retrievedEvidence.StoragePath)
		assert.Equal(tb, updatedEvidence.Description, retrievedEvidence.Description)
		assert.Equal(tb, updatedEvidence.Metadata, retrievedEvidence.Metadata)
	},
		coreTesting.WithService(typesSvc.EVIDENCE_SERVICE, NewEvidenceService),
	)
}

func TestEvidenceService_Delete(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		evidenceService := core.GetService[typesSvc.EvidenceService](ctx, typesSvc.EVIDENCE_SERVICE).(*EvidenceServiceDefault)
		assert.NotNil(tb, evidenceService)

		evidenceID := uint(1)
		evidenceToDelete := &models.Evidence{
			Model:       gorm.Model{ID: evidenceID},
			CaseID:      1,
			SubmitterID: 2,
			FileName:    "test.txt",
			ContentType: "text/plain",
			FileSize:    1024,
			StoragePath: "test/path",
			Source:      models.EvidenceSourceWebUpload,
			Description: "Test evidence",
			Metadata:    datatypes.JSON(`{"key": "value"}`),
		}

		// Manually add the data
		err := ctx.DB().Create(evidenceToDelete).Error
		require.NoError(tb, err)

		// Act
		err = evidenceService.Delete(evidenceID)

		// Assert
		require.NoError(tb, err)

		// Try to retrieve the deleted evidence from the database
		var retrievedEvidence models.Evidence
		err = ctx.DB().First(&retrievedEvidence, evidenceID).Error

		// Assert that the evidence is not found in the database
		assert.Error(tb, err)
		assert.ErrorIs(tb, err, gorm.ErrRecordNotFound)
	},
		coreTesting.WithService(typesSvc.EVIDENCE_SERVICE, NewEvidenceService),
	)
}

func TestEvidenceService_GetEvidenceMetrics(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		evidenceService := core.GetService[typesSvc.EvidenceService](ctx, typesSvc.EVIDENCE_SERVICE).(*EvidenceServiceDefault)
		assert.NotNil(tb, evidenceService)

		// Define test times
		now := time.Now()
		start := now.Add(-24 * time.Hour)
		end := now

		// Create test data
		caseID1 := uint(1)
		caseID2 := uint(2)

		// Create evidence for case 1
		evidence1 := &models.Evidence{
			Model:       gorm.Model{CreatedAt: now},
			CaseID:      caseID1,
			SubmitterID: 1,
			FileName:    "test1.txt",
			ContentType: "text/plain",
			FileSize:    1024,
			StoragePath: "test/path1",
			Source:      models.EvidenceSourceWebUpload,
			Description: "Test evidence 1",
			Metadata:    datatypes.JSON(`{"key": "value1"}`),
		}
		evidence2 := &models.Evidence{
			Model:       gorm.Model{CreatedAt: now.Add(-1 * time.Hour)},
			CaseID:      caseID1,
			SubmitterID: 2,
			FileName:    "test2.txt",
			ContentType: "text/plain",
			FileSize:    2048,
			StoragePath: "test/path2",
			Source:      models.EvidenceSourceAPI,
			Description: "Test evidence 2",
			Metadata:    datatypes.JSON(`{"key": "value2"}`),
		}

		// Create evidence for case 2
		evidence3 := &models.Evidence{
			Model:       gorm.Model{CreatedAt: now.Add(-2 * time.Hour)},
			CaseID:      caseID2,
			SubmitterID: 3,
			FileName:    "test3.txt",
			ContentType: "text/plain",
			FileSize:    3072,
			StoragePath: "test/path3",
			Source:      models.EvidenceSourceSystem,
			Description: "Test evidence 3",
			Metadata:    datatypes.JSON(`{"key": "value3"}`),
		}

		// Manually add the data
		err := ctx.DB().Create(evidence1).Error
		require.NoError(tb, err)
		err = ctx.DB().Create(evidence2).Error
		require.NoError(tb, err)
		err = ctx.DB().Create(evidence3).Error
		require.NoError(tb, err)

		// Act
		analytics, err := evidenceService.GetEvidenceMetrics(start, end)

		// Assert
		require.NoError(tb, err)
		assert.NotNil(tb, analytics)

		// Verify FilesPerCase
		assert.Equal(tb, int64(2), analytics.FilesPerCase[caseID1])
		assert.Equal(tb, int64(1), analytics.FilesPerCase[caseID2])

		// Verify AvgFilesPerCase
		assert.Equal(tb, 1.5, analytics.AvgFilesPerCase)
	},
		coreTesting.WithService(typesSvc.EVIDENCE_SERVICE, NewEvidenceService),
	)
}

func TestEvidenceService_GetByCaseID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		evidenceService := core.GetService[typesSvc.EvidenceService](ctx, typesSvc.EVIDENCE_SERVICE).(*EvidenceServiceDefault)
		assert.NotNil(tb, evidenceService)

		caseID := uint(1)
		evidence1 := &models.Evidence{
			Model:       gorm.Model{ID: 1},
			CaseID:      caseID,
			SubmitterID: 2,
			FileName:    "test1.txt",
			ContentType: "text/plain",
			FileSize:    1024,
			StoragePath: "test/path1",
			Source:      models.EvidenceSourceWebUpload,
			Description: "Test evidence 1",
			Metadata:    datatypes.JSON(`{"key": "value1"}`),
		}
		evidence2 := &models.Evidence{
			Model:       gorm.Model{ID: 2},
			CaseID:      caseID,
			SubmitterID: 3,
			FileName:    "test2.txt",
			ContentType: "text/plain",
			FileSize:    2048,
			StoragePath: "test/path2",
			Source:      models.EvidenceSourceAPI,
			Description: "Test evidence 2",
			Metadata:    datatypes.JSON(`{"key": "value2"}`),
		}

		// Manually add the data
		err := ctx.DB().Create(evidence1).Error
		require.NoError(tb, err)
		err = ctx.DB().Create(evidence2).Error
		require.NoError(tb, err)

		// Act
		pagination := queryutil.DefaultPagination
		retrievedEvidence, total, err := evidenceService.GetByCaseID(caseID, pagination)

		// Assert
		require.NoError(tb, err)
		assert.Equal(tb, int64(2), total)
		assert.Len(tb, retrievedEvidence, 2)
		assert.Equal(tb, evidence1.FileName, retrievedEvidence[0].FileName)
		assert.Equal(tb, evidence2.FileName, retrievedEvidence[1].FileName)
	},
		coreTesting.WithService(typesSvc.EVIDENCE_SERVICE, NewEvidenceService),
	)
}

func TestEvidenceService_GetContent(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		evidenceService := core.GetService[typesSvc.EvidenceService](ctx, typesSvc.EVIDENCE_SERVICE).(*EvidenceServiceDefault)
		assert.NotNil(tb, evidenceService)

		mockStorageService := core.GetService[core.StorageService](ctx, core.STORAGE_SERVICE).(*coreMocks.MockStorageService)
		assert.NotNil(tb, mockStorageService)

		evidenceID := uint(1)
		storagePath := "test/path/to/file"
		contentType := "text/plain"
		expectedContent := "test data"

		evidence := &models.Evidence{
			Model:       gorm.Model{ID: evidenceID},
			CaseID:      1,
			SubmitterID: 2,
			FileName:    "test.txt",
			ContentType: contentType,
			FileSize:    1024,
			StoragePath: storagePath,
			Source:      models.EvidenceSourceWebUpload,
			Description: "Test evidence",
			Metadata:    datatypes.JSON(`{"key": "value"}`),
		}

		// Manually add the data
		err := ctx.DB().Create(evidence).Error
		require.NoError(tb, err)

		mockStorageService.On("S3GetTemporaryUpload", mock.Anything, nil, storagePath).Return(io.NopCloser(bytes.NewBufferString(expectedContent)), nil)

		// Act
		reader, retrievedContentType, err := evidenceService.GetContent(evidenceID)

		// Assert
		require.NoError(tb, err)
		assert.Equal(tb, contentType, retrievedContentType)

		content, err := io.ReadAll(reader)
		require.NoError(tb, err)
		assert.Equal(tb, expectedContent, string(content))

		mockStorageService.AssertExpectations(t)
	},
		coreTesting.WithService(typesSvc.EVIDENCE_SERVICE, NewEvidenceService))
}
