package api

import (
	"encoding/json"
	"fmt"
	"go.lumeweb.com/queryutil"
	"go.lumeweb.com/queryutil/filter/generator"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestGetCaseAnalytics_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockAnalytics := &typesSvc.CaseAnalytics{
			TotalCases:       100,
			OpenCases:        25,
			NewCasesInRange:  10,
			NeedsReviewCount: 5,
			StatusBreakdown: map[models.CaseStatus]int64{
				models.CaseStatusNew:    25,
				models.CaseStatusClosed: 75,
			},
			CaseTypeBreakdown: map[models.CaseType]int64{
				models.CaseTypeSpam:       60,
				models.CaseTypeHarassment: 40,
			},
			AvgResolutionSeconds: 86400,
		}

		mockCaseSvc.EXPECT().GetAnalytics(mock.Anything).Return(mockAnalytics, nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases", nil)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.CaseAnalyticsResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)

		assert.Equal(tb, int64(100), response.TotalCases)
		assert.Equal(tb, int64(25), response.OpenCases)
		assert.Len(tb, response.StatusDistribution, 2)
		assert.Len(tb, response.CaseTypeDistribution, 2)
		assert.Equal(tb, int64(86400), response.ResolutionMetrics.AverageSeconds)
	})
}

func TestGetCaseAnalytics_ServiceError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockCaseSvc.EXPECT().GetAnalytics(mock.Anything).Return(nil, fmt.Errorf("analytics error")).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases", nil)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusInternalServerError, w.Code)
	})
}

func TestGet7DayAnalytics_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockAnalytics := &typesSvc.CaseAnalytics{
			NewCasesInRange: 15,
			ResolutionTrends: map[time.Time]int64{
				time.Now().AddDate(0, 0, -1): 5,
				time.Now().AddDate(0, 0, -2): 10,
			},
		}

		mockCaseSvc.EXPECT().Get7DayAnalytics().Return(mockAnalytics, nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/7d", nil)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.CaseAnalyticsResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)

		assert.Equal(tb, int64(15), response.NewCases)
		assert.Len(tb, response.ResolutionMetrics.DailyTrends, 2)
	})
}

func TestGet30DayAnalytics_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockAnalytics := &typesSvc.CaseAnalytics{
			NewCasesInRange: 50,
			ResolutionTrends: map[time.Time]int64{
				time.Now().AddDate(0, 0, -7):  20,
				time.Now().AddDate(0, 0, -14): 30,
			},
		}

		mockCaseSvc.EXPECT().Get30DayAnalytics().Return(mockAnalytics, nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/30d", nil)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.CaseAnalyticsResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)

		assert.Equal(tb, int64(50), response.NewCases)
		assert.Len(tb, response.ResolutionMetrics.DailyTrends, 2)
	})
}

func TestGet24HourAnalytics_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockAnalytics := &typesSvc.CaseAnalytics{
			NewCasesInRange: 5,
			ResolutionTrends: map[time.Time]int64{
				time.Now().Add(-12 * time.Hour): 3,
				time.Now().Add(-6 * time.Hour):  2,
			},
		}

		mockCaseSvc.EXPECT().Get24HourAnalytics().Return(mockAnalytics, nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/24h", nil)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.CaseAnalyticsResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)

		assert.Equal(tb, int64(5), response.NewCases)
		assert.Len(tb, response.ResolutionMetrics.DailyTrends, 2)
	})
}

func TestHandleCaseTimeSeriesAnalytics_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockData := []int64{10, 20, 30}
		mockCaseSvc.EXPECT().GetTimeSeriesMetrics("open_cases", "7d", mock.Anything).Return(mockData, nil).Once()

		// Generate query params with valid time range
		queryGen := generator.NewDefaultQueryParamGenerator()
		queryParams, err := queryGen.Generate([]queryutil.CrudFilter{
			queryutil.FieldEqual("metric", "open_cases"),
			queryutil.FieldEqual("time_range", "7d"),
		})
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/time-series", nil)
		req.URL.RawQuery = queryutil.ToQueryString(queryParams)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response []int64
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, mockData, response)
	})
}

func TestHandleCaseTimeSeriesAnalytics_MissingParams(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/time-series", nil)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusBadRequest, w.Code)
	})
}

func TestHandleCaseTimeSeriesAnalytics_ServiceError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockCaseSvc.EXPECT().GetTimeSeriesMetrics("open_cases", "7d", mock.Anything).Return(nil, fmt.Errorf("service error")).Once()

		// Generate query params with valid time range
		queryGen := generator.NewDefaultQueryParamGenerator()
		queryParams, err := queryGen.Generate([]queryutil.CrudFilter{
			queryutil.FieldEqual("metric", "open_cases"),
			queryutil.FieldEqual("time_range", "7d"),
		})
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/time-series", nil)
		req.URL.RawQuery = queryutil.ToQueryString(queryParams)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusInternalServerError, w.Code)
	})
}

func TestGetStatusFlow_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockFlowData := &typesSvc.StatusFlowGraph{
			Nodes: []typesSvc.StatusFlowNode{
				{Name: "new"},
				{Name: "in_progress"},
			},
			Links: []typesSvc.StatusFlowLink{
				{Source: "new", Target: "in_progress", Value: 10},
			},
		}

		mockCaseSvc.EXPECT().GetStatusFlowData(mock.MatchedBy(func(filters []queryutil.CrudFilter) bool {
			// Verify we have the expected filters using DeepFindFilterWithOperator
			dateGteFilter := queryutil.DeepFindFilterWithOperator(filters, "transition_date", queryutil.OpGte)
			dateLteFilter := queryutil.DeepFindFilterWithOperator(filters, "transition_date", queryutil.OpLte)

			return dateGteFilter != nil &&
				dateLteFilter != nil
		})).Return(mockFlowData, nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/status-flow", nil)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.StatusFlowGraphResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)

		assert.Len(tb, response.Nodes, 2)
		assert.Len(tb, response.Links, 1)
		assert.Equal(tb, "new", response.Links[0].Source)
		assert.Equal(tb, "in_progress", response.Links[0].Target)
		assert.Equal(tb, int64(10), response.Links[0].Value)
	})
}

func TestGetStatusFlow_ServiceError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockCaseSvc.EXPECT().GetStatusFlowData(mock.Anything).Return(nil, fmt.Errorf("service error")).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/status-flow", nil)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusInternalServerError, w.Code)
	})
}

func TestGetStatusFlow_InvalidDateRange(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/status-flow?start_date=invalid&end_date=invalid", nil)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusBadRequest, w.Code)
	})
}

func TestGetTypeSourceMatrix_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, mockCaseSvc)

		mockData := []models.CaseTypeSourceBreakdown{
			{
				CaseDate:     "2024-01-01",
				CaseType:     models.CaseTypeSpam,
				ReportSource: models.ReportSourceWebForm,
				CaseCount:    5,
			},
			{
				CaseDate:     "2024-01-01",
				CaseType:     models.CaseTypeHarassment,
				ReportSource: models.ReportSourceEmail,
				CaseCount:    3,
			},
		}

		mockCaseSvc.EXPECT().GetTypeSourceMatrix("7d", mock.Anything).Return(mockData, nil).Once()

		// Arrange
		mockCaseSvc = core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, mockCaseSvc)

		mockData = []models.CaseTypeSourceBreakdown{
			{
				CaseDate:     "2024-01-01",
				CaseType:     models.CaseTypeSpam,
				ReportSource: models.ReportSourceWebForm,
				CaseCount:    5,
			},
			{
				CaseDate:     "2024-01-01",
				CaseType:     models.CaseTypeHarassment,
				ReportSource: models.ReportSourceEmail,
				CaseCount:    3,
			},
		}
		// Generate query params with valid time range
		queryGen := generator.NewDefaultQueryParamGenerator()
		queryParams, err := queryGen.Generate([]queryutil.CrudFilter{
			queryutil.FieldEqual("time_range", "7d"),
		})
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/type-source-matrix", nil)
		req.URL.RawQuery = queryutil.ToQueryString(queryParams)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.CaseTypeSourceMatrixResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Len(tb, response.Items, 2)
		assert.Equal(tb, "spam", response.Items[0].CaseType)
		assert.Equal(tb, "web_form", response.Items[0].ReportSource)
		assert.Equal(tb, int64(5), response.Items[0].CaseCount)
	})
}

func TestGetTypeSourceMatrix_MissingTimeRange(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/type-source-matrix", nil)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusBadRequest, w.Code)
	})
}

func TestGetTypeSourceMatrix_InvalidTimeRange(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/type-source-matrix?time_range=invalid", nil)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusBadRequest, w.Code)
	})
}

func TestGetTypeSourceMatrix_ServiceError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, mockCaseSvc)

		mockCaseSvc.EXPECT().GetTypeSourceMatrix("7d", mock.Anything).Return(nil, fmt.Errorf("service error")).Once()

		// Generate query params with valid time range
		queryGen := generator.NewDefaultQueryParamGenerator()
		queryParams, err := queryGen.Generate([]queryutil.CrudFilter{
			queryutil.FieldEqual("time_range", "7d"),
		})
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/type-source-matrix", nil)
		req.URL.RawQuery = queryutil.ToQueryString(queryParams)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusInternalServerError, w.Code)
	})
}

func TestGetStatusFlow_WithFilters(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		assert.NotNil(tb, mockCaseSvc)

		now := time.Now()
		startDate := now.Add(-24 * time.Hour)
		endDate := now

		// Create proper query filters using queryutil with grouped date range
		filters := []queryutil.CrudFilter{
			queryutil.And(
				queryutil.FieldGte("changed_at", startDate),
				queryutil.FieldLte("changed_at", endDate),
			),
			queryutil.FieldEqual("from_status", "new"),
			queryutil.FieldEqual("to_status", "in_progress"),
		}

		mockFlowData := &typesSvc.StatusFlowGraph{
			Nodes: []typesSvc.StatusFlowNode{
				{Name: "new"},
				{Name: "in_progress"},
			},
			Links: []typesSvc.StatusFlowLink{
				{Source: "new", Target: "in_progress", Value: 5},
			},
		}

		// Expect the filters to be passed to the service
		mockCaseSvc.EXPECT().GetStatusFlowData(mock.MatchedBy(func(f []queryutil.CrudFilter) bool {
			// Verify the filters contain our expected values
			hasGte := queryutil.DeepFindFilterWithOperator(f, "changed_at", queryutil.OpGte) != nil
			hasLte := queryutil.DeepFindFilterWithOperator(f, "changed_at", queryutil.OpLte) != nil
			hasFromStatus := queryutil.DeepFindFilterWithOperator(f, "from_status", queryutil.OpEq) != nil
			hasToStatus := queryutil.DeepFindFilterWithOperator(f, "to_status", queryutil.OpEq) != nil

			return hasGte && hasLte && hasFromStatus && hasToStatus
		})).Return(mockFlowData, nil).Once()

		// Generate query params using queryutil
		queryGen := generator.NewDefaultQueryParamGenerator()
		queryParams, err := queryGen.Generate(filters)
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/cases/status-flow", nil)
		req.URL.RawQuery = queryutil.ToQueryString(queryParams)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.StatusFlowGraphResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Len(tb, response.Nodes, 2)
		assert.Len(tb, response.Links, 1)
		assert.Equal(tb, "new", response.Links[0].Source)
		assert.Equal(tb, "in_progress", response.Links[0].Target)
		assert.Equal(tb, int64(5), response.Links[0].Value)
	})
}
