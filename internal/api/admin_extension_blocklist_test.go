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

const (
	exampleHashStr = "QmSnuWmxptJZdLJpKRarxBMS2Ju2oANVrgbr2xWbie9b2D"
)

func setupAdminBlocklistTest(t *testing.T) (*AdminExtension, *mocks.MockBlockListService, *mux.Router) {
	ctx, adminExt, _ := setupAdminServices(t)
	mockBlocklistSvc := core.GetService[typesSvc.BlockListService](ctx, typesSvc.BLOCKLIST_SERVICE).(*mocks.MockBlockListService)
	accessSvc := core.GetService[core.AccessService](ctx, core.ACCESS_SERVICE).(*coreMocks.MockAccessService)

	// Set up router
	router := mux.NewRouter()
	abuseRouter := router.PathPrefix("/admin/abuse").Subrouter()
	err := adminExt.registerBlockListHandlers(abuseRouter, accessSvc)
	if err != nil {
		t.Fatal(err)
	}

	return adminExt, mockBlocklistSvc, router
}

func TestCreateBlock_Success(t *testing.T) {
	_, mockBlocklistSvc, router := setupAdminBlocklistTest(t)

	exampleHash, _ := core.ParseStorageHash(exampleHashStr)
	// Mock expectations
	mockBlock := &models.BlockList{
		Model:     gorm.Model{ID: 1},
		Hash:      exampleHash.Multihash(),
		Reason:    "malware",
		Severity:  "critical",
		Action:    "quarantine",
		BlockedBy: 1,
	}
	mockBlocklistSvc.On("BlockContent", mock.AnythingOfType("*models.BlockList")).Return(mockBlock, nil)

	// Create valid request
	reqBody := dto.BlockContentCreateRequest{
		Hash:     exampleHash.Multihash().String(),
		Reason:   "malware",
		Severity: "critical",
		Action:   "quarantine",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/admin/abuse/blocklist", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response dto.BlockContentResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "malware", response.Reason)
	assert.Equal(t, "critical", response.Severity)
}

func TestCreateBlock_ValidationFailure(t *testing.T) {
	_, _, router := setupAdminBlocklistTest(t)

	// Invalid request - missing required fields
	reqBody := `{"reason": "invalid"}`
	req := httptest.NewRequest("POST", "/admin/abuse/blocklist", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestGetBlockedContent_Success(t *testing.T) {
	_, mockBlocklistSvc, router := setupAdminBlocklistTest(t)

	// Mock expectations
	exampleHash, _ := core.ParseStorageHash(exampleHashStr)
	mockBlock := &models.BlockList{
		Model:    gorm.Model{ID: 1},
		Hash:     exampleHash.Multihash(),
		Reason:   "copyright",
		Severity: "high",
	}
	mockBlocklistSvc.On("GetBlockedContent", exampleHash).Return(mockBlock, nil)

	req := httptest.NewRequest("GET", fmt.Sprintf("/admin/abuse/blocklist/%s", exampleHash.Multihash().String()), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.BlockContentResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "copyright", response.Reason)
	assert.Equal(t, "high", response.Severity)
}

func TestGetBlockedContent_NotFound(t *testing.T) {
	_, mockBlocklistSvc, router := setupAdminBlocklistTest(t)

	exampleHash, _ := core.ParseStorageHash(exampleHashStr)
	mockBlocklistSvc.On("GetBlockedContent", exampleHash).Return(nil, gorm.ErrRecordNotFound)

	req := httptest.NewRequest("GET", fmt.Sprintf("/admin/abuse/blocklist/%s", exampleHash.Multihash().String()), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestListBlocks_Success(t *testing.T) {
	_, mockBlocklistSvc, router := setupAdminBlocklistTest(t)

	mockBlocks := []models.BlockList{
		{Model: gorm.Model{ID: 1}, Reason: "malware"},
		{Model: gorm.Model{ID: 2}, Reason: "copyright"},
	}
	mockBlocklistSvc.On("ListBlockedContent", mock.Anything, mock.Anything, mock.Anything).Return(mockBlocks, int64(2), nil)

	req := httptest.NewRequest("GET", "/admin/abuse/blocklist", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response struct {
		Data []dto.BlockContentResponse `json:"data"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response.Data, 2)
	assert.Equal(t, "malware", response.Data[0].Reason)
}

func TestUnblockContent_Success(t *testing.T) {
	_, mockBlocklistSvc, router := setupAdminBlocklistTest(t)

	// Mock expectations
	exampleHash, _ := core.ParseStorageHash(exampleHashStr)
	mockBlock := &models.BlockList{
		Model: gorm.Model{ID: 1},
		Hash:  exampleHash.Multihash(),
	}
	mockBlocklistSvc.On("GetBlockedContent", exampleHash).Return(mockBlock, nil)
	mockBlocklistSvc.On("UnblockContent", exampleHash).Return(nil)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/admin/abuse/blocklist/%s", exampleHash.Multihash().String()), nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
