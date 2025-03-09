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
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func setupAdminEvidenceTest(t *testing.T) (*AdminExtension, *mocks.MockEvidenceService, *mocks.MockCaseService, *mux.Router) {
	ctx, adminExt, _ := setupAdminServices(t)
	mockEvidenceSvc := core.GetService[typesSvc.EvidenceService](ctx, typesSvc.EVIDENCE_SERVICE).(*mocks.MockEvidenceService)
	mockCaseSvc := core.GetService[typesSvc.CaseService](ctx, typesSvc.CASE_SERVICE).(*mocks.MockCaseService)
	accessSvc := core.GetService[core.AccessService](ctx, core.ACCESS_SERVICE).(*coreMocks.MockAccessService)

	router := mux.NewRouter()
	abuseRouter := router.PathPrefix("/admin/abuse").Subrouter()
	err := adminExt.registerEvidenceHandlers(abuseRouter, accessSvc)
	if err != nil {
		t.Fatal(err)
	}

	return adminExt, mockEvidenceSvc, mockCaseSvc, router
}

func TestListCaseEvidence_Success(t *testing.T) {
	_, mockEvidenceSvc, _, router := setupAdminEvidenceTest(t)

	mockEvidenceSvc.On("GetByCaseID", uint(1), mock.Anything).Return([]models.Evidence{
		{Model: gorm.Model{ID: 1}, FileName: "evidence1.txt"},
		{Model: gorm.Model{ID: 2}, FileName: "evidence2.txt"},
	}, int64(2), nil)

	req := httptest.NewRequest("GET", "/admin/abuse/cases/1/evidence", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Data []dto.EvidenceResponse `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Data, 2)
	assert.Equal(t, "evidence1.txt", response.Data[0].FileName)
}

func TestListCaseEvidence_ServiceError(t *testing.T) {
	_, mockEvidenceSvc, _, router := setupAdminEvidenceTest(t)

	mockEvidenceSvc.On("GetByCaseID", uint(1), mock.Anything).Return(nil, int64(0), fmt.Errorf("service error"))

	req := httptest.NewRequest("GET", "/admin/abuse/cases/1/evidence", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetEvidence_Success(t *testing.T) {
	_, mockEvidenceSvc, _, router := setupAdminEvidenceTest(t)

	mockEvidence := &models.Evidence{
		Model:    gorm.Model{ID: 1},
		FileName: "test.txt",
	}
	mockEvidenceSvc.On("GetByID", uint(1)).Return(mockEvidence, nil)

	req := httptest.NewRequest("GET", "/admin/abuse/evidence/1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.EvidenceResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test.txt", response.FileName)
}

func TestGetEvidence_NotFound(t *testing.T) {
	_, mockEvidenceSvc, _, router := setupAdminEvidenceTest(t)

	mockEvidenceSvc.On("GetByID", uint(999)).Return(nil, gorm.ErrRecordNotFound)

	req := httptest.NewRequest("GET", "/admin/abuse/evidence/999", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUploadEvidence_Success(t *testing.T) {
	_, mockEvidenceSvc, _, router := setupAdminEvidenceTest(t)

	// Create multipart request body
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add JSON data part
	jsonData := `{
		"file_name": "test.txt",
		"content_type": "text/plain",
		"source": "web_upload",
		"description": "Test evidence",
		"metadata": {"key": "value"},
		"storage_hash": "QmHash123",
		"file_size": 1024
	}`
	dataPart, _ := writer.CreateFormField("data")
	_, err := dataPart.Write([]byte(jsonData))
	if err != nil {
		t.Fatal(err)
	}

	// Add file part
	filePart, _ := writer.CreateFormFile("file", "test.txt")
	_, err = filePart.Write([]byte("test file content"))
	if err != nil {
		t.Fatal(err)
	}
	err = writer.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Mock service call
	mockEvidence := &models.Evidence{
		Model:    gorm.Model{ID: 1},
		FileName: "test.txt",
		CaseID:   1,
	}
	// Match the actual types being passed - io.ReadCloser for file content and *models.Evidence
	mockEvidenceSvc.On("CreateFromData", mock.AnythingOfType("multipart.sectionReadCloser"), mock.AnythingOfType("*models.Evidence")).Return(mockEvidence, nil)

	req := httptest.NewRequest("POST", "/admin/abuse/cases/1/evidence", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response dto.EvidenceResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "test.txt", response.FileName)
	assert.Equal(t, uint(1), response.CaseID)
}

func TestGetEvidenceContent_Success(t *testing.T) {
	_, mockEvidenceSvc, _, router := setupAdminEvidenceTest(t)

	mockEvidence := &models.Evidence{
		Model:       gorm.Model{ID: 1},
		FileName:    "test.txt",
		ContentType: "text/plain",
	}
	mockEvidenceSvc.On("GetByID", uint(1)).Return(mockEvidence, nil)
	mockEvidenceSvc.On("GetContent", uint(1)).Return(io.NopCloser(bytes.NewReader([]byte("test content"))), "text/plain", nil)

	req := httptest.NewRequest("GET", "/admin/abuse/evidence/1/content", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test content", w.Body.String())
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
}

func TestGetEvidenceContent_NotFound(t *testing.T) {
	_, mockEvidenceSvc, _, router := setupAdminEvidenceTest(t)

	mockEvidenceSvc.On("GetByID", uint(999)).Return(nil, gorm.ErrRecordNotFound)

	req := httptest.NewRequest("GET", "/admin/abuse/evidence/999/content", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetEvidenceContent_RetrievalError(t *testing.T) {
	_, mockEvidenceSvc, _, router := setupAdminEvidenceTest(t)

	mockEvidence := &models.Evidence{Model: gorm.Model{ID: 1}}
	mockEvidenceSvc.On("GetByID", uint(1)).Return(mockEvidence, nil)
	mockEvidenceSvc.On("GetContent", uint(1)).Return(nil, "", fmt.Errorf("storage error"))

	req := httptest.NewRequest("GET", "/admin/abuse/evidence/1/content", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
