package service

import (
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/util"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"go.lumeweb.com/queryutil"
	"gorm.io/gorm"
)

func TestCaseService_Create(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		caseService := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, caseService)

		mockEmailService := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE).(*mocks.MockEmailService)

		// Create reporter first
		reporter := &models.Reporter{
			Email: "test@example.com",
			Name:  "Test Reporter",
		}
		err := ctx.DB().Create(reporter).Error
		require.NoError(tb, err)

		// Create subject
		subject := &models.Subject{
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}
		err = ctx.DB().Create(subject).Error
		require.NoError(tb, err)

		caseData := &models.Case{
			Type:            models.CaseTypeSpam,
			Status:          models.CaseStatusNew,
			Priority:        models.CasePriorityMedium,
			Description:     "Test spam report",
			Source:          models.ReportSourceWebForm,
			ReporterID:      reporter.ID,
			SubjectID:       subject.ID,
			ReferenceNumber: "testref",
		}

		expectedCase := &models.Case{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Type:            models.CaseTypeSpam,
			Status:          models.CaseStatusNew,
			Priority:        models.CasePriorityMedium,
			Description:     "Test spam report",
			ReporterID:      1,
			SubjectID:       1,
			ReferenceNumber: "testref",
		}

		// Get mock services
		mockReporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*mocks.MockReporterService)
		mockSubjectService := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE).(*mocks.MockSubjectService)
		mockTokenService := core.GetService[typesSvc.TokenService](ctx, typesSvc.TOKEN_SERVICE).(*mocks.MockTokenService)

		// Set up mock expectations
		mockReporterService.EXPECT().GetByID(reporter.ID).Return(reporter, nil).Once()
		mockSubjectService.EXPECT().GetByID(subject.ID).Return(subject, nil).Once()
		mockTokenService.EXPECT().
			GenerateToken(mock.AnythingOfType("uint"), reporter.ID, 90).
			Return("test-token", time.Now().Add(90*24*time.Hour), nil).
			Once()

		// Mock the email service expectations
		mockEmailService.EXPECT().
			GenerateCaseThreadID(mock.AnythingOfType("uint"), mock.AnythingOfType("string")).
			Return("thread-id").
			Once()

		// Get mock communication service
		mockCommService := core.GetService[*mocks.MockCommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
		mockCommService.EXPECT().
			Create(mock.AnythingOfType("*models.Communication")).
			Run(func(comm *models.Communication) {
				assert.Equal(tb, models.CommunicationTypeEmail, comm.Type)
				assert.Equal(tb, models.CommunicationDirectionOutgoing, comm.Direction)
				assert.Contains(tb, comm.Content, "Case creation notification sent to reporter")
			}).
			Return(&models.Communication{}, nil).
			Once()

		waitForChan := make(chan bool)
		// Expect single case_access email with all information
		mockEmailService.EXPECT().
			SendTemplatedEmail([]string{reporter.Email}, "case_access", mock.MatchedBy(func(data core.MailerTemplateData) bool {
				return data["AccessURL"] != nil && data["CaseID"] != nil && data["CreatedDate"] != nil
			})).
			Run(func(to []string, templateName string, data core.MailerTemplateData) {
				waitForChan <- true
				assert.Equal(tb, []string{"test@example.com"}, to)
				assert.Equal(tb, "case_access", templateName)
				assert.NotEmpty(tb, data["AccessURL"])
				assert.NotEmpty(tb, data["CaseID"])
				assert.NotEmpty(tb, data["CreatedDate"])
				if caseData.Priority == models.CasePriorityHigh {
					assert.True(tb, data["HighPriorityWarning"].(bool))
					assert.NotEmpty(tb, data["PriorityReason"])
				}
			}).
			Return(nil).
			Once()

		// Act
		waitForAsync(tb, waitForChan, func() {
			createdCase, err := caseService.Create(caseData)

			// Assert
			assert.NoError(tb, err)
			assert.NotNil(tb, createdCase)
			assert.Equal(tb, expectedCase.Type, createdCase.Type)
			assert.Equal(tb, expectedCase.Status, createdCase.Status)
			assert.Equal(tb, expectedCase.Priority, createdCase.Priority)
			assert.Equal(tb, expectedCase.Description, createdCase.Description)
		})
	},
		coreTesting.WithService(typesSvc.CASE_SERVICE, NewCaseService),
	)
}

func TestCaseService_GetCaseByReference(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		caseService := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, caseService)

		db := ctx.DB()

		// Create test data directly in database
		reporter := &models.Reporter{
			Email: "test@example.com",
			Name:  "Test Reporter",
		}
		err := db.Create(reporter).Error
		assert.NoError(tb, err)

		subject := &models.Subject{
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}
		err = db.Create(subject).Error
		assert.NoError(tb, err)

		expectedCase := &models.Case{
			Type:            models.CaseTypeSpam,
			Status:          models.CaseStatusNew,
			Priority:        models.CasePriorityMedium,
			Description:     "Test spam report",
			Source:          models.ReportSourceWebForm, // Add valid source
			ReporterID:      reporter.ID,
			SubjectID:       subject.ID,
			ReferenceNumber: "testref",
		}
		err = db.Create(expectedCase).Error
		assert.NoError(tb, err)

		// Act
		retrievedCase, err := caseService.GetCaseByReference(expectedCase.ReferenceNumber)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, retrievedCase)
		assert.Equal(tb, expectedCase.ReferenceNumber, retrievedCase.ReferenceNumber)
		assert.Equal(tb, expectedCase.Type, retrievedCase.Type)
		assert.Equal(tb, expectedCase.Status, retrievedCase.Status)
	},
		coreTesting.WithService(typesSvc.CASE_SERVICE, NewCaseService),
	)
}

func TestCaseService_GetTimeSeriesMetrics(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		caseService := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, caseService)

		db := ctx.DB()

		// Create test cases with different dates
		now := time.Now().UTC()
		testCases := []struct {
			createdAt time.Time
			status    models.CaseStatus
			priority  models.CasePriority
			source    models.ReportSource
		}{
			{now.Add(-24 * time.Hour), models.CaseStatusNew, models.CasePriorityMedium, models.ReportSourceWebForm},
			{now.Add(-12 * time.Hour), models.CaseStatusInProgress, models.CasePriorityHigh, models.ReportSourceEmail},
			{now, models.CaseStatusResolved, models.CasePriorityLow, models.ReportSourceAPI},
			// Add more test cases to ensure we have open cases
			{now.Add(-6 * time.Hour), models.CaseStatusNew, models.CasePriorityMedium, models.ReportSourceWebForm},
			{now.Add(-3 * time.Hour), models.CaseStatusInProgress, models.CasePriorityHigh, models.ReportSourceEmail},
		}

		// Create reporter and subject first
		reporter := &models.Reporter{
			Email: "test@example.com",
			Name:  "Test Reporter",
		}
		err := db.Create(reporter).Error
		assert.NoError(tb, err)

		subject := &models.Subject{
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}
		err = db.Create(subject).Error
		assert.NoError(tb, err)

		for _, tc := range testCases {
			caseModel := &models.Case{
				Type:        models.CaseTypeSpam,
				Status:      tc.status,
				Priority:    tc.priority,
				Description: "Test case",
				Source:      tc.source,
				ReporterID:  reporter.ID,
				SubjectID:   subject.ID,
				Model: gorm.Model{
					CreatedAt: tc.createdAt,
					UpdatedAt: tc.createdAt,
				},
			}
			err := db.Create(caseModel).Error
			assert.NoError(tb, err)
		}

		// Act
		data, err := caseService.GetTimeSeriesMetrics("open_cases", "24h", nil)

		// Assert
		assert.NoError(tb, err)
		assert.NotEmpty(tb, data)
		assert.Greater(tb, data[0], int64(0)) // Should have at least 1 open case
	},
		coreTesting.WithService(typesSvc.CASE_SERVICE, NewCaseService),
	)
}

func TestGetStatusFlowData_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		caseService := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, caseService)

		db := ctx.DB()

		// Create reporter first
		reporter := &models.Reporter{
			Email: "test@example.com",
			Name:  "Test Reporter",
		}
		err := db.Create(reporter).Error
		assert.NoError(tb, err)

		// Create subject
		subject := &models.Subject{
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}
		err = db.Create(subject).Error
		assert.NoError(tb, err)

		// Create test case with all required fields
		caseModel := &models.Case{
			Type:        models.CaseTypeSpam,
			Status:      models.CaseStatusNew,
			Priority:    models.CasePriorityMedium,
			Description: "Test case",
			Source:      models.ReportSourceWebForm,
			ReporterID:  reporter.ID,
			SubjectID:   subject.ID,
		}
		err = db.Create(caseModel).Error
		assert.NoError(tb, err)

		// Create status history entries
		histories := []models.CaseStatusHistory{
			{
				CaseID:    caseModel.ID,
				OldStatus: models.CaseStatusNew,
				NewStatus: models.CaseStatusInProgress,
				ChangedAt: time.Now().Add(-2 * time.Hour),
				ChangedBy: 1,
			},
			{
				CaseID:    caseModel.ID,
				OldStatus: models.CaseStatusInProgress,
				NewStatus: models.CaseStatusResolved,
				ChangedAt: time.Now().Add(-1 * time.Hour),
				ChangedBy: 1,
			},
			{
				CaseID:    caseModel.ID,
				OldStatus: models.CaseStatusResolved,
				NewStatus: models.CaseStatusClosed,
				ChangedAt: time.Now(),
				ChangedBy: 1,
			},
		}
		for _, h := range histories {
			err := db.Create(&h).Error
			assert.NoError(tb, err)
		}

		// Act
		flowData, err := caseService.GetStatusFlowData(nil)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, flowData)
		assert.Len(tb, flowData.Nodes, 4) // new, in_progress, resolved, closed
		assert.Len(tb, flowData.Links, 3) // 3 transitions

		// Verify nodes contain all statuses
		nodeStatuses := make(map[string]bool)
		for _, node := range flowData.Nodes {
			nodeStatuses[node.Name] = true
		}
		assert.True(tb, nodeStatuses["new"])
		assert.True(tb, nodeStatuses["in_progress"])
		assert.True(tb, nodeStatuses["resolved"])
		assert.True(tb, nodeStatuses["closed"])

		// Verify links contain all transitions
		linkMap := make(map[string]map[string]int64)
		for _, link := range flowData.Links {
			if linkMap[link.Source] == nil {
				linkMap[link.Source] = make(map[string]int64)
			}
			linkMap[link.Source][link.Target] = link.Value
		}

		assert.Equal(tb, int64(1), linkMap["new"]["in_progress"])
		assert.Equal(tb, int64(1), linkMap["in_progress"]["resolved"])
		assert.Equal(tb, int64(1), linkMap["resolved"]["closed"])
	},
		coreTesting.WithService(typesSvc.CASE_SERVICE, NewCaseService),
	)
}

func TestGetStatusFlowData_Empty(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		caseService := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, caseService)

		// Act
		flowData, err := caseService.GetStatusFlowData(nil)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, flowData)
		assert.Empty(tb, flowData.Nodes)
		assert.Empty(tb, flowData.Links)
	},
		coreTesting.WithService(typesSvc.CASE_SERVICE, NewCaseService),
	)
}

func TestGetStatusFlowData_WithFilters(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		caseService := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, caseService)

		db := ctx.DB()

		// Create test cases with different dates
		now := time.Now()
		testCases := []struct {
			createdAt time.Time
			status    models.CaseStatus
		}{
			{now.Add(-24 * time.Hour), models.CaseStatusNew},
			{now.Add(-12 * time.Hour), models.CaseStatusInProgress},
			{now, models.CaseStatusResolved},
		}

		// Create reporter and subject first
		reporter := &models.Reporter{
			Email: "test@example.com",
			Name:  "Test Reporter",
		}
		err := db.Create(reporter).Error
		assert.NoError(tb, err)

		subject := &models.Subject{
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}
		err = db.Create(subject).Error
		assert.NoError(tb, err)

		for _, tc := range testCases {
			caseModel := &models.Case{
				Type:        models.CaseTypeSpam,
				Status:      tc.status,
				Priority:    models.CasePriorityMedium,
				Description: "Test case",
				Source:      models.ReportSourceWebForm,
				ReporterID:  reporter.ID,
				SubjectID:   subject.ID,
				Model: gorm.Model{
					CreatedAt: tc.createdAt,
					UpdatedAt: tc.createdAt,
				},
			}
			err := db.Create(caseModel).Error
			assert.NoError(tb, err)

			// Create status history
			history := models.CaseStatusHistory{
				CaseID:    caseModel.ID,
				OldStatus: models.CaseStatusNew,
				NewStatus: tc.status,
				ChangedAt: tc.createdAt,
				ChangedBy: 1,
			}
			err = db.Create(&history).Error
			assert.NoError(tb, err)
		}

		// Create filter for last 24 hours
		filters := []queryutil.CrudFilter{
			queryutil.FieldGte("changed_at", now.Add(-24*time.Hour)),
		}

		// Act
		flowData, err := caseService.GetStatusFlowData(filters)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, flowData)
		assert.Len(tb, flowData.Nodes, 3) // new, in_progress, resolved
		assert.Len(tb, flowData.Links, 2) // new->in_progress, new->resolved

		// Verify links contain filtered transitions
		linkMap := make(map[string]map[string]int64)
		for _, link := range flowData.Links {
			if linkMap[link.Source] == nil {
				linkMap[link.Source] = make(map[string]int64)
			}
			linkMap[link.Source][link.Target] = link.Value
		}

		assert.Equal(tb, int64(1), linkMap["new"]["in_progress"])
	},
		coreTesting.WithService(typesSvc.CASE_SERVICE, NewCaseService),
	)
}

func TestGetStatusFlowData_ValidTimeRange(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		caseService := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, caseService)

		db := ctx.DB()

		// Define a time range
		timeRange := "7d"
		startDate, endDate, err := util.ParseTimeRange(timeRange)
		assert.NoError(tb, err)

		// Create reporter and subject first
		reporter := &models.Reporter{
			Email: "test@example.com",
			Name:  "Test Reporter",
		}
		err = db.Create(reporter).Error
		assert.NoError(tb, err)

		subject := &models.Subject{
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}
		err = db.Create(subject).Error
		assert.NoError(tb, err)

		// Create a test case within the time range
		caseModel := &models.Case{
			Type:        models.CaseTypeSpam,
			Status:      models.CaseStatusNew,
			Priority:    models.CasePriorityMedium,
			Description: "Test case",
			Source:      models.ReportSourceWebForm,
			ReporterID:  reporter.ID,
			SubjectID:   subject.ID,
		}
		err = db.Create(caseModel).Error
		assert.NoError(tb, err)

		// Create status history entries within the time range
		histories := []models.CaseStatusHistory{
			{
				CaseID:    caseModel.ID,
				OldStatus: models.CaseStatusNew,
				NewStatus: models.CaseStatusInProgress,
				ChangedAt: startDate.Add(time.Hour),
				ChangedBy: 1,
			},
			{
				CaseID:    caseModel.ID,
				OldStatus: models.CaseStatusInProgress,
				NewStatus: models.CaseStatusResolved,
				ChangedAt: endDate.Add(-time.Hour),
				ChangedBy: 1,
			},
		}
		for _, h := range histories {
			err := db.Create(&h).Error
			assert.NoError(tb, err)
		}

		// Create a filter with a valid time_range
		filters := []queryutil.CrudFilter{
			queryutil.FieldEqual("time_range", timeRange),
		}

		// Act
		flowData, err := caseService.GetStatusFlowData(filters)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, flowData)
		assert.Len(tb, flowData.Nodes, 3) // new, in_progress, resolved
		assert.Len(tb, flowData.Links, 2) // 2 transitions

		// Verify nodes contain all statuses
		nodeStatuses := make(map[string]bool)
		for _, node := range flowData.Nodes {
			nodeStatuses[node.Name] = true
		}
		assert.True(tb, nodeStatuses["new"])
		assert.True(tb, nodeStatuses["in_progress"])
		assert.True(tb, nodeStatuses["resolved"])

		// Verify links contain all transitions
		linkMap := make(map[string]map[string]int64)
		for _, link := range flowData.Links {
			if linkMap[link.Source] == nil {
				linkMap[link.Source] = make(map[string]int64)
			}
			linkMap[link.Source][link.Target] = link.Value
		}

		assert.Equal(tb, int64(1), linkMap["new"]["in_progress"])
		assert.Equal(tb, int64(1), linkMap["in_progress"]["resolved"])
	},
		coreTesting.WithService(typesSvc.CASE_SERVICE, NewCaseService),
	)
}

func TestGetTypeSourceMatrix_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		caseService := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, caseService)

		db := ctx.DB()

		// Create test data with different types and sources
		now := time.Now()
		testCases := []struct {
			caseType models.CaseType
			source   models.ReportSource
			count    int
		}{
			{models.CaseTypeSpam, models.ReportSourceWebForm, 3},
			{models.CaseTypeSpam, models.ReportSourceEmail, 2},
			{models.CaseTypeHarassment, models.ReportSourceWebForm, 1},
			{models.CaseTypeHarassment, models.ReportSourceAPI, 4},
		}

		// Create reporter and subject first
		reporter := &models.Reporter{
			Email: "test@example.com",
			Name:  "Test Reporter",
		}
		err := db.Create(reporter).Error
		assert.NoError(tb, err)

		subject := &models.Subject{
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}
		err = db.Create(subject).Error
		assert.NoError(tb, err)

		// Create test cases
		for _, tc := range testCases {
			for i := 0; i < tc.count; i++ {
				caseModel := &models.Case{
					Type:        tc.caseType,
					Status:      models.CaseStatusNew,
					Priority:    models.CasePriorityMedium,
					Description: "Test case",
					Source:      tc.source,
					ReporterID:  reporter.ID,
					SubjectID:   subject.ID,
					Model: gorm.Model{
						CreatedAt: now.Add(-time.Duration(i) * time.Hour),
					},
				}
				err := db.Create(caseModel).Error
				assert.NoError(tb, err)
			}
		}

		// Act
		results, err := caseService.GetTypeSourceMatrix("7d", nil)

		// Assert
		assert.NoError(tb, err)
		assert.NotEmpty(tb, results)

		// Verify counts match expected
		resultMap := make(map[string]map[string]int64)
		for _, result := range results {
			if resultMap[string(result.CaseType)] == nil {
				resultMap[string(result.CaseType)] = make(map[string]int64)
			}
			resultMap[string(result.CaseType)][string(result.ReportSource)] = result.CaseCount
		}

		assert.Equal(tb, int64(3), resultMap["spam"]["web_form"])
		assert.Equal(tb, int64(2), resultMap["spam"]["email"])
		assert.Equal(tb, int64(1), resultMap["harassment"]["web_form"])
		assert.Equal(tb, int64(4), resultMap["harassment"]["api"])
	},
		coreTesting.WithService(typesSvc.CASE_SERVICE, NewCaseService),
	)
}

func TestGetTypeSourceMatrix_Empty(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		caseService := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, caseService)

		// Act
		results, err := caseService.GetTypeSourceMatrix("7d", nil)

		// Assert
		assert.NoError(tb, err)
		assert.Empty(tb, results)
	},
		coreTesting.WithService(typesSvc.CASE_SERVICE, NewCaseService),
	)
}

func TestGetTypeSourceMatrix_InvalidTimeRange(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		caseService := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, caseService)

		// Act
		_, err := caseService.GetTypeSourceMatrix("invalid", nil)

		// Assert
		assert.Error(tb, err)
		assert.ErrorContains(tb, err, "invalid timeRange")
	},
		coreTesting.WithService(typesSvc.CASE_SERVICE, NewCaseService),
	)
}

func TestGetTypeSourceMatrix_WithFilters(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		caseService := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, caseService)

		db := ctx.DB()

		// Create test data
		reporter := &models.Reporter{
			Email: "test@example.com",
			Name:  "Test Reporter",
		}
		err := db.Create(reporter).Error
		assert.NoError(tb, err)

		subject := &models.Subject{
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}
		err = db.Create(subject).Error
		assert.NoError(tb, err)

		// Create cases with different priorities
		now := time.Now()
		for i := 0; i < 5; i++ {
			caseModel := &models.Case{
				Type:        models.CaseTypeSpam,
				Status:      models.CaseStatusNew,
				Priority:    models.CasePriorityMedium,
				Description: "Test case",
				Source:      models.ReportSourceWebForm,
				ReporterID:  reporter.ID,
				SubjectID:   subject.ID,
				Model: gorm.Model{
					CreatedAt: now.Add(-time.Duration(i) * time.Hour),
				},
			}
			err := db.Create(caseModel).Error
			assert.NoError(tb, err)
		}

		// Create filter for high priority cases (should match none)
		filters := []queryutil.CrudFilter{
			queryutil.FieldEqual("priority", models.CasePriorityHigh),
		}

		// Act
		results, err := caseService.GetTypeSourceMatrix("7d", filters)

		// Assert
		assert.NoError(tb, err)
		assert.Empty(tb, results)
	},
		coreTesting.WithService(typesSvc.CASE_SERVICE, NewCaseService),
	)
}

func TestCaseService_UpdateStatus(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		caseService := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, caseService)

		mockEmailService := core.GetService[typesSvc.EmailService](ctx, typesSvc.EMAIL_SERVICE).(*mocks.MockEmailService)

		db := ctx.DB()

		// Create test data directly in database
		reporter := &models.Reporter{
			Email: "test@example.com",
			Name:  "Test Reporter",
		}
		err := db.Create(reporter).Error
		require.NoError(tb, err)

		subject := &models.Subject{
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}
		err = db.Create(subject).Error
		require.NoError(tb, err)

		initialCase := &models.Case{
			Type:            models.CaseTypeSpam,
			Status:          models.CaseStatusNew,
			Priority:        models.CasePriorityMedium,
			Description:     "Test spam report",
			Source:          models.ReportSourceWebForm,
			ReporterID:      reporter.ID,
			SubjectID:       subject.ID,
			ReferenceNumber: "testref",
		}
		err = db.Create(initialCase).Error
		require.NoError(tb, err)

		newStatus := models.CaseStatusResolved

		waitForChan := make(chan bool)

		// Get mock reporter service
		mockReporterService := core.GetService[*mocks.MockReporterService](ctx, typesSvc.REPORTER_SERVICE)

		// Set up mock expectations
		mockReporterService.EXPECT().
			GetByID(reporter.ID).
			Return(reporter, nil).
			Once()

		// Get mock communication service
		mockCommService := core.GetService[*mocks.MockCommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)

		// Mock the email service
		mockEmailService.EXPECT().
			GenerateCaseThreadID(initialCase.ID, initialCase.ReferenceNumber).
			Return("test-thread-id").
			Once()

		mockEmailService.EXPECT().
			SendTemplatedEmail([]string{reporter.Email}, "case_status_updated", mock.MatchedBy(func(data core.MailerTemplateData) bool {
				return data["OldStatus"] != nil && data["NewStatus"] != nil
			})).
			Run(func(to []string, templateName string, data core.MailerTemplateData) {
				waitForChan <- true
				assert.Equal(tb, []string{"test@example.com"}, to)
				assert.Equal(tb, "case_status_updated", templateName)
				assert.Equal(tb, models.CaseStatusNew, data["OldStatus"])
				assert.Equal(tb, models.CaseStatusResolved, data["NewStatus"])
			}).
			Return(nil).
			Once()

		// Mock the communication service
		mockCommService.EXPECT().
			Create(mock.AnythingOfType("*models.Communication")).
			Run(func(comm *models.Communication) {
				assert.Equal(tb, models.CommunicationTypeEmail, comm.Type)
				assert.Equal(tb, models.CommunicationDirectionOutgoing, comm.Direction)
				assert.Contains(tb, comm.Content, "Case status update notification sent to reporter")
			}).
			Return(&models.Communication{}, nil).
			Once()

		// Act
		waitForAsync(tb, waitForChan, func() {
			err = caseService.UpdateStatus(initialCase.ID, newStatus)
			assert.NoError(tb, err)

			// Retrieve the updated case from the database
			var updatedCase models.Case
			err = db.First(&updatedCase, initialCase.ID).Error
			assert.NoError(tb, err)

			// Assert that the status has been updated
			assert.Equal(tb, newStatus, updatedCase.Status)
		})
	},
		coreTesting.WithService(typesSvc.CASE_SERVICE, NewCaseService),
	)
}
