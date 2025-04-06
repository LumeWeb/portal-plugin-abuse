package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"go.lumeweb.com/queryutil"
	"gorm.io/gorm"
)

func TestReporterService_Create(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*ReporterServiceDefault)
		assert.NotNil(tb, reporterService)

		reporter := &models.Reporter{
			Email: "test@example.com",
			Name:  "Test Reporter",
		}

		createdReporter, err := reporterService.Create(reporter)

		require.NoError(tb, err)
		assert.NotNil(tb, createdReporter)
		assert.Equal(tb, reporter.Email, createdReporter.Email)
		assert.Equal(tb, reporter.Name, createdReporter.Name)

		var retrievedReporter models.Reporter
		err = ctx.DB().First(&retrievedReporter, createdReporter.ID).Error
		require.NoError(tb, err)

		assert.Equal(tb, reporter.Email, retrievedReporter.Email)
		assert.Equal(tb, reporter.Name, retrievedReporter.Name)

	},
		coreTesting.WithService(typesSvc.REPORTER_SERVICE, NewReporterService),
	)
}

func TestReporterService_GetByID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*ReporterServiceDefault)
		assert.NotNil(tb, reporterService)

		reporterID := uint(1)
		expectedReporter := &models.Reporter{
			Model: gorm.Model{ID: reporterID},
			Email: "test@example.com",
			Name:  "Test Reporter",
		}

		err := ctx.DB().Create(expectedReporter).Error
		require.NoError(tb, err)

		retrievedReporter, err := reporterService.GetByID(reporterID)

		require.NoError(tb, err)
		assert.NotNil(tb, retrievedReporter)
		assert.Equal(tb, expectedReporter.ID, retrievedReporter.ID)
		assert.Equal(tb, expectedReporter.Email, retrievedReporter.Email)
		assert.Equal(tb, expectedReporter.Name, retrievedReporter.Name)
	},
		coreTesting.WithService(typesSvc.REPORTER_SERVICE, NewReporterService),
	)
}

func TestReporterService_GetByEmail(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*ReporterServiceDefault)
		assert.NotNil(tb, reporterService)

		email := "test@example.com"
		expectedReporter := &models.Reporter{
			Model: gorm.Model{ID: 1},
			Email: email,
			Name:  "Test Reporter",
		}

		err := ctx.DB().Create(expectedReporter).Error
		require.NoError(tb, err)

		retrievedReporter, err := reporterService.GetByEmail(email)

		require.NoError(tb, err)
		assert.NotNil(tb, retrievedReporter)
		assert.Equal(tb, expectedReporter.ID, retrievedReporter.ID)
		assert.Equal(tb, expectedReporter.Email, retrievedReporter.Email)
		assert.Equal(tb, expectedReporter.Name, retrievedReporter.Name)
	},
		coreTesting.WithService(typesSvc.REPORTER_SERVICE, NewReporterService),
	)
}

func TestReporterService_GetByEmail_InvalidEmail(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*ReporterServiceDefault)
		assert.NotNil(tb, reporterService)

		email := "invalid-email"

		retrievedReporter, err := reporterService.GetByEmail(email)

		assert.ErrorIs(tb, err, ErrInvalidReporterEmail)
		assert.Nil(tb, retrievedReporter)
	},
		coreTesting.WithService(typesSvc.REPORTER_SERVICE, NewReporterService),
	)
}

func TestReporterService_GetByEmail_NotFound(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*ReporterServiceDefault)
		assert.NotNil(tb, reporterService)

		email := "test@example.com"

		retrievedReporter, err := reporterService.GetByEmail(email)

		assert.ErrorIs(tb, err, db.ErrRecordNotFound)
		assert.Nil(tb, retrievedReporter)
	},
		coreTesting.WithService(typesSvc.REPORTER_SERVICE, NewReporterService),
	)
}

func TestReporterService_List(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*ReporterServiceDefault)
		assert.NotNil(tb, reporterService)

		reporter1 := &models.Reporter{
			Model: gorm.Model{ID: 1},
			Email: "test1@example.com",
			Name:  "Test Reporter 1",
		}
		reporter2 := &models.Reporter{
			Model: gorm.Model{ID: 2},
			Email: "test2@example.com",
			Name:  "Test Reporter 2",
		}

		err := ctx.DB().Create(reporter1).Error
		require.NoError(tb, err)
		err = ctx.DB().Create(reporter2).Error
		require.NoError(tb, err)

		filters := []queryutil.CrudFilter{}
		sorts := []queryutil.Sort{}
		pagination := queryutil.DefaultPagination

		retrievedReporters, total, err := reporterService.List(filters, sorts, pagination)

		require.NoError(tb, err)
		assert.Equal(tb, int64(2), total)
		assert.Len(tb, retrievedReporters, 2)
		assert.Equal(tb, reporter1.Email, retrievedReporters[0].Email)
		assert.Equal(tb, reporter2.Email, retrievedReporters[1].Email)
	},
		coreTesting.WithService(typesSvc.REPORTER_SERVICE, NewReporterService),
	)
}

func TestReporterService_Update(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*ReporterServiceDefault)
		assert.NotNil(tb, reporterService)

		reporterID := uint(1)
		initialReporter := &models.Reporter{
			Model: gorm.Model{ID: reporterID},
			Email: "test@example.com",
			Name:  "Test Reporter",
		}

		err := ctx.DB().Create(initialReporter).Error
		require.NoError(tb, err)

		updatedReporter := &models.Reporter{
			Model: gorm.Model{ID: reporterID},
			Email: "updated@example.com",
			Name:  "Updated Reporter",
		}

		err = reporterService.Update(updatedReporter)

		require.NoError(tb, err)

		var retrievedReporter models.Reporter
		err = ctx.DB().First(&retrievedReporter, reporterID).Error
		require.NoError(tb, err)

		assert.Equal(tb, updatedReporter.Email, retrievedReporter.Email)
		assert.Equal(tb, updatedReporter.Name, retrievedReporter.Name)
	},
		coreTesting.WithService(typesSvc.REPORTER_SERVICE, NewReporterService),
	)
}

func TestReporterService_Update_NotFound(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*ReporterServiceDefault)
		assert.NotNil(tb, reporterService)

		reporterID := uint(1)

		updatedReporter := &models.Reporter{
			Model: gorm.Model{ID: reporterID},
			Email: "updated@example.com",
			Name:  "Updated Reporter",
		}

		err := reporterService.Update(updatedReporter)

		assert.Error(tb, err)
	},
		coreTesting.WithService(typesSvc.REPORTER_SERVICE, NewReporterService),
	)
}

func TestReporterService_Create_DBError(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*ReporterServiceDefault)
		assert.NotNil(tb, reporterService)

		reporter := &models.Reporter{
			Email: "", // Invalid email to trigger DB error
			Name:  "Test Reporter",
		}

		createdReporter, err := reporterService.Create(reporter)

		assert.Error(tb, err)
		assert.Nil(tb, createdReporter)
	},
		coreTesting.WithService(typesSvc.REPORTER_SERVICE, NewReporterService),
	)
}

func TestReporterService_GetByID_NotFound(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*ReporterServiceDefault)
		assert.NotNil(tb, reporterService)

		reporterID := uint(1)

		retrievedReporter, err := reporterService.GetByID(reporterID)

		assert.ErrorIs(tb, err, db.ErrRecordNotFound)
		assert.Nil(tb, retrievedReporter)
	},
		coreTesting.WithService(typesSvc.REPORTER_SERVICE, NewReporterService),
	)
}

func TestReporterService_Update_DBError(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*ReporterServiceDefault)
		assert.NotNil(tb, reporterService)

		reporterID := uint(1)
		initialReporter := &models.Reporter{
			Model: gorm.Model{ID: reporterID},
			Email: "test@example.com",
			Name:  "Test Reporter",
		}

		err := ctx.DB().Create(initialReporter).Error
		require.NoError(tb, err)

		updatedReporter := &models.Reporter{
			Model: gorm.Model{ID: reporterID},
			Email: "", // Invalid email to trigger DB error
			Name:  "Updated Reporter",
		}

		err = reporterService.Update(updatedReporter)

		assert.Error(tb, err)
	},
		coreTesting.WithService(typesSvc.REPORTER_SERVICE, NewReporterService),
	)
}

func TestReporterService_Create_InvalidEmail(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*ReporterServiceDefault)
		assert.NotNil(tb, reporterService)

		reporter := &models.Reporter{
			Email: "invalid-email",
			Name:  "Test Reporter",
		}

		createdReporter, err := reporterService.Create(reporter)

		assert.Error(tb, err)
		assert.Nil(tb, createdReporter)
	},
		coreTesting.WithService(typesSvc.REPORTER_SERVICE, NewReporterService),
	)
}

func TestReporterService_List_Empty(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		reporterService := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*ReporterServiceDefault)
		assert.NotNil(tb, reporterService)

		filters := []queryutil.CrudFilter{}
		sorts := []queryutil.Sort{}
		pagination := queryutil.DefaultPagination

		retrievedReporters, total, err := reporterService.List(filters, sorts, pagination)

		require.NoError(tb, err)
		assert.Equal(tb, int64(0), total)
		assert.Len(tb, retrievedReporters, 0)
	},
		coreTesting.WithService(typesSvc.REPORTER_SERVICE, NewReporterService),
	)
}
