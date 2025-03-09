package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreMocks "go.lumeweb.com/portal/core/testing/mocks"
)

func setupAdminAnalyticsTest(t *testing.T) (*AdminExtension, *mocks.MockCaseService, *mux.Router) {
	ctx, adminExt, _ := setupAdminServices(t)
	mockCaseSvc := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE).(*mocks.MockCaseService)
	accessSvc := core.GetService[core.AccessService](ctx, core.ACCESS_SERVICE).(*coreMocks.MockAccessService)

	router := mux.NewRouter()
	abuseRouter := router.PathPrefix("/admin/abuse").Subrouter()
	err := adminExt.registerAnalyticsHandlers(abuseRouter, accessSvc)
	if err != nil {
		t.Fatal(err)
	}

	return adminExt, mockCaseSvc, router
}

func TestGetCaseAnalytics_Success(t *testing.T) {
	_, mockCaseSvc, router := setupAdminAnalyticsTest(t)

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

	mockCaseSvc.On("GetAnalytics", mock.Anything).Return(mockAnalytics, nil)

	req := httptest.NewRequest("GET", "/admin/abuse/analytics/cases", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.CaseAnalyticsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, int64(100), response.TotalCases)
	assert.Equal(t, int64(25), response.OpenCases)
	assert.Len(t, response.StatusDistribution, 2)
	assert.Len(t, response.CaseTypeDistribution, 2)
	assert.Equal(t, 86400.0, response.ResolutionMetrics.AverageSeconds)
}

func TestGetCaseAnalytics_ServiceError(t *testing.T) {
	_, mockCaseSvc, router := setupAdminAnalyticsTest(t)

	mockCaseSvc.On("GetAnalytics", mock.Anything).Return(nil, fmt.Errorf("analytics error"))

	req := httptest.NewRequest("GET", "/admin/abuse/analytics/cases", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGet7DayAnalytics_Success(t *testing.T) {
	_, mockCaseSvc, router := setupAdminAnalyticsTest(t)

	mockAnalytics := &typesSvc.CaseAnalytics{
		NewCasesInRange: 15,
		ResolutionTrends: map[time.Time]int64{
			time.Now().AddDate(0, 0, -1): 5,
			time.Now().AddDate(0, 0, -2): 10,
		},
	}

	mockCaseSvc.On("Get7DayAnalytics").Return(mockAnalytics, nil)

	req := httptest.NewRequest("GET", "/admin/abuse/analytics/cases/7d", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.CaseAnalyticsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, int64(15), response.NewCases)
	assert.Len(t, response.ResolutionMetrics.DailyTrends, 2)
}

func TestGet30DayAnalytics_Success(t *testing.T) {
	_, mockCaseSvc, router := setupAdminAnalyticsTest(t)

	mockAnalytics := &typesSvc.CaseAnalytics{
		NewCasesInRange: 50,
		ResolutionTrends: map[time.Time]int64{
			time.Now().AddDate(0, 0, -7):  20,
			time.Now().AddDate(0, 0, -14): 30,
		},
	}

	mockCaseSvc.On("Get30DayAnalytics").Return(mockAnalytics, nil)

	req := httptest.NewRequest("GET", "/admin/abuse/analytics/cases/30d", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.CaseAnalyticsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.Equal(t, int64(50), response.NewCases)
	assert.Len(t, response.ResolutionMetrics.DailyTrends, 2)
}
