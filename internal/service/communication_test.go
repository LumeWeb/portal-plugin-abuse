package service

import (
	"fmt"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"go.lumeweb.com/queryutil"
	"gorm.io/gorm"
)

func TestCommunicationService_Create(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		commService := core.GetService[typesSvc.CommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
		assert.NotNil(tb, commService)

		caseID := uint(1)
		reporterID := uint(2)

		comm := &models.Communication{
			CaseID:    caseID,
			SenderID:  1,
			Type:      models.CommunicationTypeNote,
			Direction: models.CommunicationDirectionIncoming,
			Content:   "Test comment",
		}

		caseModel := &models.Case{
			Model:           gorm.Model{ID: caseID},
			ReferenceNumber: "REF123",
			Type:            models.CaseTypeSpam,
			Status:          models.CaseStatusNew,
			Priority:        models.CasePriorityMedium,
			Description:     "Test case description",
			Source:          models.ReportSourceWebForm,
			ReporterID:      reporterID,
			SubjectID:       1, // Must match the subject created below
		}

		reporter := &models.Reporter{
			Model: gorm.Model{ID: reporterID},
			Name:  "Test Reporter",
			Email: "test@example.com",
		}

		// Create subject first since case requires it
		subject := &models.Subject{
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}
		err := ctx.DB().Create(subject).Error
		require.NoError(tb, err)

		// Get mock services and set expectations
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		require.NotNil(tb, mockCaseSvc)
		mockReporterSvc := core.GetService[*mocks.MockReporterService](ctx, typesSvc.REPORTER_SERVICE)
		require.NotNil(tb, mockReporterSvc)

		// Get mock email service and set expectations
		mockEmailSvc := core.GetService[*mocks.MockEmailService](ctx, typesSvc.EMAIL_SERVICE)
		require.NotNil(tb, mockEmailSvc)

		waitForChan := make(chan bool)

		// Setup expected calls
		mockCaseSvc.On("GetByID", caseID).Return(caseModel, nil)
		mockReporterSvc.On("GetByID", reporterID).Return(reporter, nil)
		mockEmailSvc.EXPECT().SendTemplatedEmail(mock.Anything, mock.Anything, mock.Anything).Run(func(to []string, templateName string, data core.MailerTemplateData) {
			waitForChan <- true
		}).Return(nil)

		// Act
		createdComm, err := commService.Create(comm)
		require.NoError(tb, err)

		waitForAsync(tb, waitForChan, nil)

		// Retrieve the created communication from the database
		var retrievedComm models.Communication
		err = ctx.DB().First(&retrievedComm, createdComm.ID).Error
		require.NoError(tb, err)

		// Assert
		assert.NotNil(tb, retrievedComm)
		assert.Equal(tb, comm.CaseID, retrievedComm.CaseID)
		assert.Equal(tb, comm.Content, retrievedComm.Content)
	},
		coreTesting.WithService(typesSvc.COMMUNICATION_SERVICE, NewCommunicationService),
	)
}

func TestCommunicationService_GetByID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		commService := core.GetService[typesSvc.CommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
		assert.NotNil(tb, commService)

		commID := uint(1)
		expectedComm := &models.Communication{
			Model:     gorm.Model{ID: commID},
			CaseID:    1,
			SenderID:  1,
			Type:      models.CommunicationTypeNote,
			Direction: models.CommunicationDirectionIncoming,
			Content:   "Test comment",
		}

		// Manually add the data
		err := ctx.DB().Create(expectedComm).Error
		require.NoError(tb, err)

		// Act
		retrievedComm, err := commService.GetByID(commID)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, retrievedComm)
		assert.Equal(tb, expectedComm.ID, retrievedComm.ID)
		assert.Equal(tb, expectedComm.Content, retrievedComm.Content)
	},
		coreTesting.WithService(typesSvc.COMMUNICATION_SERVICE, NewCommunicationService),
	)
}

func TestCommunicationService_GetByThreadID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		commService := core.GetService[typesSvc.CommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
		assert.NotNil(tb, commService)

		threadID := "test-thread-id"
		expectedComm := &models.Communication{
			Model:     gorm.Model{ID: 1},
			CaseID:    1,
			SenderID:  1,
			Type:      models.CommunicationTypeEmail,
			Direction: models.CommunicationDirectionIncoming,
			Content:   "Test comment",
			ThreadID:  threadID,
		}

		// Manually add the data
		err := ctx.DB().Create(expectedComm).Error
		require.NoError(tb, err)

		// Act
		retrievedComm, err := commService.GetByThreadID(threadID)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, retrievedComm)
		assert.Equal(tb, expectedComm.ID, retrievedComm.ID)
		assert.Equal(tb, expectedComm.ThreadID, retrievedComm.ThreadID)
	},
		coreTesting.WithService(typesSvc.COMMUNICATION_SERVICE, NewCommunicationService),
	)
}

func TestCommunicationService_ListByCaseID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		commService := core.GetService[typesSvc.CommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
		assert.NotNil(tb, commService)

		caseID := uint(1)
		comm1 := &models.Communication{
			Model:     gorm.Model{ID: 1},
			CaseID:    caseID,
			SenderID:  1,
			Type:      models.CommunicationTypeNote,
			Direction: models.CommunicationDirectionIncoming,
			Content:   "Test comment 1",
		}
		comm2 := &models.Communication{
			Model:     gorm.Model{ID: 2},
			CaseID:    caseID,
			SenderID:  2,
			Type:      models.CommunicationTypeEmail,
			Direction: models.CommunicationDirectionOutgoing,
			Content:   "Test comment 2",
		}

		// Manually add the data
		err := ctx.DB().Create(comm1).Error
		require.NoError(tb, err)
		err = ctx.DB().Create(comm2).Error
		require.NoError(tb, err)

		// Act
		filters := []queryutil.CrudFilter{}
		sorts := []queryutil.Sort{}
		pagination := queryutil.DefaultPagination
		retrievedComms, total, err := commService.ListByCaseID(caseID, filters, sorts, pagination)

		// Assert
		assert.NoError(tb, err)
		assert.Equal(tb, int64(2), total)
		assert.Len(tb, retrievedComms, 2)
		assert.Equal(tb, comm1.Content, retrievedComms[0].Content)
		assert.Equal(tb, comm2.Content, retrievedComms[1].Content)
	},
		coreTesting.WithService(typesSvc.COMMUNICATION_SERVICE, NewCommunicationService),
	)
}

func TestCommunicationService_GetCommunicationTimeline_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		commService := core.GetService[typesSvc.CommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
		assert.NotNil(tb, commService)

		// Create test communications at different hours
		now := time.Now().UTC()
		testComms := []struct {
			createdAt time.Time
			count     int
		}{
			{now.Add(-2 * time.Hour), 3}, // 3 comms 2 hours ago
			{now.Add(-1 * time.Hour), 2}, // 2 comms 1 hour ago
			{now, 1},                     // 1 comm now
		}

		for _, tc := range testComms {
			for i := 0; i < tc.count; i++ {
				comm := &models.Communication{
					Model:     gorm.Model{CreatedAt: tc.createdAt},
					CaseID:    1,
					SenderID:  1,
					Type:      models.CommunicationTypeNote,
					Direction: models.CommunicationDirectionIncoming,
					Content:   fmt.Sprintf("Test comment %d", i),
				}
				err := ctx.DB().Create(comm).Error
				require.NoError(tb, err)
			}
		}

		// Act
		timeline, err := commService.GetCommunicationTimeline("24h", nil)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, timeline)
		assert.Len(tb, timeline, 3)

		// Verify counts per hour
		for _, item := range timeline {
			switch item.HourlyInterval {
			case now.Add(-2 * time.Hour).Format("2006-01-02 15:00:00"):
				assert.Equal(tb, int64(3), item.CommCount)
			case now.Add(-1 * time.Hour).Format("2006-01-02 15:00:00"):
				assert.Equal(tb, int64(2), item.CommCount)
			case now.Format("2006-01-02 15:00:00"):
				assert.Equal(tb, int64(1), item.CommCount)
			}
		}
	},
		coreTesting.WithService(typesSvc.COMMUNICATION_SERVICE, NewCommunicationService),
	)
}

func TestCommunicationService_GetCommunicationTimeline_Empty(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		commService := core.GetService[typesSvc.CommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
		assert.NotNil(tb, commService)

		// Act
		timeline, err := commService.GetCommunicationTimeline("24h", nil)

		// Assert
		assert.NoError(tb, err)
		assert.Empty(tb, timeline)
	},
		coreTesting.WithService(typesSvc.COMMUNICATION_SERVICE, NewCommunicationService),
	)
}

func TestCommunicationService_GetCommunicationTimeline_InvalidTimeRange(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		commService := core.GetService[typesSvc.CommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
		assert.NotNil(tb, commService)

		// Act
		_, err := commService.GetCommunicationTimeline("invalid", nil)

		// Assert
		assert.Error(tb, err)
		assert.ErrorContains(tb, err, "invalid timeRange")
	},
		coreTesting.WithService(typesSvc.COMMUNICATION_SERVICE, NewCommunicationService),
	)
}

func TestCommunicationService_GetCommunicationMetrics(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		commService := core.GetService[typesSvc.CommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
		assert.NotNil(tb, commService)

		// Define test times
		now := time.Now()
		start := now.Add(-24 * time.Hour)
		end := now

		// Create test data
		caseID1 := uint(1)
		caseID2 := uint(2)

		// Create communications for case 1
		comm1 := &models.Communication{
			Model:     gorm.Model{CreatedAt: now},
			CaseID:    caseID1,
			SenderID:  1,
			Type:      models.CommunicationTypeNote,
			Direction: models.CommunicationDirectionIncoming,
			Content:   "Test comment 1",
		}
		comm2 := &models.Communication{
			Model:     gorm.Model{CreatedAt: now.Add(-1 * time.Hour)},
			CaseID:    caseID1,
			SenderID:  2,
			Type:      models.CommunicationTypeEmail,
			Direction: models.CommunicationDirectionOutgoing,
			Content:   "Test comment 2",
		}

		// Create communications for case 2
		comm3 := &models.Communication{
			Model:     gorm.Model{CreatedAt: now.Add(-2 * time.Hour)},
			CaseID:    caseID2,
			SenderID:  3,
			Type:      models.CommunicationTypeResponse,
			Direction: models.CommunicationDirectionIncoming,
			Content:   "Test comment 3",
		}

		// Manually add the data
		err := ctx.DB().Create(comm1).Error
		require.NoError(tb, err)
		err = ctx.DB().Create(comm2).Error
		require.NoError(tb, err)
		err = ctx.DB().Create(comm3).Error
		require.NoError(tb, err)

		// Act
		analytics, err := commService.GetCommunicationMetrics(start, end)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, analytics)

		// Verify CommsPerCase
		assert.Equal(tb, int64(2), analytics.CommsPerCase[caseID1])
		assert.Equal(tb, int64(1), analytics.CommsPerCase[caseID2])

		// There is no good way to test this without mocking a lot of data
		// However, we can at least assert that they are initialized to zero
		assert.Equal(tb, time.Duration(0), analytics.AvgResponseTime)
		assert.Equal(tb, time.Duration(0), analytics.MaxResponseTime)
	},
		coreTesting.WithService(typesSvc.COMMUNICATION_SERVICE, NewCommunicationService),
	)
}
