package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

func TestListCaseEvidence_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockEvidenceSvc := core.GetService[*mocks.MockEvidenceService](ctx, typesSvc.EVIDENCE_SERVICE)

		mockEvidenceSvc.EXPECT().GetByCaseID(uint(1), mock.AnythingOfType("filter.Pagination")).Return([]models.Evidence{
			{Model: gorm.Model{ID: 1}, FileName: "evidence1.txt"},
			{Model: gorm.Model{ID: 2}, FileName: "evidence2.txt"},
		}, int64(2), nil)

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/cases/1/evidence", nil)
		w := httptest.NewRecorder()

		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusOK, w.Code)

		var response struct {
			Data []dto.EvidenceResponse `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Len(tb, response.Data, 2)
		assert.Equal(tb, "evidence1.txt", response.Data[0].FileName)
	})
}

func TestListCaseEvidence_ServiceError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockEvidenceSvc := core.GetService[*mocks.MockEvidenceService](ctx, typesSvc.EVIDENCE_SERVICE)

		mockEvidenceSvc.EXPECT().GetByCaseID(uint(1), mock.AnythingOfType("filter.Pagination")).Return(nil, int64(0), fmt.Errorf("service error"))

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/cases/1/evidence", nil)
		w := httptest.NewRecorder()

		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusInternalServerError, w.Code)
	})
}

func TestGetEvidence_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockEvidenceSvc := core.GetService[*mocks.MockEvidenceService](ctx, typesSvc.EVIDENCE_SERVICE)

		mockEvidence := &models.Evidence{
			Model:    gorm.Model{ID: 1},
			FileName: "test.txt",
		}
		mockEvidenceSvc.EXPECT().GetByID(uint(1)).Return(mockEvidence, nil)

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/evidence/1", nil)
		w := httptest.NewRecorder()

		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.EvidenceResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, "test.txt", response.FileName)
	})
}

func TestGetEvidence_NotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockEvidenceSvc := core.GetService[*mocks.MockEvidenceService](ctx, typesSvc.EVIDENCE_SERVICE)

		mockEvidenceSvc.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound)

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/evidence/999", nil)
		w := httptest.NewRecorder()

		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusNotFound, w.Code)
	})
}

func TestUploadEvidence_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockEvidenceSvc := core.GetService[*mocks.MockEvidenceService](ctx, typesSvc.EVIDENCE_SERVICE)

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
		mockEvidenceSvc.EXPECT().CreateFromData(mock.AnythingOfType("multipart.sectionReadCloser"), mock.AnythingOfType("*models.Evidence")).Return(mockEvidence, nil)

		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/cases/1/evidence", body.Bytes())
		req.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()

		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusCreated, w.Code)

		var response dto.EvidenceResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, "test.txt", response.FileName)
		assert.Equal(tb, uint(1), response.CaseID)
	})
}

func TestGetEvidenceContent_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockEvidenceSvc := core.GetService[*mocks.MockEvidenceService](ctx, typesSvc.EVIDENCE_SERVICE)

		mockEvidence := &models.Evidence{
			Model:       gorm.Model{ID: 1},
			FileName:    "test.txt",
			ContentType: "text/plain",
		}
		mockEvidenceSvc.EXPECT().GetByID(uint(1)).Return(mockEvidence, nil)
		mockEvidenceSvc.EXPECT().GetContent(uint(1)).Return(io.NopCloser(bytes.NewReader([]byte("test content"))), "text/plain", nil)

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/evidence/1/content", nil)
		w := httptest.NewRecorder()

		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusOK, w.Code)
		assert.Equal(tb, "test content", w.Body.String())
		assert.Equal(tb, "text/plain", w.Header().Get("Content-Type"))
	})
}

func TestGetEvidenceContent_NotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockEvidenceSvc := core.GetService[*mocks.MockEvidenceService](ctx, typesSvc.EVIDENCE_SERVICE)

		mockEvidenceSvc.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound)

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/evidence/999/content", nil)
		w := httptest.NewRecorder()

		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusNotFound, w.Code)
	})
}

func TestGetEvidenceContent_RetrievalError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockEvidenceSvc := core.GetService[*mocks.MockEvidenceService](ctx, typesSvc.EVIDENCE_SERVICE)

		mockEvidence := &models.Evidence{Model: gorm.Model{ID: 1}}
		mockEvidenceSvc.EXPECT().GetByID(uint(1)).Return(mockEvidence, nil)
		mockEvidenceSvc.EXPECT().GetContent(uint(1)).Return(nil, "", fmt.Errorf("storage error"))

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/evidence/1/content", nil)
		w := httptest.NewRecorder()

		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusInternalServerError, w.Code)
	})
}
