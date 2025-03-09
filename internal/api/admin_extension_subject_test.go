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
)

func setupAdminSubjectTest(t *testing.T) (*AdminExtension, *mocks.MockSubjectService, *mux.Router) {
	ctx, adminExt, _ := setupAdminServices(t)
	mockSubjectSvc := core.GetService[typesSvc.SubjectService](ctx, typesSvc.SUBJECT_SERVICE).(*mocks.MockSubjectService)
	accessSvc := core.GetService[core.AccessService](ctx, core.ACCESS_SERVICE).(*coreMocks.MockAccessService)

	router := mux.NewRouter()
	abuseRouter := router.PathPrefix("/admin/abuse").Subrouter()
	err := adminExt.registerSubjectHandlers(abuseRouter, accessSvc)
	if err != nil {
		t.Fatal(err)
	}

	return adminExt, mockSubjectSvc, router
}

func TestCreateSubject_Success(t *testing.T) {
	_, mockSubjectSvc, router := setupAdminSubjectTest(t)

	hash, err := core.ParseStorageHash(exampleCID)
	if err != nil {
		t.Fatal(err)
	}

	mockSubject := &models.Subject{
		Model:      gorm.Model{ID: 1},
		Identifier: hash.Multihash(),
		Type:       models.SubjectTypeHash,
	}
	mockSubjectSvc.On("Create", mock.AnythingOfType("*models.Subject")).Return(mockSubject, nil)

	reqBody := dto.SubjectCreateRequest{
		Identifier: exampleCID,
		Type:       string(models.SubjectTypeHash),
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/abuse/subjects", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response dto.SubjectResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, exampleCID, response.Identifier)
}

func TestCreateSubject_ValidationFailure(t *testing.T) {
	_, _, router := setupAdminSubjectTest(t)

	reqBody := `{"type": "invalid"}`
	req := httptest.NewRequest("POST", "/admin/abuse/subjects", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestGetSubject_Success(t *testing.T) {
	_, mockSubjectSvc, router := setupAdminSubjectTest(t)

	hash, err := core.ParseStorageHash(exampleCID)
	if err != nil {
		t.Fatal(err)
	}

	mockSubject := &models.Subject{
		Model:      gorm.Model{ID: 1},
		Identifier: hash.Multihash(),
		Type:       models.SubjectTypeURL,
	}
	mockSubjectSvc.On("GetByID", uint(1)).Return(mockSubject, nil)

	req := httptest.NewRequest("GET", "/admin/abuse/subjects/1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.SubjectResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, exampleCID, response.Identifier)
}

func TestGetSubject_NotFound(t *testing.T) {
	_, mockSubjectSvc, router := setupAdminSubjectTest(t)

	mockSubjectSvc.On("GetByID", uint(999)).Return(nil, gorm.ErrRecordNotFound)

	req := httptest.NewRequest("GET", "/admin/abuse/subjects/999", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListSubjects_Success(t *testing.T) {
	_, mockSubjectSvc, router := setupAdminSubjectTest(t)

	hash, err := core.ParseStorageHash(exampleCID)
	if err != nil {
		t.Fatal(err)
	}

	mockSubjects := []models.Subject{
		{Model: gorm.Model{ID: 1}, Identifier: hash.Multihash()},
		{Model: gorm.Model{ID: 2}, Identifier: hash.Multihash()},
	}
	mockSubjectSvc.On("List", mock.Anything, mock.Anything, mock.Anything).Return(mockSubjects, int64(2), nil)

	req := httptest.NewRequest("GET", "/admin/abuse/subjects", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Data []dto.SubjectResponse `json:"data"`
	}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Data, 2)
	assert.Equal(t, exampleCID, response.Data[0].Identifier)
}

func TestListSubjects_ServiceError(t *testing.T) {
	_, mockSubjectSvc, router := setupAdminSubjectTest(t)

	mockSubjectSvc.On("List", mock.Anything, mock.Anything, mock.Anything).Return(nil, int64(0), fmt.Errorf("service error"))

	req := httptest.NewRequest("GET", "/admin/abuse/subjects", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
