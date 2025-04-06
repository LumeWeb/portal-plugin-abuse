package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

func TestListCaseScans_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		mockScanSvc := core.GetService[*mocks.MockScanService](ctx, typesSvc.SCAN_SERVICE)

		mockCaseSvc.EXPECT().GetByID(uint(1)).Return(&models.Case{Model: gorm.Model{ID: 1}}, nil).Once()
		mockScanSvc.EXPECT().GetScansForCase(uint(1), mock.AnythingOfType("filter.Pagination")).Return([]models.CaseScan{
			{Model: gorm.Model{ID: 1}, Status: models.ScanStatusClean},
			{Model: gorm.Model{ID: 2}, Status: models.ScanStatusPending},
		}, int64(2), nil).Once()

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/cases/1/scans", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusOK, w.Code)

		var response struct {
			Data []struct {
				ID     uint   `json:"id"`
				Status string `json:"status"`
			} `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Len(tb, response.Data, 2)
		assert.EqualValues(tb, models.ScanStatusClean, response.Data[0].Status)
	})
}

func TestListCaseScans_CaseNotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockCaseSvc.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound).Once()

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/cases/999/scans", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusNotFound, w.Code)
	})
}

func TestCreateScanRequest_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		mockScanSvc := core.GetService[*mocks.MockScanService](ctx, typesSvc.SCAN_SERVICE)

		mockCaseSvc.EXPECT().GetByID(uint(1)).Return(&models.Case{Model: gorm.Model{ID: 1}}, nil).Once()
		mockScanSvc.EXPECT().CreateScanRequest(uint(1)).Return(nil).Once()

		reqBody := map[string]interface{}{
			"case_id": 1,
		}
		body, err := json.Marshal(reqBody)
		assert.NoError(tb, err)

		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/cases/1/scans", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusAccepted, w.Code)
	})
}

func TestCreateScanRequest_CaseNotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)

		mockCaseSvc.EXPECT().GetByID(uint(999)).Return(nil, gorm.ErrRecordNotFound).Once()

		reqBody := map[string]interface{}{
			"case_id": 999,
		}
		body, err := json.Marshal(reqBody)
		assert.NoError(tb, err)

		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/cases/999/scans", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusNotFound, w.Code)
	})
}

func TestGetScan_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockScanSvc := core.GetService[*mocks.MockScanService](ctx, typesSvc.SCAN_SERVICE)

		mockScan := &models.CaseScan{
			Model: gorm.Model{
				ID: 1,
			},
			CaseID: 1,
			Status: models.ScanStatusClean,
		}

		mockScanSvc.EXPECT().GetScanById(uint(1)).Return(mockScan, nil).Once()

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/cases/1/scans/1", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusOK, w.Code)

		var response struct {
			ID     uint   `json:"id"`
			Status string `json:"status"`
			CaseID uint   `json:"case_id"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, uint(1), response.ID)
		assert.EqualValues(tb, models.ScanStatusClean, response.Status)
		assert.Equal(tb, uint(1), response.CaseID)
	})
}

func TestGetScan_NotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockScanSvc := core.GetService[*mocks.MockScanService](ctx, typesSvc.SCAN_SERVICE)

		mockScanSvc.EXPECT().GetScanById(uint(999)).Return(nil, db.ErrRecordNotFound).Once()

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/cases/1/scans/999", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusNotFound, w.Code)
	})
}

func TestGetScanResults_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockScanSvc := core.GetService[*mocks.MockScanService](ctx, typesSvc.SCAN_SERVICE)

		mockResults := []*core.ScanResult{
			{ScannerID: "av1", Passed: true, Reason: "clean"},
			{ScannerID: "malware", Passed: false, Reason: "detected"},
		}

		mockScanSvc.EXPECT().GetScanResults(uint(1), mock.AnythingOfType("[]filter.CrudFilter"), mock.AnythingOfType("[]filter.Sort"), mock.AnythingOfType("filter.Pagination")).Return(mockResults, int64(2), nil).Once()

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/cases/1/scans/1/results", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusOK, w.Code)

		var response struct {
			Data []struct {
				ScannerID string `json:"scanner_id"`
				Passed    bool   `json:"passed"`
				Reason    string `json:"reason"`
			} `json:"data"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Len(tb, response.Data, 2)
		assert.Equal(tb, "av1", response.Data[0].ScannerID)
		assert.True(tb, response.Data[0].Passed)
	})
}

func TestGetScanResults_ScanNotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockScanSvc := core.GetService[*mocks.MockScanService](ctx, typesSvc.SCAN_SERVICE)

		mockScanSvc.EXPECT().GetScanResults(uint(999), mock.AnythingOfType("[]filter.CrudFilter"), mock.AnythingOfType("[]filter.Sort"), mock.AnythingOfType("filter.Pagination")).Return(nil, int64(0), db.ErrRecordNotFound).Once()

		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/cases/1/scans/999/results", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		assert.Equal(tb, http.StatusNotFound, w.Code)
	})
}
