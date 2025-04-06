package api

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/queryutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"go.lumeweb.com/queryutil/filter/generator"
	"gorm.io/gorm"
)

func TestGetCommunicationTimeline_Success(t *testing.T) {
	testCases := []struct {
		name      string
		timeRange string
	}{
		{"24h", "24h"},
		{"7d", "7d"},
		{"30d", "30d"},
		{"90d", "90d"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
				// Arrange
				mockCommSvc := core.GetService[*mocks.MockCommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)
				mockTimelineData := []models.CommunicationHourlyCount{
					{HourlyInterval: time.Now().Format("2006-01-02 15:00:00"), CommCount: 5},
				}

				mockCommSvc.EXPECT().GetCommunicationTimeline(tc.timeRange, mock.Anything).Return(mockTimelineData, nil).Once()

				// Act
				queryGen := generator.NewDefaultQueryParamGenerator()
				queryParams, err := queryGen.Generate([]queryutil.CrudFilter{
					queryutil.FieldEqual("time_range", tc.timeRange),
				})
				require.NoError(tb, err)

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
				assert.Len(tb, response.Items, 1)
				assert.Equal(tb, int64(5), response.Items[0].Count)
			})
		})
	}
}

func TestGetCommunicationTimeline_EmptyTimeRange(t *testing.T) {
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

func TestAddCaseCommunication_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		mockCommSvc := core.GetService[*mocks.MockCommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)

		mockCaseSvc.EXPECT().GetByID(uint(1)).Return(&models.Case{Model: gorm.Model{ID: 1}}, nil).Once()
		mockCommSvc.EXPECT().Create(mock.AnythingOfType("*models.Communication")).Return(&models.Communication{
			Model:   gorm.Model{ID: 1},
			Content: "New communication",
		}, nil).Once()

		reqBody := map[string]interface{}{
			"content":   "New communication",
			"type":      "email",
			"direction": "incoming",
		}
		body, err := json.Marshal(reqBody)
		assert.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/cases/1/communications", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusCreated, w.Code)

		var response struct {
			ID      uint   `json:"id"`
			Content string `json:"content"`
		}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, "New communication", response.Content)
	})
}

func TestAddCaseCommunication_ValidationFailure(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		reqBody := `{"content": ""}` // Invalid empty content

		// Act
		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/cases/1/communications", []byte(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusUnprocessableEntity, w.Code)
	})
}

func TestAddCaseCommunication_ServiceError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, typesSvc.CASE_SERVICE)
		mockCommSvc := core.GetService[*mocks.MockCommunicationService](ctx, typesSvc.COMMUNICATION_SERVICE)

		mockCaseSvc.EXPECT().GetByID(uint(1)).Return(&models.Case{Model: gorm.Model{ID: 1}}, nil).Once()
		mockCommSvc.EXPECT().Create(mock.MatchedBy(func(comm *models.Communication) bool {
			return comm.Content == "Valid content" &&
				comm.Type == models.CommunicationTypeEmail &&
				comm.Direction == models.CommunicationDirectionIncoming
		})).Return(nil, fmt.Errorf("service error")).Once()

		reqBody := map[string]interface{}{
			"content":   "Valid content",
			"type":      "email",
			"direction": "incoming",
		}
		body, err := json.Marshal(reqBody)
		assert.NoError(tb, err)

		// Act
		req := ctx.NewAPIRequest(http.MethodPost, "/api/abuse/cases/1/communications", body)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusInternalServerError, w.Code)
	})
}
