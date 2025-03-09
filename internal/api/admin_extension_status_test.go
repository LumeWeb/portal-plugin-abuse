package api

import (
	"bytes"
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
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func setupAdminStatusTest(t *testing.T) (*AdminExtension, *mocks.MockCaseService, *mux.Router) {
	ctx, adminExt, _ := setupAdminServices(t)
	mockCaseSvc := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE).(*mocks.MockCaseService)
	accessSvc := core.GetService[core.AccessService](ctx, core.ACCESS_SERVICE).(*coreMocks.MockAccessService)

	router := mux.NewRouter()
	abuseRouter := router.PathPrefix("/admin/abuse").Subrouter()
	err := adminExt.registerStatusUpdateHandlers(abuseRouter, accessSvc)
	if err != nil {
		t.Fatal(err)
	}

	return adminExt, mockCaseSvc, router
}

func TestUpdateCaseStatus_Success(t *testing.T) {
	_, mockCaseSvc, router := setupAdminStatusTest(t)

	// Mock expectations
	existingCase := &models.Case{
		Model:  gorm.Model{ID: 1},
		Status: models.CaseStatusNew,
	}
	updatedCase := &models.Case{
		Model:  gorm.Model{ID: 1},
		Status: models.CaseStatusInProgress,
	}
	// Initial GetByID
	mockCaseSvc.On("GetByID", uint(1)).Return(existingCase, nil).Once()

	// UpdateStatus
	mockCaseSvc.On("UpdateStatus", uint(1), models.CaseStatusInProgress).Return(nil).
		Run(func(args mock.Arguments) {
			// Update the *next* call
			mockCaseSvc.ExpectedCalls[2].ReturnArguments = mock.Arguments{updatedCase, nil}

		}).Once() // ONLY run this ONCE, after call2.

	mockCaseSvc.On("GetByID", uint(1)).Return(&updatedCase, nil).Once()

	// SendStatusUpdateNotification
	mockCaseSvc.On("SendStatusUpdateNotification", uint(1), models.CaseStatusNew, models.CaseStatusInProgress).Return(nil).Once()

	reqBody := dto.CaseStatusUpdateRequest{
		Status: string(models.CaseStatusInProgress),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/admin/abuse/cases/1/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.CaseStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, string(models.CaseStatusNew), response.OldStatus)
	assert.Equal(t, string(models.CaseStatusInProgress), response.NewStatus)

	time.Sleep(1500 * time.Millisecond)
}

func TestUpdateCaseStatus_ValidationFailure(t *testing.T) {
	_, mockCaseSvc, router := setupAdminStatusTest(t)

	existingCase := &models.Case{Model: gorm.Model{ID: 1}}
	mockCaseSvc.On("GetByID", uint(1)).Return(existingCase, nil).Once()
	// Invalid request - invalid status
	reqBody := `{"status": "invalid"}`
	req := httptest.NewRequest("PUT", "/admin/abuse/cases/1/status", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestUpdateCaseStatus_NotFound(t *testing.T) {
	_, mockCaseSvc, router := setupAdminStatusTest(t)

	mockCaseSvc.On("GetByID", uint(999)).Return(nil, gorm.ErrRecordNotFound)

	reqBody := dto.CaseStatusUpdateRequest{
		Status: string(models.CaseStatusInProgress),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/admin/abuse/cases/999/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUpdateCaseStatus_ServiceError(t *testing.T) {
	_, mockCaseSvc, router := setupAdminStatusTest(t)

	existingCase := &models.Case{Model: gorm.Model{ID: 1}}
	mockCaseSvc.On("GetByID", uint(1)).Return(existingCase, nil)
	mockCaseSvc.On("UpdateStatus", uint(1), models.CaseStatusInProgress).Return(fmt.Errorf("service error"))

	reqBody := dto.CaseStatusUpdateRequest{
		Status: string(models.CaseStatusInProgress),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/admin/abuse/cases/1/status", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestUpdateCaseStatus_NotificationError(t *testing.T) {
	_, mockCaseSvc, router := setupAdminStatusTest(t)

	existingCase := &models.Case{
		Model:  gorm.Model{ID: 1},
		Status: models.CaseStatusNew,
	}
	updatedCase := *existingCase
	updatedCase.Status = models.CaseStatusInProgress
	// Initial GetByID
	mockCaseSvc.On("GetByID", uint(1)).Return(existingCase, nil).Once()

	// UpdateStatus
	mockCaseSvc.On("UpdateStatus", uint(1), models.CaseStatusInProgress).Return(nil).
		Run(func(args mock.Arguments) {
			// Access and update the second GetByID with updatedCase

			mockCaseSvc.ExpectedCalls[2].ReturnArguments = mock.Arguments{&updatedCase, nil}

		}).Once() // ONLY run this ONCE, after call2.

	mockCaseSvc.On("GetByID", uint(1)).Return(&updatedCase, nil).Once()
	// SendStatusUpdateNotification
	mockCaseSvc.On("SendStatusUpdateNotification", uint(1), models.CaseStatusNew, models.CaseStatusInProgress).Return(fmt.Errorf("email error")).Once()

	reqBody := dto.CaseStatusUpdateRequest{
		Status: string(models.CaseStatusInProgress),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/admin/abuse/cases/1/status", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Notification failure shouldn't affect the response status
	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.CaseStatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, string(models.CaseStatusInProgress), response.NewStatus)

	time.Sleep(1500 * time.Millisecond)
}
