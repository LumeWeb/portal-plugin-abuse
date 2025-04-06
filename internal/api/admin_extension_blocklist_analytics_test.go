package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"go.lumeweb.com/queryutil"
	"go.lumeweb.com/queryutil/filter/generator"
)

func TestGetBlockReasons_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockBlocklistSvc := core.GetService[*mocks.MockBlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, mockBlocklistSvc)

		now := time.Now()
		dateStr := now.Format("2006-01-02")

		mockData := []typesSvc.BlockReasonCount{
			{BlockDate: dateStr, BlockReason: string(models.BlockReasonSpam), BlockCount: 5},
			{BlockDate: dateStr, BlockReason: string(models.BlockReasonMalware), BlockCount: 3},
		}

		mockBlocklistSvc.EXPECT().GetBlockReasonCounts(mock.Anything).Return(mockData, nil).Once()

		// Generate query params with valid time range
		queryGen := generator.NewDefaultQueryParamGenerator()
		queryParams, err := queryGen.Generate([]queryutil.CrudFilter{
			queryutil.FieldEqual("time_range", "7d"),
		})
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/blocklist/block-reasons", nil)
		req.URL.RawQuery = queryutil.ToQueryString(queryParams)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response struct {
			Items []struct {
				BlockDate   string `json:"block_date"`
				Reason      string `json:"reason"`
				BlockCount  int64  `json:"count"`
			} `json:"items"`
		}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Len(tb, response.Items, 2)
		assert.Equal(tb, "spam", response.Items[0].Reason)
		assert.Equal(tb, int64(5), response.Items[0].BlockCount)
	})
}

func TestGetBlockReasons_ServiceError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockBlocklistSvc := core.GetService[*mocks.MockBlockListService](ctx, typesSvc.BLOCKLIST_SERVICE)
		assert.NotNil(tb, mockBlocklistSvc)

		mockBlocklistSvc.EXPECT().GetBlockReasonCounts(mock.Anything).Return(nil, fmt.Errorf("service error")).Once()

		// Generate query params with valid time range
		queryGen := generator.NewDefaultQueryParamGenerator()
		queryParams, err := queryGen.Generate([]queryutil.CrudFilter{
			queryutil.FieldEqual("time_range", "7d"),
		})
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/blocklist/block-reasons", nil)
		req.URL.RawQuery = queryutil.ToQueryString(queryParams)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusInternalServerError, w.Code)
	})
}

func TestGetBlockReasons_InvalidTimeRange(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/blocklist/block-reasons?time_range=invalid", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusBadRequest, w.Code)
	})
}
