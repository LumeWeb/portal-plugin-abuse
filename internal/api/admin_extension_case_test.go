package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

func TestCreateCase_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockCase := &models.Case{
			Model:           gorm.Model{ID: 1},
			ReferenceNumber: "1234",
			Type:            models.CaseTypeSpam,
			Description:     "Test spam case",
		}

		mockCaseSvc.EXPECT().Create(mock.AnythingOfType("*models.Case")).Return(mockCase, nil).Once()

		reqBody := dto.CreateCaseRequest{
			Type:        "spam",
			Description: "Test spam case",
			Priority:    "medium",
			Source:      "web_form",
			ReporterID:  1,
			SubjectID:   1,
		}
		body, err := json.Marshal(reqBody)
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/cases", body)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusCreated, w.Code)

		var response dto.CaseResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, "CASE-1234", response.ReferenceNumber)
		assert.Equal(tb, "spam", response.Type)
	})
}

func TestCreateCase_ValidationFailure(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		reqBody := `{"type": "invalid"}`

		// Act
		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/cases", []byte(reqBody))
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusUnprocessableEntity, w.Code)
	})
}

func TestAdminGetCase_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockCase := &models.Case{
			Model:           gorm.Model{ID: 1},
			ReferenceNumber: "1234",
			Type:            models.CaseTypeHarassment,
			Priority:        models.CasePriorityMedium,
			Source:          models.ReportSourceWebForm,
		}

		mockCaseSvc.EXPECT().GetByID(uint(1)).Return(mockCase, nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/cases/1", nil)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.CaseResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, "CASE-1234", response.ReferenceNumber)
		assert.Equal(tb, "harassment", response.Type)
	})
}

func TestGetCase_NotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockCaseSvc.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/cases/999", nil)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusNotFound, w.Code)
	})
}

func TestListCases_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockCases := []models.Case{
			{Model: gorm.Model{ID: 1}, ReferenceNumber: "1"},
			{Model: gorm.Model{ID: 2}, ReferenceNumber: "2"},
		}

		mockCaseSvc.EXPECT().List(mock.Anything, mock.Anything, mock.Anything).Return(mockCases, int64(2), nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/cases", nil)
		require.NotNil(tb, req)
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

func TestUpdateCase_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		existingCase := &models.Case{
			Model:       gorm.Model{ID: 1},
			Type:        models.CaseTypeSpam,
			Priority:    models.CasePriorityHigh,
			Description: "Original description",
			ReporterID:  1,
			SubjectID:   1,
		}

		mockCaseSvc.EXPECT().GetByID(uint(1)).Return(existingCase, nil).Once()
		mockCaseSvc.EXPECT().Update(mock.MatchedBy(func(c *models.Case) bool {
			return c.ReporterID == 1 &&
				c.SubjectID == 1 &&
				c.Description == "Updated description" &&
				c.Type == models.CaseTypeSpam &&
				c.Priority == models.CasePriorityHigh
		})).Return(nil).Once()

		reqBody := dto.UpdateCaseRequest{
			Description: lo.ToPtr("Updated description"),
			Type:        lo.ToPtr(string(models.CaseTypeSpam)),
			Priority:    lo.ToPtr(string(models.CasePriorityHigh)),
			ReporterID:  lo.ToPtr(1),
			SubjectID:   lo.ToPtr(1),
		}
		body, err := json.Marshal(reqBody)
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodPatch, "/api/abuse/cases/1", body)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)
	})
}

func TestUpdateCase_NotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockCaseSvc.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound).Once()

		reqBody := dto.UpdateCaseRequest{
			Description: lo.ToPtr("Updated"),
		}
		body, err := json.Marshal(reqBody)
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodPatch, "/api/abuse/cases/999", body)
		require.NotNil(tb, req)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusNotFound, w.Code)
	})
}
