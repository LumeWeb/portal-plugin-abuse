package api

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreMocks "go.lumeweb.com/portal/core/testing/mocks"
	"go.lumeweb.com/queryutil"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupAdminSearchTest(t *testing.T) (*AdminExtension, *mocks.MockSearchService, *mux.Router) {
	ctx, adminExt, _ := setupAdminServices(t)
	mockSearchSvc := core.GetService[typesSvc.SearchService](ctx, typesSvc.SEARCH_SERVICE).(*mocks.MockSearchService)
	accessSvc := core.GetService[core.AccessService](ctx, core.ACCESS_SERVICE).(*coreMocks.MockAccessService)

	router := mux.NewRouter()
	abuseRouter := router.PathPrefix("/admin/abuse").Subrouter()
	err := adminExt.registerSearchHandlers(abuseRouter, accessSvc)
	if err != nil {
		t.Fatal(err)
	}

	return adminExt, mockSearchSvc, router
}

func TestSearchCases_Success(t *testing.T) {
	_, mockSearchSvc, router := setupAdminSearchTest(t)

	mockCases := []models.Case{
		{Model: gorm.Model{ID: 1}, ReferenceNumber: "CASE-1"},
		{Model: gorm.Model{ID: 2}, ReferenceNumber: "CASE-2"},
	}
	mockSearchSvc.On("SearchCases", mock.Anything, "test", mock.Anything, mock.Anything).
		Return(mockCases, int64(2), nil)

	req := httptest.NewRequest("GET", "/admin/abuse/search/cases?q=test", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Data []dto.CaseResponse `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Data, 2)
	assert.Equal(t, "CASE-1", response.Data[0].ReferenceNumber)
}

func TestSearchCases_ServiceError(t *testing.T) {
	_, mockSearchSvc, router := setupAdminSearchTest(t)

	mockSearchSvc.On("SearchCases", mock.Anything, "error", mock.Anything, mock.Anything).
		Return(nil, int64(0), fmt.Errorf("search error"))

	req := httptest.NewRequest("GET", "/admin/abuse/search/cases?q=error", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGlobalSearch_Success(t *testing.T) {
	_, mockSearchSvc, router := setupAdminSearchTest(t)

	mockResult := &typesSvc.GlobalSearchResult{
		Cases: struct {
			Items []*models.Case
			Total int64
		}{
			Items: []*models.Case{{Model: gorm.Model{ID: 1}}},
			Total: 1,
		},
		Reporters: struct {
			Items []*models.Reporter
			Total int64
		}{
			Items: []*models.Reporter{{Model: gorm.Model{ID: 1}}},
			Total: 1,
		},
		Pagination: queryutil.Pagination{Start: 0, End: 10},
	}
	mockSearchSvc.On("GlobalSearch", mock.Anything, "global", mock.Anything).
		Return(mockResult, nil)

	req := httptest.NewRequest("GET", "/admin/abuse/search/global?q=global", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.GlobalSearchResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Cases, 1)
	assert.Len(t, response.Reporters, 1)
	assert.Equal(t, int64(2), response.TotalCount)
}

func TestGlobalSearch_ValidationError(t *testing.T) {
	_, _, router := setupAdminSearchTest(t)

	req := httptest.NewRequest("GET", "/admin/abuse/search/global?_start=invalid", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGlobalSearch_ServiceError(t *testing.T) {
	_, mockSearchSvc, router := setupAdminSearchTest(t)

	mockSearchSvc.On("GlobalSearch", mock.Anything, "error", mock.Anything).
		Return(nil, fmt.Errorf("global search error"))

	req := httptest.NewRequest("GET", "/admin/abuse/search/global?q=error", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
