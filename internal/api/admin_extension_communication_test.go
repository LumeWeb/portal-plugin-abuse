package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreMocks "go.lumeweb.com/portal/core/testing/mocks"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupAdminCommunicationTest(t *testing.T) (*AdminExtension, *mocks.MockCommunicationService, *mocks.MockCaseService, *mux.Router) {
	ctx, adminExt, _ := setupAdminServices(t)
	mockCommSvc := core.GetService[typesSvc.CommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE).(*mocks.MockCommunicationService)
	mockCaseSvc := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE).(*mocks.MockCaseService)
	accessSvc := core.GetService[core.AccessService](ctx, core.ACCESS_SERVICE).(*coreMocks.MockAccessService)

	router := mux.NewRouter()
	abuseRouter := router.PathPrefix("/admin/abuse").Subrouter()
	err := adminExt.registerCommunicationHandlers(abuseRouter, accessSvc)
	if err != nil {
		t.Fatal(err)
	}

	return adminExt, mockCommSvc, mockCaseSvc, router
}

func TestListCaseCommunications_Success(t *testing.T) {
	_, mockCommSvc, _, router := setupAdminCommunicationTest(t)

	mockCommSvc.On("ListByCaseID", uint(1), mock.Anything, mock.Anything, mock.Anything).Return([]models.Communication{
		{Model: gorm.Model{ID: 1}, Content: "Test communication"},
	}, int64(1), nil)

	req := httptest.NewRequest("GET", "/admin/abuse/cases/1/communications", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Data []struct {
			ID      uint   `json:"id"`
			Content string `json:"content"`
		} `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Data, 1)
	assert.Equal(t, "Test communication", response.Data[0].Content)
}

func TestListCaseCommunications_CaseNotFound(t *testing.T) {
	_, mockCommSvc, _, router := setupAdminCommunicationTest(t)

	mockCommSvc.On("ListByCaseID", uint(999), mock.Anything, mock.Anything, mock.Anything).Return(nil, int64(0), gorm.ErrRecordNotFound)

	req := httptest.NewRequest("GET", "/admin/abuse/cases/999/communications", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAddCaseCommunication_Success(t *testing.T) {
	_, mockCommSvc, mockCaseSvc, router := setupAdminCommunicationTest(t)

	mockCaseSvc.On("GetByID", uint(1)).Return(&models.Case{Model: gorm.Model{ID: 1}}, nil)
	mockCommSvc.On("Create", mock.AnythingOfType("*models.Communication")).Return(&models.Communication{
		Model:   gorm.Model{ID: 1},
		Content: "New communication",
	}, nil)

	reqBody := map[string]interface{}{
		"content":   "New communication",
		"type":      "email",
		"direction": "incoming",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/abuse/cases/1/communications", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response struct {
		ID      uint   `json:"id"`
		Content string `json:"content"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "New communication", response.Content)
}

func TestAddCaseCommunication_ValidationFailure(t *testing.T) {
	_, _, _, router := setupAdminCommunicationTest(t)

	reqBody := `{"content": ""}` // Invalid empty content
	req := httptest.NewRequest("POST", "/admin/abuse/cases/1/communications", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestAddCaseCommunication_ServiceError(t *testing.T) {
	_, mockCommSvc, mockCaseSvc, router := setupAdminCommunicationTest(t)

	mockCaseSvc.On("GetByID", uint(1)).Return(&models.Case{Model: gorm.Model{ID: 1}}, nil)
	mockCommSvc.On("Create", mock.Anything).Return(nil, fmt.Errorf("service error"))

	reqBody := map[string]interface{}{
		"content":   "Valid content",
		"type":      "email",
		"direction": "incoming",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/abuse/cases/1/communications", bytes.NewReader(body))
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
