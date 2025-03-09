package api

import (
	"bytes"
	"encoding/json"
	"github.com/samber/lo"
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

func setupAdminCaseTest(t *testing.T) (*AdminExtension, *mocks.MockCaseService, *mux.Router) {
	ctx, adminExt, _ := setupAdminServices(t)
	mockCaseSvc := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE).(*mocks.MockCaseService)
	accessSvc := core.GetService[core.AccessService](ctx, core.ACCESS_SERVICE).(*coreMocks.MockAccessService)

	// Set up router
	router := mux.NewRouter()
	abuseRouter := router.PathPrefix("/admin/abuse").Subrouter()
	err := adminExt.registerCaseHandlers(abuseRouter, accessSvc)
	if err != nil {
		t.Fatal(err)
	}

	return adminExt, mockCaseSvc, router
}

func TestCreateCase_Success(t *testing.T) {
	_, mockCaseSvc, router := setupAdminCaseTest(t)

	// Mock expectations
	mockCase := &models.Case{
		Model:           gorm.Model{ID: 1},
		ReferenceNumber: "CASE-1234",
		Type:            models.CaseTypeSpam,
		Description:     "Test spam case",
	}
	mockCaseSvc.On("Create", mock.AnythingOfType("*models.Case")).Return(mockCase, nil)

	// Create valid request
	reqBody := dto.CreateCaseRequest{
		Type: "spam",
		BaseRequest: dto.BaseRequest{
			Description: "Test spam case",
		},
		Priority:   "medium",
		Source:     "web_form",
		ReporterID: 1,
		SubjectID:  1,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/abuse/cases", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response dto.CaseResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "CASE-1234", response.ReferenceNumber)
	assert.Equal(t, "spam", response.Type)
}

func TestCreateCase_ValidationFailure(t *testing.T) {
	_, _, router := setupAdminCaseTest(t)

	// Invalid request - missing required fields
	reqBody := `{"type": "invalid"}`
	req := httptest.NewRequest("POST", "/admin/abuse/cases", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestGetCase_Success(t *testing.T) {
	_, mockCaseSvc, router := setupAdminCaseTest(t)

	// Mock expectations
	mockCase := &models.Case{
		Model:           gorm.Model{ID: 1},
		ReferenceNumber: "CASE-1234",
		Type:            models.CaseTypeHarassment,
		Priority:        models.CasePriorityMedium,
		Source:          models.ReportSourceWebForm,
	}
	mockCaseSvc.On("GetByID", uint(1)).Return(mockCase, nil)

	req := httptest.NewRequest("GET", "/admin/abuse/cases/1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.CaseResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "CASE-1234", response.ReferenceNumber)
	assert.Equal(t, "harassment", response.Type)
}

func TestGetCase_NotFound(t *testing.T) {
	_, mockCaseSvc, router := setupAdminCaseTest(t)

	mockCaseSvc.On("GetByID", uint(999)).Return(nil, gorm.ErrRecordNotFound)

	req := httptest.NewRequest("GET", "/admin/abuse/cases/999", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListCases_Success(t *testing.T) {
	_, mockCaseSvc, router := setupAdminCaseTest(t)

	mockCases := []models.Case{
		{Model: gorm.Model{ID: 1}, ReferenceNumber: "CASE-1"},
		{Model: gorm.Model{ID: 1}, ReferenceNumber: "CASE-2"},
	}
	mockCaseSvc.On("List", mock.Anything, mock.Anything, mock.Anything).Return(mockCases, int64(2), nil)

	req := httptest.NewRequest("GET", "/admin/abuse/cases", nil)
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

func TestUpdateCase_Success(t *testing.T) {
	_, mockCaseSvc, router := setupAdminCaseTest(t)

	existingCase := &models.Case{
		Model:       gorm.Model{ID: 1},
		Type:        models.CaseTypeSpam,
		Priority:    models.CasePriorityHigh,
		Description: "Original description",
	}
	updatedCase := *existingCase
	updatedCase.Description = "Updated description"

	mockCaseSvc.On("GetByID", uint(1)).Return(existingCase, nil)
	mockCaseSvc.On("Update", &updatedCase).Return(nil)

	reqBody := dto.UpdateCaseRequest{
		Description: lo.ToPtr("Updated description"),
		Type:        lo.ToPtr(string(models.CaseTypeSpam)),
		Priority:    lo.ToPtr(string(models.CasePriorityHigh)),
		ReporterID:  lo.ToPtr(1),
		SubjectID:   lo.ToPtr(1),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/admin/abuse/cases/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.CaseResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.EqualValues(t, models.CaseTypeSpam, response.Type)
	assert.EqualValues(t, models.CasePriorityHigh, response.Priority)
	assert.NoError(t, err)
}

func TestUpdateCase_NotFound(t *testing.T) {
	_, mockCaseSvc, router := setupAdminCaseTest(t)

	mockCaseSvc.On("GetByID", uint(999)).Return(nil, gorm.ErrRecordNotFound)

	reqBody := dto.UpdateCaseRequest{
		Description: lo.ToPtr("Updated"),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("PUT", "/admin/abuse/cases/999", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
