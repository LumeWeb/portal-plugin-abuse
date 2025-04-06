package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

func TestUpdateCaseStatus_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		existingCase := &models.Case{
			Model:  gorm.Model{ID: 1},
			Status: models.CaseStatusNew,
		}
		updatedCase := &models.Case{
			Model:  gorm.Model{ID: 1},
			Status: models.CaseStatusInProgress,
		}

		mockCaseSvc.EXPECT().GetByID(uint(1)).Return(existingCase, nil).Once()
		mockCaseSvc.EXPECT().UpdateStatus(uint(1), models.CaseStatusInProgress).Return(nil).Once()
		mockCaseSvc.EXPECT().GetByID(uint(1)).Return(updatedCase, nil).Once()
		mockCaseSvc.EXPECT().SendStatusUpdateNotification(uint(1), models.CaseStatusNew, models.CaseStatusInProgress).Return(nil).Once()

		reqBody := dto.CaseStatusUpdateRequest{
			Status: string(models.CaseStatusInProgress),
		}
		body, err := json.Marshal(reqBody)
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodPut, "/api/abuse/cases/1/status", body)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.CaseStatusResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, string(models.CaseStatusNew), response.OldStatus)
		assert.Equal(tb, string(models.CaseStatusInProgress), response.NewStatus)

		time.Sleep(1500 * time.Millisecond)
	})
}

func TestUpdateCaseStatus_ValidationFailure(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		mockCaseSvc.EXPECT().GetByID(uint(1)).Return(&models.Case{
			Model:  gorm.Model{ID: 1},
			Status: models.CaseStatusNew,
		}, nil).Once()

		reqBody := `{"status": "invalid"}`

		// Act
		req := ctx.NewAPIRequest(http.MethodPut, "/api/abuse/cases/1/status", []byte(reqBody))
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusUnprocessableEntity, w.Code)
	})
}

func TestUpdateCaseStatus_NotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockCaseSvc.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound).Once()

		reqBody := dto.CaseStatusUpdateRequest{
			Status: string(models.CaseStatusInProgress),
		}
		body, err := json.Marshal(reqBody)
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodPut, "/api/abuse/cases/999/status", body)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusNotFound, w.Code)
	})
}

func TestUpdateCaseStatus_NotificationError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		existingCase := &models.Case{
			Model:  gorm.Model{ID: 1},
			Status: models.CaseStatusNew,
		}
		updatedCase := &models.Case{
			Model:  gorm.Model{ID: 1},
			Status: models.CaseStatusInProgress,
		}

		mockCaseSvc.EXPECT().GetByID(uint(1)).Return(existingCase, nil).Once()
		mockCaseSvc.EXPECT().UpdateStatus(uint(1), models.CaseStatusInProgress).Return(nil).Once()
		mockCaseSvc.EXPECT().GetByID(uint(1)).Return(updatedCase, nil).Once()
		mockCaseSvc.EXPECT().SendStatusUpdateNotification(uint(1), models.CaseStatusNew, models.CaseStatusInProgress).Return(fmt.Errorf("email error")).Once()

		reqBody := dto.CaseStatusUpdateRequest{
			Status: string(models.CaseStatusInProgress),
		}
		body, err := json.Marshal(reqBody)
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodPut, "/api/abuse/cases/1/status", body)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.CaseStatusResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, string(models.CaseStatusNew), response.OldStatus)
		assert.Equal(tb, string(models.CaseStatusInProgress), response.NewStatus)

		time.Sleep(1500 * time.Millisecond)
	})
}
