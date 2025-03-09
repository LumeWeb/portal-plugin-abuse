package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func setupAbuseAPITest(t *testing.T) (*AbuseAPI, *mocks.MockAbuseReportService, *mocks.MockEmailService, *mux.Router) {
	// Create test context using core testing framework
	ctx := coreTesting.NewTestContext(t)

	// Initialize mock services
	mockReportSvc := mocks.NewMockAbuseReportService(t)
	mockEmailSvc := mocks.NewMockEmailService(t)

	// Register services in context
	ctx.RegisterService(service.ABUSE_REPORT_SERVICE, mockReportSvc)
	ctx.RegisterService(service.EMAIL_SERVICE, mockEmailSvc)

	// Create API instance using context
	api := &AbuseAPI{
		ctx:                ctx,
		logger:             ctx.NamedLogger("test"),
		abuseReportService: mockReportSvc,
		emailService:       mockEmailSvc,
	}

	// Set up router
	router := mux.NewRouter()
	err := api.Configure(router, nil)
	if err != nil {
		t.Fatal(err)
		return nil, nil, nil, nil
	}

	return api, mockReportSvc, mockEmailSvc, router
}

func TestSubmitReport_Success(t *testing.T) {
	_, mockReportSvc, _, router := setupAbuseAPITest(t)

	// Mock expectations
	mockCase := &models.Case{
		ReferenceNumber: "AR-1234ABCD",
		Reporter: models.Reporter{
			Email: "user@example.com",
		},
		Description: "Test description of abusive content",
	}
	mockReportSvc.On("SubmitReport", mock.Anything, mock.MatchedBy(func(c *models.Case) bool {
		return c.Reporter.Email == "user@example.com" &&
			c.Description == "Test description of abusive content"
	})).
		Return(mockCase, nil)

	// Create valid request
	reqBody := dto.AbuseReportRequest{
		Email:       "user@example.com",
		Location:    "https://example.com/abuse-content",
		AbuseType:   "malicious_content",
		Description: "Test description of abusive content",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/abuse/report", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	router.ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusCreated, w.Code)

	var response dto.AbuseReportResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "AR-1234ABCD", response.ConfirmationNumber)
}

func TestSubmitReport_ValidationFailure(t *testing.T) {
	_, _, _, router := setupAbuseAPITest(t)

	// Invalid request - missing required fields
	reqBody := `{"email": "invalid-email", "location": "not-a-url"}`
	req := httptest.NewRequest("POST", "/api/abuse/report", bytes.NewReader([]byte(reqBody)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute
	router.ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestGetReportStatus_Success(t *testing.T) {
	_, mockReportSvc, _, router := setupAbuseAPITest(t)

	// Mock expectations
	mockCase := &models.Case{
		ReferenceNumber: "AR-1234ABCD",
	}
	mockReportSvc.On("GetReportStatus", mock.Anything, "AR-1234ABCD").
		Return(mockCase, nil)

	// Create request
	req := httptest.NewRequest("GET", "/api/abuse/report/AR-1234ABCD", nil)
	w := httptest.NewRecorder()

	// Execute
	router.ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusOK, w.Code)

	var response dto.AbuseReportResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "AR-1234ABCD", response.ConfirmationNumber)
}

func TestGetReportStatus_InvalidFormat(t *testing.T) {
	_, _, _, router := setupAbuseAPITest(t)

	// Test invalid confirmation number format
	req := httptest.NewRequest("GET", "/api/abuse/report/invalid_format", nil)
	w := httptest.NewRecorder()

	// Execute
	router.ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetReportStatus_NotFound(t *testing.T) {
	_, mockReportSvc, _, router := setupAbuseAPITest(t)

	// Mock expectations
	mockReportSvc.On("GetReportStatus", mock.Anything, "AR-NOTFOUND").
		Return(nil, fmt.Errorf("not found"))

	// Create request
	req := httptest.NewRequest("GET", "/api/abuse/report/AR-NOTFOUND", nil)
	w := httptest.NewRecorder()

	// Execute
	router.ServeHTTP(w, req)

	// Verify
	assert.Equal(t, http.StatusNotFound, w.Code)
}
