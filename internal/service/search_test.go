package service

import (
	"context"
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

func TestSearchService_SearchCases(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		searchService := core.GetService[typesSvc.SearchService](ctx, typesSvc.SEARCH_SERVICE)
		assert.NotNil(tb, searchService)

		// Create test data directly in database
		reporter := &models.Reporter{
			Email: "test@example.com",
			Name:  "Test Reporter",
		}
		err := ctx.DB().Create(reporter).Error
		require.NoError(tb, err)

		subject := &models.Subject{
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}
		err = ctx.DB().Create(subject).Error
		require.NoError(tb, err)

		case1 := &models.Case{
			Model:           gorm.Model{ID: 1},
			Type:            models.CaseTypeSpam,
			Status:          models.CaseStatusNew,
			Priority:        models.CasePriorityMedium,
			Description:     "Test spam report",
			Source:          models.ReportSourceWebForm,
			ReporterID:      reporter.ID,
			SubjectID:       subject.ID,
			ReferenceNumber: "testref1",
		}
		err = ctx.DB().Create(case1).Error
		require.NoError(tb, err)

		case2 := &models.Case{
			Model:           gorm.Model{ID: 2},
			Type:            models.CaseTypeHarassment,
			Status:          models.CaseStatusInProgress,
			Priority:        models.CasePriorityHigh,
			Description:     "Test harassment report",
			Source:          models.ReportSourceWebForm,
			ReporterID:      reporter.ID,
			SubjectID:       subject.ID,
			ReferenceNumber: "testref2",
		}
		err = ctx.DB().Create(case2).Error
		require.NoError(tb, err)

		// Act
		query := "Test"
		filters := []queryutil.CrudFilter{}
		pagination := queryutil.DefaultPagination
		cases, total, err := searchService.SearchCases(context.Background(), query, filters, pagination)

		// Assert
		assert.NoError(tb, err)
		assert.Equal(tb, int64(2), total)
		assert.Len(tb, cases, 2)
	}, coreTesting.WithService(typesSvc.SEARCH_SERVICE, NewSearchService))
}

func TestSearchService_SearchReporters(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		searchService := core.GetService[typesSvc.SearchService](ctx, typesSvc.SEARCH_SERVICE)
		assert.NotNil(tb, searchService)

		// Create test data directly in database
		reporter1 := &models.Reporter{
			Model: gorm.Model{ID: 1},
			Email: "test1@example.com",
			Name:  "Test Reporter 1",
		}
		err := ctx.DB().Create(reporter1).Error
		require.NoError(tb, err)

		reporter2 := &models.Reporter{
			Model: gorm.Model{ID: 2},
			Email: "test2@example.com",
			Name:  "Test Reporter 2",
		}
		err = ctx.DB().Create(reporter2).Error
		require.NoError(tb, err)

		// Act
		query := "Test"
		filters := []queryutil.CrudFilter{}
		pagination := queryutil.DefaultPagination
		reporters, total, err := searchService.SearchReporters(context.Background(), query, filters, pagination)

		// Assert
		assert.NoError(tb, err)
		assert.Equal(tb, int64(2), total)
		assert.Len(tb, reporters, 2)
	}, coreTesting.WithService(typesSvc.SEARCH_SERVICE, NewSearchService))
}

func TestSearchService_GlobalSearch(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		searchService := core.GetService[typesSvc.SearchService](ctx, typesSvc.SEARCH_SERVICE)
		assert.NotNil(tb, searchService)

		// Create test data directly in database
		reporter1 := &models.Reporter{
			Model: gorm.Model{ID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Email: "test1@example.com",
			Name:  "Test Reporter 1",
		}
		err := ctx.DB().Create(reporter1).Error
		require.NoError(tb, err)

		subject := &models.Subject{
			Model:      gorm.Model{ID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Identifier: []byte("testhash"),
			Type:       models.SubjectTypeHash,
		}
		err = ctx.DB().Create(subject).Error
		require.NoError(tb, err)

		case1 := &models.Case{
			Model:           gorm.Model{ID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Type:            models.CaseTypeSpam,
			Status:          models.CaseStatusNew,
			Priority:        models.CasePriorityMedium,
			Description:     "Test spam report",
			Source:          models.ReportSourceWebForm,
			ReporterID:      reporter1.ID,
			SubjectID:       subject.ID,
			ReferenceNumber: "testref1",
		}
		err = ctx.DB().Create(case1).Error
		require.NoError(tb, err)

		// Act
		query := "Test"
		pagination := queryutil.DefaultPagination
		result, err := searchService.GlobalSearch(context.Background(), query, pagination)

		// Assert
		assert.NoError(tb, err)
		assert.NotNil(tb, result)
		assert.Equal(tb, int64(1), result.Cases.Total)
		assert.Equal(tb, int64(1), result.Reporters.Total)
		assert.Len(tb, result.Cases.Items, 1)
		assert.Len(tb, result.Reporters.Items, 1)
	}, coreTesting.WithService(typesSvc.SEARCH_SERVICE, NewSearchService))
}
