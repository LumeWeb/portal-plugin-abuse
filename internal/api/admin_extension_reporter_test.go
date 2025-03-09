package api

import (
	"bytes"
	"encoding/json"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	"go.lumeweb.com/portal/core"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	coreMocks "go.lumeweb.com/portal/core/testing/mocks"
	"gorm.io/gorm"
)

func setupAdminReporterTest(t *testing.T) (*AdminExtension, *mocks.MockReporterService, *mux.Router) {
	ctx, adminExt, _ := setupAdminServices(t)
	mockReporterSvc := core.GetService[typesSvc.ReporterService](ctx, typesSvc.REPORTER_SERVICE).(*mocks.MockReporterService)
	accessSvc := core.GetService[core.AccessService](ctx, core.ACCESS_SERVICE).(*coreMocks.MockAccessService)

	// Set up router
	router := mux.NewRouter()
	abuseRouter := router.PathPrefix("/admin/abuse").Subrouter()
	err := adminExt.registerReporterHandlers(abuseRouter, accessSvc)
	if err != nil {
		t.Fatal(err)
	}

	return adminExt, mockReporterSvc, router
}

func TestCreateReporter_Success(t *testing.T) {
	_, mockReporterSvc, router := setupAdminReporterTest(t)

	// Mock expectations
	mockReporter := &models.Reporter{
		Model: gorm.Model{ID: 1},
		Email: "test@example.com",
		Name:  "Test Reporter",
	}
	mockReporterSvc.On("Create", mock.AnythingOfType("*models.Reporter")).Return(mockReporter, nil)

	// Create valid request
	reqBody := dto.ReporterCreateRequest{
		Email: "test@example.com",
		Name:  "Test Reporter",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/abuse/reporters", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response dto.ReporterResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test@example.com", response.Email)
	assert.Equal(t, "Test Reporter", response.Name)
}

func TestCreateReporter_ValidationFailure(t *testing.T) {
	_, _, router := setupAdminReporterTest(t)

	// Invalid request - missing required email
	reqBody := `{"name": "Test Reporter"}`
	req := httptest.NewRequest("POST", "/admin/abuse/reporters", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestGetReporter_Success(t *testing.T) {
	_, mockReporterSvc, router := setupAdminReporterTest(t)

	// Mock expectations
	mockReporter := &models.Reporter{
		Model: gorm.Model{ID: 1},
		Email: "test@example.com",
		Name:  "Test Reporter",
	}
	mockReporterSvc.On("GetByID", uint(1)).Return(mockReporter, nil)

	req := httptest.NewRequest("GET", "/admin/abuse/reporters/1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.ReporterResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test@example.com", response.Email)
	assert.Equal(t, "Test Reporter", response.Name)
}

func TestGetReporter_NotFound(t *testing.T) {
	_, mockReporterSvc, router := setupAdminReporterTest(t)

	mockReporterSvc.On("GetByID", uint(999)).Return(nil, gorm.ErrRecordNotFound)

	req := httptest.NewRequest("GET", "/admin/abuse/reporters/999", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListReporters_Success(t *testing.T) {
	_, mockReporterSvc, router := setupAdminReporterTest(t)

	mockReporters := []models.Reporter{
		{Model: gorm.Model{ID: 1}, Email: "test1@example.com"},
		{Model: gorm.Model{ID: 2}, Email: "test2@example.com"},
	}
	mockReporterSvc.On("List", mock.Anything, mock.Anything, mock.Anything).Return(mockReporters, int64(2), nil)

	req := httptest.NewRequest("GET", "/admin/abuse/reporters", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Data []dto.ReporterResponse `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Data, 2)
	assert.Equal(t, "test1@example.com", response.Data[0].Email)
}
