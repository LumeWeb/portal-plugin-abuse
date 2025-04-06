package service

import (
	"context"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

func TestAbuseReportService_SubmitReport(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		abuseReportService := core.GetService[typesSvc.AbuseReportService](ctx, typesSvc.ABUSE_REPORT_SERVICE)
		assert.NotNil(tb, abuseReportService)

		mockCaseService := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		mockReporterService := core.GetService[*mocks.MockReporterService](ctx, typesSvc.REPORTER_SERVICE)
		mockSubjectService := core.GetService[*mocks.MockSubjectService](ctx, typesSvc.SUBJECT_SERVICE)

		caseData := &models.Case{
			Type:        models.CaseTypeSpam,
			Status:      models.CaseStatusNew,
			Priority:    models.CasePriorityMedium,
			Description: "Test spam report",
			Reporter: models.Reporter{
				Email: "test@example.com",
				Name:  "Test Reporter",
			},
			Subject: models.Subject{
				Identifier: []byte("testhash"),
				Type:       models.SubjectTypeHash,
			},
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

		// Mock ReporterService.GetByEmail first
		mockReporterService.EXPECT().
			GetByEmail(caseData.Reporter.Email).
			Return(nil, db.ErrRecordNotFound).
			Once()

		// Then mock ReporterService.Create
		mockReporterService.EXPECT().
			Create(mock.AnythingOfType("*models.Reporter")).
			Return(&models.Reporter{
				Model: gorm.Model{
					ID:        1,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Email: caseData.Reporter.Email,
				Name:  caseData.Reporter.Name,
			}, nil).
			Once()

		// Mock SubjectService.Create with exact expected subject
		expectedSubject := &models.Subject{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Identifier: caseData.Subject.Identifier,
			Type:       caseData.Subject.Type,
		}
		mockSubjectService.EXPECT().
			Create(mock.AnythingOfType("*models.Subject")).
			Return(expectedSubject, nil).
			Once()

		// Mock CaseService.Create with expected case data
		mockCaseService.EXPECT().
			Create(mock.AnythingOfType("*models.Case")).
			Return(expectedCase, nil).
			Once()

		// Act
		createdCase, err := abuseReportService.SubmitReport(context.Background(), caseData)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, createdCase)
		assert.Equal(tb, expectedCase.Type, createdCase.Type)
		assert.Equal(tb, expectedCase.Status, createdCase.Status)
		assert.Equal(tb, expectedCase.Priority, createdCase.Priority)
		assert.Equal(tb, expectedCase.Description, createdCase.Description)
	},
		coreTesting.WithService(typesSvc.ABUSE_REPORT_SERVICE, NewAbuseReportService),
	)
}

func TestAbuseReportService_GetReportStatus(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		abuseReportService := core.GetService[typesSvc.AbuseReportService](ctx, typesSvc.ABUSE_REPORT_SERVICE)
		assert.NotNil(tb, abuseReportService)

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
		retrievedCase, err := abuseReportService.GetReportStatus(context.Background(), expectedCase.ReferenceNumber)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, retrievedCase)
		assert.Equal(tb, expectedCase.ReferenceNumber, retrievedCase.ReferenceNumber)
		assert.Equal(tb, expectedCase.Type, retrievedCase.Type)
		assert.Equal(tb, expectedCase.Status, retrievedCase.Status)
	},
		coreTesting.WithService(typesSvc.ABUSE_REPORT_SERVICE, NewAbuseReportService),
	)
}
