package api

import (
	"encoding/json"
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
	"gorm.io/gorm"
)

func TestCreateReporter_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockReporterSvc := core.GetService[*mocks.MockReporterService](ctx, typesSvc.REPORTER_SERVICE)

		mockReporter := &models.Reporter{
			Model: gorm.Model{ID: 1},
			Email: "test@example.com",
			Name:  "Test Reporter",
		}

		mockReporterSvc.EXPECT().Create(mock.AnythingOfType("*models.Reporter")).Return(mockReporter, nil).Once()

		reqBody := dto.ReporterCreateRequest{
			Email: "test@example.com",
			Name:  "Test Reporter",
		}
		body, err := json.Marshal(reqBody)
		assert.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/reporters", body)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusCreated, w.Code)

		var response dto.ReporterResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, "test@example.com", response.Email)
		assert.Equal(tb, "Test Reporter", response.Name)
	})
}

func TestCreateReporter_ValidationFailure(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		reqBody := `{"name": "Test Reporter"}`

		// Act
		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/reporters", []byte(reqBody))
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusUnprocessableEntity, w.Code)
	})
}

func TestGetReporter_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockReporterSvc := core.GetService[*mocks.MockReporterService](ctx, typesSvc.REPORTER_SERVICE)

		mockReporter := &models.Reporter{
			Model: gorm.Model{ID: 1},
			Email: "test@example.com",
			Name:  "Test Reporter",
		}

		mockReporterSvc.EXPECT().GetByID(uint(1)).Return(mockReporter, nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/reporters/1", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.ReporterResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, "test@example.com", response.Email)
		assert.Equal(tb, "Test Reporter", response.Name)
	})
}

func TestGetReporter_NotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockReporterSvc := core.GetService[*mocks.MockReporterService](ctx, typesSvc.REPORTER_SERVICE)

		mockReporterSvc.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/reporters/999", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusNotFound, w.Code)
	})
}

func TestListReporters_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockReporterSvc := core.GetService[*mocks.MockReporterService](ctx, typesSvc.REPORTER_SERVICE)

		mockReporters := []models.Reporter{
			{Model: gorm.Model{ID: 1}, Email: "test1@example.com"},
			{Model: gorm.Model{ID: 2}, Email: "test2@example.com"},
		}

		mockReporterSvc.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return(mockReporters, int64(2), nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/reporters", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response struct {
			Data []dto.ReporterResponse `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Len(tb, response.Data, 2)
		assert.Equal(tb, "test1@example.com", response.Data[0].Email)
	})
}
