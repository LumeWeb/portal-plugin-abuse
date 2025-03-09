package api

import (
	"bytes"
	"encoding/json"
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
	"time"
)

func setupAdminScanTest(t *testing.T) (*AdminExtension, *mocks.MockScanService, *mocks.MockCaseService, *mux.Router) {
	ctx, adminExt, _ := setupAdminServices(t)
	mockScanSvc := core.GetService[typesSvc.ScanService](ctx, typesSvc.SCAN_SERVICE).(*mocks.MockScanService)
	mockCaseSvc := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE).(*mocks.MockCaseService)
	accessSvc := core.GetService[core.AccessService](ctx, core.ACCESS_SERVICE).(*coreMocks.MockAccessService)

	router := mux.NewRouter()
	abuseRouter := router.PathPrefix("/admin/abuse").Subrouter()
	err := adminExt.registerScanHandlers(abuseRouter, accessSvc)
	if err != nil {
		t.Fatal(err)
	}

	return adminExt, mockScanSvc, mockCaseSvc, router
}

func TestListCaseScans_Success(t *testing.T) {
	_, mockScanSvc, mockCaseSvc, router := setupAdminScanTest(t)

	mockCase := &models.Case{Model: gorm.Model{ID: 1}}
	mockCaseSvc.On("GetByID", uint(1)).Return(mockCase, nil)

	mockScans := []models.CaseScan{
		{Model: gorm.Model{ID: 1}, Status: models.ScanStatusClean},
		{Model: gorm.Model{ID: 2}, Status: models.ScanStatusPending},
	}
	mockScanSvc.On("GetScansForCase", uint(1), mock.Anything).Return(mockScans, int64(2), nil)

	req := httptest.NewRequest("GET", "/admin/abuse/cases/1/scans", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Data []struct {
			ID     uint   `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Data, 2)
	assert.EqualValues(t, models.ScanStatusClean, response.Data[0].Status)
}

func TestListCaseScans_CaseNotFound(t *testing.T) {
	_, _, mockCaseSvc, router := setupAdminScanTest(t)

	mockCaseSvc.On("GetByID", uint(999)).Return(nil, gorm.ErrRecordNotFound)

	req := httptest.NewRequest("GET", "/admin/abuse/cases/999/scans", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestCreateScanRequest_Success(t *testing.T) {
	_, mockScanSvc, mockCaseSvc, router := setupAdminScanTest(t)

	mockCase := &models.Case{Model: gorm.Model{ID: 1}}
	mockCaseSvc.On("GetByID", uint(1)).Return(mockCase, nil)
	mockScanSvc.On("CreateScanRequest", uint(1)).Return(nil)

	reqBody := map[string]interface{}{
		"case_id": 1,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/abuse/cases/1/scans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}

func TestCreateScanRequest_CaseNotFound(t *testing.T) {
	_, _, mockCaseSvc, router := setupAdminScanTest(t)

	mockCaseSvc.On("GetByID", uint(999)).Return(nil, gorm.ErrRecordNotFound)

	reqBody := map[string]interface{}{
		"case_id": 999,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/abuse/cases/999/scans", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetScan_Success(t *testing.T) {
	_, mockScanSvc, _, router := setupAdminScanTest(t)

	mockScan := &models.CaseScan{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: time.Now().Add(-1 * time.Hour),
			UpdatedAt: time.Now(),
		},
		CaseID: 1,
		Status: models.ScanStatusClean,
	}
	mockScanSvc.On("GetScanById", uint(1)).Return(mockScan, nil)

	req := httptest.NewRequest("GET", "/admin/abuse/cases/1/scans/1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		ID     uint   `json:"id"`
		Status string `json:"status"`
		CaseID uint   `json:"case_id"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, uint(1), response.ID)
	assert.EqualValues(t, models.ScanStatusClean, response.Status)
}

func TestGetScan_NotFound(t *testing.T) {
	_, mockScanSvc, _, router := setupAdminScanTest(t)

	mockScanSvc.On("GetScanById", uint(999)).Return(nil, gorm.ErrRecordNotFound)

	req := httptest.NewRequest("GET", "/admin/abuse/cases/1/scans/999", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetScanResults_Success(t *testing.T) {
	_, mockScanSvc, _, router := setupAdminScanTest(t)

	mockResults := []*core.ScanResult{
		{ScannerID: "av1", Passed: true, Reason: "clean"},
		{ScannerID: "malware", Passed: false, Reason: "detected"},
	}
	mockScanSvc.On("GetScanResults", uint(1)).Return(mockResults, nil)

	req := httptest.NewRequest("GET", "/admin/abuse/cases/1/scans/1/results", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Data []struct {
			ScannerID string `json:"scanner_id"`
			Passed    bool   `json:"passed"`
			Reason    string `json:"reason"`
		} `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Data, 2)
	assert.Equal(t, "av1", response.Data[0].ScannerID)
	assert.True(t, response.Data[0].Passed)
}

func TestGetScanResults_ScanNotFound(t *testing.T) {
	_, mockScanSvc, _, router := setupAdminScanTest(t)

	mockScanSvc.On("GetScanResults", uint(999)).Return(nil, gorm.ErrRecordNotFound)

	req := httptest.NewRequest("GET", "/admin/abuse/cases/1/scans/999/results", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
