package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"go.lumeweb.com/queryutil"
	"gorm.io/gorm"
)

func TestSearchCases_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockSearchSvc := core.GetService[*mocks.MockSearchService](ctx, typesSvc.SEARCH_SERVICE)

		mockCases := []models.Case{
			{Model: gorm.Model{ID: 1}, ReferenceNumber: "1"},
			{Model: gorm.Model{ID: 2}, ReferenceNumber: "2"},
		}
		mockSearchSvc.EXPECT().SearchCases(mock.Anything, "test", mock.Anything, mock.Anything).
			Return(mockCases, int64(2), nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/search/cases?q=test", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response struct {
			Data []dto.CaseResponse `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Len(tb, response.Data, 2)
		assert.Equal(tb, "CASE-1", response.Data[0].ReferenceNumber)
	})
}

func TestSearchCases_ServiceError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockSearchSvc := core.GetService[*mocks.MockSearchService](ctx, typesSvc.SEARCH_SERVICE)

		mockSearchSvc.EXPECT().SearchCases(mock.Anything, "error", mock.Anything, mock.Anything).
			Return(nil, int64(0), fmt.Errorf("search error")).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/search/cases?q=error", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusInternalServerError, w.Code)
	})
}

func TestGlobalSearch_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockSearchSvc := core.GetService[*mocks.MockSearchService](ctx, typesSvc.SEARCH_SERVICE)

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
		mockSearchSvc.EXPECT().GlobalSearch(mock.Anything, "global", mock.Anything).
			Return(mockResult, nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/search/global?q=global", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.GlobalSearchResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Len(tb, response.Cases, 1)
		assert.Len(tb, response.Reporters, 1)
		assert.Equal(tb, int64(2), response.TotalCount)
	})
}

func TestGlobalSearch_ValidationError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/search/global?_start=invalid", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusBadRequest, w.Code)
	})
}

func TestGlobalSearch_ServiceError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockSearchSvc := core.GetService[*mocks.MockSearchService](ctx, typesSvc.SEARCH_SERVICE)

		mockSearchSvc.EXPECT().GlobalSearch(mock.Anything, "error", mock.Anything).
			Return(nil, fmt.Errorf("global search error")).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/search/global?q=error", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusInternalServerError, w.Code)
	})
}
