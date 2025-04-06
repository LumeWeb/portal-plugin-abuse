package api

import (
	"encoding/json"
	"fmt"
	"go.lumeweb.com/portal-plugin-abuse/internal/util"
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

func TestGetCommunicationTimeline(t *testing.T) {
	testCases := []struct {
		name           string
		timeRange      string
		mockData       []models.CommunicationHourlyCount
		mockError      error
		expectedStatus int
		expectedError  string
		validate       func(t *testing.T, response *httptest.ResponseRecorder)
		shouldMock     bool // New flag to control mocking
	}{
		{
			name:      "success with valid time range",
			timeRange: "24h",
			mockData: []models.CommunicationHourlyCount{
				{HourlyInterval: time.Now().Format("2006-01-02 15:00:00"), CommCount: 5},
				{HourlyInterval: time.Now().Add(-1 * time.Hour).Format("2006-01-02 15:00:00"), CommCount: 3},
			},
			expectedStatus: http.StatusOK,
			shouldMock:     true, // Explicitly enable mock for this case
			validate: func(t *testing.T, response *httptest.ResponseRecorder) {
				var result struct {
					Items []models.CommunicationHourlyCount `json:"items"`
				}
				err := json.Unmarshal(response.Body.Bytes(), &result)
				assert.NoError(t, err)
				assert.Len(t, result.Items, 2)
			},
		},
		{
			name:           "empty time range returns bad request",
			timeRange:      "",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid timeRange",
		},
		{
			name:           "invalid time range returns bad request",
			timeRange:      "invalid",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "invalid timeRange",
			shouldMock:     true,
			mockError:      util.ErrInvalidTimeRange,
		},
		{
			name:           "empty response returns success",
			timeRange:      "24h",
			mockData:       []models.CommunicationHourlyCount{},
			expectedStatus: http.StatusOK,
			shouldMock:     true,
			validate: func(t *testing.T, response *httptest.ResponseRecorder) {
				var result struct {
					Items []models.CommunicationHourlyCount `json:"items"`
				}
				err := json.Unmarshal(response.Body.Bytes(), &result)
				assert.NoError(t, err)
				assert.Empty(t, result.Items)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
				// Arrange
				if tc.shouldMock {
					mockCommSvc := core.GetService[*mocks.MockCommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
					mockCommSvc.EXPECT().
						GetCommunicationTimeline(tc.timeRange, mock.Anything).
						Return(tc.mockData, tc.mockError).Once()
				}

				// Generate query params
				var req *http.Request
				if tc.timeRange != "" {
					queryGen := generator.NewDefaultQueryParamGenerator()
					queryParams, err := queryGen.Generate([]queryutil.CrudFilter{
						queryutil.FieldEqual("time_range", tc.timeRange),
					})
					require.NoError(tb, err)

					req = ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/communications/timeline", nil)
					req.URL.RawQuery = queryutil.ToQueryString(queryParams)
				} else {
					req = ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/communications/timeline", nil)
				}

				w := httptest.NewRecorder()
				ctx.Router().ServeHTTP(w, req)

				// Assert status code
				assert.Equal(tb, tc.expectedStatus, w.Code)

				// Check for expected error message
				if tc.expectedError != "" {
					var response map[string]interface{}
					err := json.Unmarshal(w.Body.Bytes(), &response)
					assert.NoError(tb, err)
					assert.Contains(tb, response["error"], tc.expectedError)
				}

				// Run custom validation if provided
				if tc.validate != nil {
					tc.validate(t, w)
				}
			})
		})
	}
}

func TestGetCommunicationsTimeline_MissingTimeRange(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/communications/timeline", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, "invalid timeRange", response["error"])
	})
}

func TestGetCommunicationsTimeline_ServiceError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/communications/timeline?time_range=24h", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusBadRequest, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Contains(tb, response["error"], "invalid filters")
	})
}

func TestGetCommunicationsTimeline_InvalidTimeRange(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCommSvc := core.GetService[*mocks.MockCommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)

		mockCommSvc.EXPECT().GetCommunicationTimeline("invalid", nil).Return(nil, fmt.Errorf("service error")).Maybe()

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/communications/timeline?time_range=invalid", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusBadRequest, w.Code)
	})
}

func TestGetCommunicationsTimeline_EncodeError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/communications/timeline?time_range=24h", nil)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusBadRequest, w.Code)
	})
}

func TestGetCommunicationsTimeline_NoData(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCommSvc := core.GetService[*mocks.MockCommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
		mockCommSvc.EXPECT().GetCommunicationTimeline("24h", mock.Anything).Return([]models.CommunicationHourlyCount{}, nil).Once()

		// Generate query params with valid time range
		queryGen := generator.NewDefaultQueryParamGenerator()
		queryParams, err := queryGen.Generate([]queryutil.CrudFilter{
			queryutil.FieldEqual("time_range", "24h"),
		})
		require.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodGet, "/api/abuse/analytics/communications/timeline", nil)
		req.URL.RawQuery = queryutil.ToQueryString(queryParams)
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response struct {
			Items []struct {
				HourlyInterval string `json:"hourly_interval"`
				Count          int64  `json:"count"`
			} `json:"items"`
		}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Len(tb, response.Items, 0)
	})
}
