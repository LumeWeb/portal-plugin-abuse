package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	"go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

const (
	exampleHashStr = "QmSnuWmxptJZdLJpKRarxBMS2Ju2oANVrgbr2xWbie9b2D"
)

func TestCreateBlock_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockBlocklistSvc := core.GetService[*mocks.MockBlockListService](ctx, service.BLOCKLIST_SERVICE)
		assert.NotNil(tb, mockBlocklistSvc)

		exampleHash, err := core.ParseStorageHash(exampleHashStr)
		if err != nil {
			tb.Fatalf("Failed to parse hash: %v", err)
		}

		mockBlock := &models.BlockList{
			Model:     gorm.Model{ID: 1},
			Hash:      exampleHash.Multihash(),
			Reason:    "malware",
			Severity:  "critical",
			Action:    "quarantine",
			BlockedBy: 1,
		}

		mockBlocklistSvc.EXPECT().BlockContent(mock.MatchedBy(func(block *models.BlockList) bool {
			return block.Reason == "malware" && block.Severity == "critical" && block.Action == "quarantine"
		})).Return(mockBlock, nil).Once()

		reqBody := dto.BlockContentCreateRequest{
			Hash:     exampleHashStr,
			Reason:   "malware",
			Severity: "critical",
			Action:   "quarantine",
		}
		body, err := json.Marshal(reqBody)
		assert.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/blocklist", body)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusCreated, w.Code)

		var response dto.BlockContentResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, models.BlockReasonMalware, response.Reason)
		assert.Equal(tb, models.BlockSeverityCritical, response.Severity)
	})
}

func TestCreateBlock_ValidationFailure(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		reqBody := `{"reason": "invalid"}`

		// Act
		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/blocklist", []byte(reqBody))
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusUnprocessableEntity, w.Code)
	})
}

func TestGetBlockedContent_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockBlocklistSvc := core.GetService[*mocks.MockBlockListService](ctx, service.BLOCKLIST_SERVICE)
		assert.NotNil(tb, mockBlocklistSvc)

		exampleHash, err := core.ParseStorageHash(exampleHashStr)
		if err != nil {
			tb.Fatalf("Failed to parse hash: %v", err)
		}

		mockBlock := &models.BlockList{
			Model:    gorm.Model{ID: 1},
			Hash:     exampleHash.Multihash(),
			Reason:   "copyright",
			Severity: "high",
		}

		mockBlocklistSvc.EXPECT().GetBlockedContent(exampleHash).Return(mockBlock, nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, fmt.Sprintf("/api/abuse/blocklist/%s", exampleHashStr), nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.BlockContentResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, models.BlockReasonCopyright, response.Reason)
		assert.Equal(tb, models.BlockSeverityHigh, response.Severity)
	})
}

func TestGetBlockedContent_NotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockBlocklistSvc := core.GetService[*mocks.MockBlockListService](ctx, service.BLOCKLIST_SERVICE)
		assert.NotNil(tb, mockBlocklistSvc)

		exampleHash, err := core.ParseStorageHash(exampleHashStr)
		if err != nil {
			tb.Fatalf("Failed to parse hash: %v", err)
		}

		mockBlocklistSvc.EXPECT().GetBlockedContent(exampleHash).Return(nil, gorm.ErrRecordNotFound).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, fmt.Sprintf("/api/abuse/blocklist/%s", exampleHashStr), nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusNotFound, w.Code)
	})
}

func TestListBlocks_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockBlocklistSvc := core.GetService[*mocks.MockBlockListService](ctx, service.BLOCKLIST_SERVICE)
		assert.NotNil(tb, mockBlocklistSvc)

		mockBlocks := []models.BlockList{
			{Model: gorm.Model{ID: 1}, Reason: "malware"},
			{Model: gorm.Model{ID: 2}, Reason: "copyright"},
		}

		mockBlocklistSvc.EXPECT().ListBlockedContent(mock.Anything, mock.Anything, mock.Anything).Return(mockBlocks, int64(2), nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/blocklist", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response struct {
			Data []dto.BlockContentResponse `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Len(tb, response.Data, 2)
		assert.Equal(tb, models.BlockReasonMalware, response.Data[0].Reason)
	})
}

func TestCheckSubjectBlocked_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockBlocklistSvc := core.GetService[*mocks.MockBlockListService](ctx, service.BLOCKLIST_SERVICE)
		assert.NotNil(tb, mockBlocklistSvc)

		mockBlocklistSvc.EXPECT().IsSubjectBlocked(uint(123)).Return(true, nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/blocklist/subjects/123/blocked", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.SubjectBlockedResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.True(tb, response.Blocked)
	})
}

func TestCheckSubjectBlocked_InvalidID(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/blocklist/subjects/invalid/blocked", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusBadRequest, w.Code)
	})
}

func TestCheckSubjectBlocked_ServiceError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockBlocklistSvc := core.GetService[*mocks.MockBlockListService](ctx, service.BLOCKLIST_SERVICE)
		assert.NotNil(tb, mockBlocklistSvc)

		mockBlocklistSvc.EXPECT().IsSubjectBlocked(uint(123)).Return(false, errors.New("service error")).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/blocklist/subjects/123/blocked", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusInternalServerError, w.Code)
	})
}

func TestUnblockContent_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockBlocklistSvc := core.GetService[*mocks.MockBlockListService](ctx, service.BLOCKLIST_SERVICE)
		assert.NotNil(tb, mockBlocklistSvc)

		exampleHash, err := core.ParseStorageHash(exampleHashStr)
		if err != nil {
			tb.Fatalf("Failed to parse hash: %v", err)
		}

		mockBlock := &models.BlockList{
			Model: gorm.Model{ID: 1},
			Hash:  exampleHash.Multihash(),
		}

		mockBlocklistSvc.EXPECT().GetBlockedContent(exampleHash).Return(mockBlock, nil).Once()
		mockBlocklistSvc.EXPECT().UnblockContent(mock.MatchedBy(func(hash *core.StorageHashDefault) bool {
			return hash.String() == exampleHashStr
		})).Return(nil).Once()

		// Act
		req := ctx.NewAPIRequest(http.MethodDelete, fmt.Sprintf("/api/abuse/blocklist/%s", exampleHashStr), nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)
	})
}
