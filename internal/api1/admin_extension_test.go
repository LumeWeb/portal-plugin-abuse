package api1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go.lumeweb.com/portal-plugin-abuse/internal/service"
	"gorm.io/gorm"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"go.lumeweb.com/queryutil"
)

// Mock implementations of all required services

// Ensure mock implementations satisfy their interfaces
var _ typesSvc.CaseService = (*MockCaseService)(nil)
var _ typesSvc.ReporterService = (*MockReporterService)(nil)
var _ typesSvc.SubjectService = (*MockSubjectService)(nil)
var _ typesSvc.CommunicationService = (*MockCommunicationService)(nil)
var _ typesSvc.ScanService = (*MockScanService)(nil)
var _ typesSvc.SearchService = (*MockSearchService)(nil)
var _ typesSvc.EmailService = (*MockEmailService)(nil)
var _ typesSvc.TokenService = (*MockTokenService)(nil)
var _ core.AccessService = (*MockAccessService)(nil)

// MockCaseService mocks the CaseService interface
type MockCaseService struct {
	mock.Mock
}

// TestConcurrentRequests tests handling of concurrent requests
func TestConcurrentRequests(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	defer func() {
		if testCtx, ok := ctx.(coreTesting.TestContext); ok {
			testCtx.Teardown()
		}
	}()
	mockCaseService := &MockCaseService{}
	mockReporterService := &MockReporterService{}
	mockSubjectService := &MockSubjectService{}

	// Create test data
	now := time.Now()
	testCase := &models.Case{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}

	testReporter := &models.Reporter{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Email: "test@example.com",
		Name:  "Test Reporter",
	}

	testSubject := &models.Subject{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Identifier: "test-identifier",
		Type:       models.SubjectTypeHash,
	}

	// Setup mock expectations
	mockCaseService.On("GetByID", uint(1)).Return(testCase, nil).Maybe()
	mockReporterService.On("GetByID", uint(1)).Return(testReporter, nil).Maybe()
	mockSubjectService.On("GetByID", uint(1)).Return(testSubject, nil).Maybe()

	// For list operations, return a list of cases
	mockCaseService.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.Case{*testCase}, int64(1), nil).Maybe()

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE:     mockCaseService,
		typesSvc.REPORTER_SERVICE: mockReporterService,
		typesSvc.SUBJECT_SERVICE:  mockSubjectService,
	})

	// Number of concurrent requests
	concurrentRequests := 10

	// Create a wait group to synchronize goroutines
	var wg sync.WaitGroup
	wg.Add(concurrentRequests)

	// Create a channel to collect errors
	errChan := make(chan error, concurrentRequests)

	// Launch concurrent requests to the same endpoint
	for i := 0; i < concurrentRequests; i++ {
		go func() {
			defer wg.Done()

			// Create request to list cases
			req := createTestRequest(t, "GET", "/abuse/cases?_start=0&_end=10", nil)

			// Execute request
			rr := executeRequest(router, req)

			// Check for errors
			if rr.Code != http.StatusOK {
				errChan <- fmt.Errorf("request failed with status %d: %s", rr.Code, rr.Body.String())
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Check if any errors occurred
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	// There should be no errors
	assert.Empty(t, errors, "Concurrent requests should not produce errors")
}

// TestDatabaseConnectionFailures tests handling of database connection failures
func TestDatabaseConnectionFailures(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	defer func() {
		if testCtx, ok := ctx.(coreTesting.TestContext); ok {
			testCtx.Teardown()
		}
	}()
	mockCaseService := &MockCaseService{}

	// Setup mock expectations for database failure
	mockCaseService.On("GetByID", uint(1)).Return(nil, fmt.Errorf("database connection error"))

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE: mockCaseService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/cases/1", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "failed to fetch case")

	// Verify mock was called
	mockCaseService.AssertExpectations(t)

	// Test database failure in list endpoint
	mockCaseService = &MockCaseService{}
	mockCaseService.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return([]models.Case{}, int64(0), fmt.Errorf("database connection error"))

	router = configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE: mockCaseService,
	})

	req = createTestRequest(t, "GET", "/abuse/cases?_start=0&_end=10", nil)

	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "database connection error")

	// Test database failure in create endpoint
	mockCaseService = &MockCaseService{}
	mockCaseService.On("Create", mock.Anything).
		Return(nil, fmt.Errorf("database connection error"))

	router = configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE: mockCaseService,
	})

	reqBody := dto.CaseRequest{
		Type:        "spam",
		Description: "Test case",
		ReporterID:  1,
		SubjectID:   1,
	}
	req = createTestRequest(t, "POST", "/abuse/cases", reqBody)

	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "database connection error")
}

func (m *MockCaseService) ID() string {
	return typesSvc.CASE_SERVICE
}

func (m *MockCaseService) Config() (any, error) {
	return nil, nil
}

func (m *MockCaseService) Create(caseData *models.Case) (*models.Case, error) {
	args := m.Called(caseData)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Case), args.Error(1)
}

func (m *MockCaseService) GetByID(id uint) (*models.Case, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Case), args.Error(1)
}

func (m *MockCaseService) List(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Case, int64, error) {
	args := m.Called(filters, sorts, pagination)
	if args.Get(0) == nil {
		return []models.Case{}, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]models.Case), args.Get(1).(int64), args.Error(2)
}

func (m *MockCaseService) Update(caseModel *models.Case) error {
	args := m.Called(caseModel)
	return args.Error(0)
}

func (m *MockCaseService) UpdateStatus(id uint, status models.CaseStatus) error {
	args := m.Called(id, status)
	return args.Error(0)
}

func (m *MockCaseService) GetCaseByReference(reference string) (*models.Case, error) {
	args := m.Called(reference)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Case), args.Error(1)
}

func (m *MockCaseService) GetPublicCase(reference string, reporterID uint) (*models.Case, error) {
	args := m.Called(reference, reporterID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Case), args.Error(1)
}

func (m *MockCaseService) Search(ctx context.Context, query string, filters []queryutil.Filter, pagination queryutil.Pagination) ([]models.Case, int64, error) {
	args := m.Called(ctx, query, filters, pagination)
	return args.Get(0).([]models.Case), args.Get(1).(int64), args.Error(2)
}

func (m *MockCaseService) SendCreationNotification(caseID uint) error {
	args := m.Called(caseID)
	return args.Error(0)
}

func (m *MockCaseService) SendStatusUpdateNotification(caseID uint, oldStatus, newStatus models.CaseStatus) error {
	args := m.Called(caseID, oldStatus, newStatus)
	return args.Error(0)
}

// MockReporterService mocks the ReporterService interface
type MockReporterService struct {
	mock.Mock
}

func (m *MockReporterService) ID() string {
	return typesSvc.REPORTER_SERVICE
}

func (m *MockReporterService) Config() (any, error) {
	return nil, nil
}

func (m *MockReporterService) Create(reporter *models.Reporter) (*models.Reporter, error) {
	args := m.Called(reporter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Reporter), args.Error(1)
}

func (m *MockReporterService) GetByID(id uint) (*models.Reporter, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Reporter), args.Error(1)
}

func (m *MockReporterService) GetByEmail(email string) (*models.Reporter, error) {
	args := m.Called(email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Reporter), args.Error(1)
}

func (m *MockReporterService) List(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Reporter, int64, error) {
	args := m.Called(filters, sorts, pagination)
	return args.Get(0).([]models.Reporter), args.Get(1).(int64), args.Error(2)
}

func (m *MockReporterService) Update(reporter *models.Reporter) error {
	args := m.Called(reporter)
	return args.Error(0)
}

// MockSubjectService mocks the SubjectService interface
type MockSubjectService struct {
	mock.Mock
}

func (m *MockSubjectService) ID() string {
	return typesSvc.SUBJECT_SERVICE
}

func (m *MockSubjectService) Config() (any, error) {
	return nil, nil
}

func (m *MockSubjectService) Create(subject *models.Subject) (*models.Subject, error) {
	args := m.Called(subject)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Subject), args.Error(1)
}

func (m *MockSubjectService) GetByID(id uint) (*models.Subject, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Subject), args.Error(1)
}

func (m *MockSubjectService) FindOrCreate(identifier string, subjectType models.SubjectType) (*models.Subject, error) {
	args := m.Called(identifier, subjectType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Subject), args.Error(1)
}

func (m *MockSubjectService) List(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Subject, int64, error) {
	args := m.Called(filters, sorts, pagination)
	return args.Get(0).([]models.Subject), args.Get(1).(int64), args.Error(2)
}

// MockCommunicationService mocks the CommunicationService interface
type MockCommunicationService struct {
	mock.Mock
}

func (m *MockCommunicationService) ID() string {
	return typesSvc.COMMUNICATION_SERVICE
}

func (m *MockCommunicationService) Config() (any, error) {
	return nil, nil
}

func (m *MockCommunicationService) Create(comm *models.Communication) (*models.Communication, error) {
	args := m.Called(comm)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Communication), args.Error(1)
}

func (m *MockCommunicationService) GetByID(id uint) (*models.Communication, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Communication), args.Error(1)
}

func (m *MockCommunicationService) GetByThreadID(threadID string) (*models.Communication, error) {
	args := m.Called(threadID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Communication), args.Error(1)
}

func (m *MockCommunicationService) GetByCaseID(caseID uint, pagination queryutil.Pagination) ([]models.Communication, int64, error) {
	args := m.Called(caseID, pagination)
	if args.Get(0) == nil {
		return []models.Communication{}, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]models.Communication), args.Get(1).(int64), args.Error(2)
}

// MockScanService mocks the ScanService interface
type MockScanService struct {
	mock.Mock
}

func (m *MockScanService) ID() string {
	return typesSvc.SCAN_SERVICE
}

func (m *MockScanService) Config() (any, error) {
	return nil, nil
}

func (m *MockScanService) CreateScanFromData(caseID uint, data []byte, contentType, filename string) error {
	args := m.Called(caseID, data, contentType, filename)
	return args.Error(0)
}

func (m *MockScanService) GetScansForCase(caseID uint, pagination queryutil.Pagination) ([]models.CaseScan, int64, error) {
	args := m.Called(caseID, pagination)
	return args.Get(0).([]models.CaseScan), args.Get(1).(int64), args.Error(2)
}

func (m *MockScanService) GetScanResults(scanID uint) ([]models.ScanResult, error) {
	args := m.Called(scanID)
	return args.Get(0).([]models.ScanResult), args.Error(1)
}

// MockSearchService mocks the SearchService interface
type MockSearchService struct {
	mock.Mock
}

func (m *MockSearchService) ID() string {
	return typesSvc.SEARCH_SERVICE
}

func (m *MockSearchService) Config() (any, error) {
	return nil, nil
}

func (m *MockSearchService) SearchCases(ctx context.Context, query string, filters []queryutil.Filter, pagination queryutil.Pagination) ([]models.Case, int64, error) {
	args := m.Called(ctx, query, filters, pagination)
	return args.Get(0).([]models.Case), args.Get(1).(int64), args.Error(2)
}

func (m *MockSearchService) SearchReporters(ctx context.Context, query string, filters []queryutil.Filter, pagination queryutil.Pagination) ([]models.Reporter, int64, error) {
	args := m.Called(ctx, query, filters, pagination)
	return args.Get(0).([]models.Reporter), args.Get(1).(int64), args.Error(2)
}

func (m *MockSearchService) GlobalSearch(ctx context.Context, query string, limit int) (map[string]interface{}, error) {
	args := m.Called(ctx, query, limit)
	return args.Get(0).(map[string]interface{}), args.Error(1)
}

// MockEmailService mocks the EmailService interface
type MockEmailService struct {
	mock.Mock
}

func (m *MockEmailService) ID() string {
	return typesSvc.EMAIL_SERVICE
}

func (m *MockEmailService) Config() (any, error) {
	return nil, nil
}

func (m *MockEmailService) SendEmail(to []string, subject, body string, replyTo string) error {
	args := m.Called(to, subject, body, replyTo)
	return args.Error(0)
}

func (m *MockEmailService) SendTemplatedEmail(to []string, subject, templateName string, data interface{}, replyTo string) error {
	args := m.Called(to, subject, templateName, data, replyTo)
	return args.Error(0)
}

func (m *MockEmailService) SendCaseAccessEmail(caseID uint, reporterID uint, email string) error {
	args := m.Called(caseID, reporterID, email)
	return args.Error(0)
}

func (m *MockEmailService) ProcessIncomingEmail(ctx context.Context, rawEmail io.Reader) error {
	args := m.Called(ctx, rawEmail)
	return args.Error(0)
}

func (m *MockEmailService) GenerateCaseThreadID(caseID uint, referenceNumber string) string {
	args := m.Called(caseID, referenceNumber)
	return args.String(0)
}

// MockTokenService mocks the TokenService interface
type MockTokenService struct {
	mock.Mock
}

func (m *MockTokenService) ID() string {
	return typesSvc.TOKEN_SERVICE
}

func (m *MockTokenService) Config() (any, error) {
	return nil, nil
}

func (m *MockTokenService) GenerateToken(caseID, reporterID uint, validDays int) (string, error) {
	args := m.Called(caseID, reporterID, validDays)
	return args.String(0), args.Error(1)
}

func (m *MockTokenService) ValidateToken(token string) (caseID, reporterID uint, valid bool) {
	args := m.Called(token)
	return args.Get(0).(uint), args.Get(1).(uint), args.Bool(2)
}

func (m *MockTokenService) GetTokenParts(token string) []string {
	args := m.Called(token)
	return args.Get(0).([]string)
}

// MockAdminAccessService mocks the AccessService interface
type MockAdminAccessService struct {
	mock.Mock
}

func (m *MockAdminAccessService) ID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAdminAccessService) RegisterRoute(apiName, path, method, access string) error {
	args := m.Called(apiName, path, method, access)
	return args.Error(0)
}

func (m *MockAdminAccessService) CheckAccess(userID uint, apiName, path, method string) (bool, error) {
	args := m.Called(userID, apiName, path, method)
	return args.Bool(0), args.Error(1)
}

func (m *MockAdminAccessService) AssignRoleToUser(userID uint, role string) error {
	args := m.Called(userID, role)
	return args.Error(0)
}

func (m *MockAdminAccessService) ExportModel() *core.AccessModel {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*core.AccessModel)
}

func (m *MockAdminAccessService) ExportUserPolicy(userID uint) ([]*core.AccessPolicy, error) {
	args := m.Called(userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*core.AccessPolicy), args.Error(1)
}

func (m *MockAdminAccessService) RemoveRoleFromUser(userID uint, role string) error {
	args := m.Called(userID, role)
	return args.Error(0)
}

func (m *MockAdminAccessService) GetUserRoles(userID uint) ([]string, error) {
	args := m.Called(userID)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAdminAccessService) HasRole(userID uint, role string) (bool, error) {
	args := m.Called(userID, role)
	return args.Bool(0), args.Error(1)
}

// Test setup helpers

// createTestContext creates a test context with mocked services
func createTestContext(t *testing.T) core.Context {
	ctx := coreTesting.NewTestContext(t)
	return ctx
}

// configureTestRouter creates a router with the admin extension configured
func configureTestRouter(t *testing.T, ctx core.Context, mockServices map[string]interface{}) *mux.Router {
	// Add mock services to context
	testCtx, ok := ctx.(coreTesting.TestContext)
	if !ok {
		t.Fatalf("context does not implement TestContext interface")
	}

	for id, svc := range mockServices {
		testCtx.RegisterService(id, svc)
	}

	// Create router
	router := mux.NewRouter()

	// Create mock access service
	mockAccessSvc := &MockAdminAccessService{}
	mockAccessSvc.On("ID").Return("access")
	mockAccessSvc.On("RegisterRoute", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Create admin extension
	adminExt := NewAdminExtension(ctx)

	// Configure the extension
	err := adminExt.Configure(router, mockAccessSvc)
	require.NoError(t, err)

	return router
}

// createTestRequest creates an HTTP request for testing
func createTestRequest(t *testing.T, method, path string, body interface{}) *http.Request {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, path, reqBody)
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// executeRequest executes a request against the router
func executeRequest(router *mux.Router, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// parseResponse parses the JSON response into the given struct
func parseResponse(t *testing.T, rr *httptest.ResponseRecorder, v interface{}) {
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), v))
}

// Test cases for each API endpoint

// TestCreateCase tests the case creation endpoint
func TestCreateCase(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockCaseService := &MockCaseService{}

	// Create test data
	now := time.Now()
	testCase := &models.Case{
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}
	// Set gorm.Model fields
	testCase.ID = 1
	testCase.CreatedAt = now
	testCase.UpdatedAt = now

	// Setup mock expectations
	mockCaseService.On("Create", mock.AnythingOfType("*models.Case")).Return(testCase, nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE: mockCaseService,
	})

	// Create request
	reqBody := dto.CaseRequest{
		Type:        "spam",
		Description: "Test case",
		Priority:    "medium",
		Source:      "web_form",
		NeedsReview: true,
		ReporterID:  1,
		SubjectID:   1,
	}
	req := createTestRequest(t, "POST", "/abuse/cases", reqBody)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusCreated, rr.Code)

	// Parse response
	var response dto.CaseResponse
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, uint(1), response.ID)
	assert.Equal(t, "CASE-123456", response.ReferenceNumber)
	assert.Equal(t, "spam", response.Type)
	assert.Equal(t, "new", response.Status)
	assert.Equal(t, "medium", response.Priority)
	assert.Equal(t, "Test case", response.Description)
	assert.Equal(t, "web_form", response.Source)
	assert.True(t, response.NeedsReview)
	assert.Equal(t, uint(1), response.ReporterID)
	assert.Equal(t, uint(1), response.SubjectID)

	// Verify mock was called
	mockCaseService.AssertExpectations(t)
}

// TestCreateCase_ValidationError tests case creation with invalid data
func TestCreateCase_ValidationError(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockCaseService := &MockCaseService{}

	// Setup mock expectations
	mockCaseService.On("Create", mock.AnythingOfType("*models.Case")).
		Return(nil, fmt.Errorf("invalid case data: case type is required"))

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE: mockCaseService,
	})

	// Create request with invalid data
	reqBody := dto.CaseRequest{
		// Missing required fields
		Description: "Test case",
		ReporterID:  1,
		SubjectID:   1,
	}
	req := createTestRequest(t, "POST", "/abuse/cases", reqBody)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid case data")
}

// TestGetCase tests retrieving a case by ID
func TestGetCase(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockCaseService := &MockCaseService{}

	// Create test data
	now := time.Now()
	testCase := &models.Case{
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}
	// Set gorm.Model fields
	testCase.ID = 1
	testCase.CreatedAt = now
	testCase.UpdatedAt = now

	// Setup mock expectations
	mockCaseService.On("GetByID", uint(1)).Return(testCase, nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE: mockCaseService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/cases/1", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response dto.CaseResponse
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, uint(1), response.ID)
	assert.Equal(t, "CASE-123456", response.ReferenceNumber)
	assert.Equal(t, "spam", response.Type)
	assert.Equal(t, "new", response.Status)

	// Verify mock was called
	mockCaseService.AssertExpectations(t)
}

// TestGetCase_NotFound tests retrieving a non-existent case
func TestGetCase_NotFound(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockCaseService := &MockCaseService{}

	// Setup mock expectations
	mockCaseService.On("GetByID", uint(999)).Return(nil, fmt.Errorf("case not found"))

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE: mockCaseService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/cases/999", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusNotFound, rr.Code)
	assert.Contains(t, rr.Body.String(), "not found")
}

// TestListCases tests listing cases with pagination and filtering
func TestListCases(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockCaseService := &MockCaseService{}

	// Create test data
	now := time.Now()
	testCases := []models.Case{
		{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			ReferenceNumber: "CASE-123456",
			Type:            models.CaseTypeSpam,
			Status:          models.CaseStatusNew,
			Priority:        models.CasePriorityMedium,
			Description:     "Test case 1",
			Source:          models.ReportSourceWebForm,
			NeedsReview:     true,
			ReporterID:      1,
			SubjectID:       1,
		},
		{
			Model: gorm.Model{
				ID:        2,
				CreatedAt: now,
				UpdatedAt: now,
			},
			ReferenceNumber: "CASE-789012",
			Type:            models.CaseTypeHarassment,
			Status:          models.CaseStatusInProgress,
			Priority:        models.CasePriorityHigh,
			Description:     "Test case 2",
			Source:          models.ReportSourceEmail,
			NeedsReview:     false,
			ReporterID:      2,
			SubjectID:       2,
		},
	}

	// Setup mock expectations
	mockCaseService.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(testCases, int64(2), nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE: mockCaseService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/cases?_start=0&_end=10", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var responseData struct {
		Data []dto.CaseResponse `json:"data"`
	}
	parseResponse(t, rr, &responseData)

	// Verify response
	assert.Len(t, responseData.Data, 2)
	assert.Equal(t, uint(1), responseData.Data[0].ID)
	assert.Equal(t, "CASE-123456", responseData.Data[0].ReferenceNumber)
	assert.Equal(t, uint(2), responseData.Data[1].ID)
	assert.Equal(t, "CASE-789012", responseData.Data[1].ReferenceNumber)

	// Verify Content-Range header
	assert.Equal(t, "cases 0-1/2", rr.Header().Get("Content-Range"))

	// Verify mock was called
	mockCaseService.AssertExpectations(t)
}

// TestUpdateCase tests updating a case
func TestUpdateCase(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockCaseService := &MockCaseService{}

	// Create test data
	now := time.Now()
	existingCase := &models.Case{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Original description",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}

	// Setup mock expectations
	mockCaseService.On("GetByID", uint(1)).Return(existingCase, nil)
	mockCaseService.On("Update", mock.AnythingOfType("*models.Case")).Return(nil).Run(func(args mock.Arguments) {
		// Update the case with the new values
		case_ := args.Get(0).(*models.Case)
		case_.Priority = models.CasePriorityHigh
		case_.Description = "Updated description"
		case_.NeedsReview = false
	})

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE: mockCaseService,
	})

	// Create request
	reqBody := dto.CaseRequest{
		Priority:    "high",
		Description: "Updated description",
		NeedsReview: false,
	}
	req := createTestRequest(t, "PUT", "/abuse/cases/1", reqBody)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response dto.CaseResponse
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, uint(1), response.ID)
	assert.Equal(t, "high", response.Priority)
	assert.Equal(t, "Updated description", response.Description)
	assert.False(t, response.NeedsReview)

	// Verify mock was called
	mockCaseService.AssertExpectations(t)
}

// TestUpdateCaseStatus tests updating a case's status
func TestUpdateCaseStatus(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockCaseService := &MockCaseService{}

	// Create test data
	now := time.Now()
	existingCase := &models.Case{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}

	updatedCase := &models.Case{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusInProgress, // Changed
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}

	// Setup mock expectations
	mockCaseService.On("GetByID", uint(1)).Return(existingCase, nil).Once()
	mockCaseService.On("UpdateStatus", uint(1), models.CaseStatusInProgress).Return(nil)
	mockCaseService.On("GetByID", uint(1)).Return(updatedCase, nil).Once()
	mockCaseService.On("SendStatusUpdateNotification", uint(1), models.CaseStatusNew, models.CaseStatusInProgress).Return(nil).Maybe()

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE: mockCaseService,
	})

	// Create request
	reqBody := StatusUpdateRequest{
		Status: "in_progress",
	}
	req := createTestRequest(t, "PUT", "/abuse/cases/1/status", reqBody)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response dto.CaseResponse
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, uint(1), response.ID)
	assert.Equal(t, "in_progress", response.Status)

	// Verify mock was called
	mockCaseService.AssertExpectations(t)
}

// TestCreateReporter tests creating a reporter
func TestCreateReporter(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockReporterService := &MockReporterService{}

	// Create test data
	now := time.Now()
	testReporter := &models.Reporter{
		Email: "test@example.com",
		Name:  "Test Reporter",
	}
	// Set gorm.Model fields
	testReporter.ID = 1
	testReporter.CreatedAt = now
	testReporter.UpdatedAt = now

	// Setup mock expectations
	mockReporterService.On("Create", mock.AnythingOfType("*models.Reporter")).Return(testReporter, nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.REPORTER_SERVICE: mockReporterService,
	})

	// Create request
	reqBody := dto.ReporterRequest{
		Email: "test@example.com",
		Name:  "Test Reporter",
	}
	req := createTestRequest(t, "POST", "/abuse/reporters", reqBody)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusCreated, rr.Code)

	// Parse response
	var response dto.ReporterResponse
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, uint(1), response.ID)
	assert.Equal(t, "test@example.com", response.Email)
	assert.Equal(t, "Test Reporter", response.Name)

	// Verify mock was called
	mockReporterService.AssertExpectations(t)
}

// TestGetReporter tests retrieving a reporter by ID
func TestGetReporter(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockReporterService := &MockReporterService{}

	// Create test data
	now := time.Now()
	testReporter := &models.Reporter{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Email: "test@example.com",
		Name:  "Test Reporter",
	}

	// Setup mock expectations
	mockReporterService.On("GetByID", uint(1)).Return(testReporter, nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.REPORTER_SERVICE: mockReporterService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/reporters/1", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response dto.ReporterResponse
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, uint(1), response.ID)
	assert.Equal(t, "test@example.com", response.Email)
	assert.Equal(t, "Test Reporter", response.Name)

	// Verify mock was called
	mockReporterService.AssertExpectations(t)
}

// TestListReporters tests listing reporters with pagination and filtering
func TestListReporters(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockReporterService := &MockReporterService{}

	// Create test data
	now := time.Now()
	testReporters := []models.Reporter{
		{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Email: "test1@example.com",
			Name:  "Test Reporter 1",
		},
		{
			Model: gorm.Model{
				ID:        2,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Email: "test2@example.com",
			Name:  "Test Reporter 2",
		},
	}

	// Setup mock expectations
	mockReporterService.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(testReporters, int64(2), nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.REPORTER_SERVICE: mockReporterService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/reporters?_start=0&_end=10", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var responseData struct {
		Data []dto.ReporterResponse `json:"data"`
	}
	parseResponse(t, rr, &responseData)

	// Verify response
	assert.Len(t, responseData.Data, 2)
	assert.Equal(t, uint(1), responseData.Data[0].ID)
	assert.Equal(t, "test1@example.com", responseData.Data[0].Email)
	assert.Equal(t, uint(2), responseData.Data[1].ID)
	assert.Equal(t, "test2@example.com", responseData.Data[1].Email)

	// Verify Content-Range header
	assert.Equal(t, "reporters 0-1/2", rr.Header().Get("Content-Range"))

	// Verify mock was called
	mockReporterService.AssertExpectations(t)
}

// TestCreateSubject tests creating a subject
func TestCreateSubject(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockSubjectService := &MockSubjectService{}

	// Create test data
	now := time.Now()
	testSubject := &models.Subject{
		Identifier: "test-identifier",
		Type:       models.SubjectTypeHash,
	}
	// Set gorm.Model fields
	testSubject.ID = 1
	testSubject.CreatedAt = now
	testSubject.UpdatedAt = now

	// Setup mock expectations
	mockSubjectService.On("Create", mock.AnythingOfType("*models.Subject")).Return(testSubject, nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.SUBJECT_SERVICE: mockSubjectService,
	})

	// Create request
	reqBody := dto.SubjectRequest{
		Identifier: "test-identifier",
	}
	req := createTestRequest(t, "POST", "/abuse/subjects", reqBody)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusCreated, rr.Code)

	// Parse response
	var response dto.SubjectResponse
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, uint(1), response.ID)
	assert.Equal(t, "test-identifier", response.Identifier)

	// Verify mock was called
	mockSubjectService.AssertExpectations(t)
}

// TestGetSubject tests retrieving a subject by ID
func TestGetSubject(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockSubjectService := &MockSubjectService{}

	// Create test data
	now := time.Now()
	testSubject := &models.Subject{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Identifier: "test-identifier",
		Type:       models.SubjectTypeHash,
	}

	// Setup mock expectations
	mockSubjectService.On("GetByID", uint(1)).Return(testSubject, nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.SUBJECT_SERVICE: mockSubjectService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/subjects/1", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response dto.SubjectResponse
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, uint(1), response.ID)
	assert.Equal(t, "test-identifier", response.Identifier)

	// Verify mock was called
	mockSubjectService.AssertExpectations(t)
}

// TestListSubjects tests listing subjects with pagination and filtering
func TestListSubjects(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockSubjectService := &MockSubjectService{}

	// Create test data
	now := time.Now()
	testSubjects := []models.Subject{
		{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Identifier: "test-identifier-1",
			Type:       models.SubjectTypeHash,
		},
		{
			Model: gorm.Model{
				ID:        2,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Identifier: "test-identifier-2",
			Type:       models.SubjectTypeHash,
		},
	}

	// Setup mock expectations
	mockSubjectService.On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(testSubjects, int64(2), nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.SUBJECT_SERVICE: mockSubjectService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/subjects?_start=0&_end=10", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var responseData struct {
		Data []dto.SubjectResponse `json:"data"`
	}
	parseResponse(t, rr, &responseData)

	// Verify response
	assert.Len(t, responseData.Data, 2)
	assert.Equal(t, uint(1), responseData.Data[0].ID)
	assert.Equal(t, "test-identifier-1", responseData.Data[0].Identifier)
	assert.Equal(t, uint(2), responseData.Data[1].ID)
	assert.Equal(t, "test-identifier-2", responseData.Data[1].Identifier)

	// Verify Content-Range header
	assert.Equal(t, "subjects 0-1/2", rr.Header().Get("Content-Range"))

	// Verify mock was called
	mockSubjectService.AssertExpectations(t)
}

// TestListCaseCommunications tests listing communications for a case
func TestListCaseCommunications(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	defer func() {
		if testCtx, ok := ctx.(coreTesting.TestContext); ok {
			testCtx.Teardown()
		}
	}()
	mockCaseService := &MockCaseService{}
	mockCommunicationService := &MockCommunicationService{}

	// Create test data
	now := time.Now()
	testCase := &models.Case{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}

	testCommunications := []models.Communication{
		{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			CaseID:    1,
			SenderID:  1,
			Type:      models.CommunicationTypeNote,
			Direction: models.CommunicationDirectionInternal,
			Content:   "Test note 1",
			ThreadID:  "thread-1",
		},
		{
			Model: gorm.Model{
				ID:        2,
				CreatedAt: now,
				UpdatedAt: now,
			},
			CaseID:    1,
			SenderID:  1,
			Type:      models.CommunicationTypeEmail,
			Direction: models.CommunicationDirectionOutgoing,
			Content:   "Test email 1",
			ThreadID:  "thread-2",
		},
	}

	// Setup mock expectations
	mockCaseService.On("GetByID", uint(1)).Return(testCase, nil)
	mockCommunicationService.On("GetByCaseID", uint(1), mock.Anything).
		Return(testCommunications, int64(2), nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE:          mockCaseService,
		typesSvc.COMMUNICATION_SERVICE: mockCommunicationService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/cases/1/communications", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var responseData struct {
		Data []map[string]interface{} `json:"data"`
	}
	parseResponse(t, rr, &responseData)

	// Verify response
	assert.Len(t, responseData.Data, 2)
	assert.Equal(t, float64(1), responseData.Data[0]["id"])
	assert.Equal(t, "note", responseData.Data[0]["type"])
	assert.Equal(t, "internal", responseData.Data[0]["direction"])
	assert.Equal(t, "Test note 1", responseData.Data[0]["content"])

	assert.Equal(t, float64(2), responseData.Data[1]["id"])
	assert.Equal(t, "email", responseData.Data[1]["type"])
	assert.Equal(t, "outgoing", responseData.Data[1]["direction"])
	assert.Equal(t, "Test email 1", responseData.Data[1]["content"])

	// Verify mock was called
	mockCaseService.AssertExpectations(t)
	mockCommunicationService.AssertExpectations(t)
}

// TestAddCaseCommunication tests adding a communication to a case
func TestAddCaseCommunication(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockCaseService := &MockCaseService{}
	mockCommunicationService := &MockCommunicationService{}
	mockEmailService := &MockEmailService{}

	// Create test data
	now := time.Now()
	testCase := &models.Case{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}

	testCommunication := &models.Communication{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		CaseID:    1,
		SenderID:  1,
		Type:      models.CommunicationTypeNote,
		Direction: models.CommunicationDirectionInternal,
		Content:   "Test note",
	}

	// Setup mock expectations
	mockCaseService.On("GetByID", uint(1)).Return(testCase, nil)
	mockEmailService.On("GenerateCaseThreadID", uint(1), "CASE-123456").Return("thread-id").Maybe()
	mockCommunicationService.On("Create", mock.AnythingOfType("*models.Communication")).Return(testCommunication, nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE:          mockCaseService,
		typesSvc.COMMUNICATION_SERVICE: mockCommunicationService,
		typesSvc.EMAIL_SERVICE:         mockEmailService,
	})

	// Create request
	reqBody := CommunicationRequest{
		Content:   "Test note",
		Type:      "note",
		Direction: "internal",
	}
	req := createTestRequest(t, "POST", "/abuse/cases/1/communications", reqBody)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusCreated, rr.Code)

	// Parse response
	var response map[string]interface{}
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, float64(1), response["id"])
	assert.Equal(t, float64(1), response["case_id"])
	assert.Equal(t, float64(1), response["sender_id"])
	assert.Equal(t, "note", response["type"])
	assert.Equal(t, "internal", response["direction"])
	assert.Equal(t, "Test note", response["content"])

	// Verify mock was called
	mockCaseService.AssertExpectations(t)
	mockCommunicationService.AssertExpectations(t)
}

// TestListCaseScans tests listing scans for a case
func TestListCaseScans(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	defer func() {
		if testCtx, ok := ctx.(coreTesting.TestContext); ok {
			testCtx.Teardown()
		}
	}()
	mockCaseService := &MockCaseService{}
	mockScanService := &MockScanService{}

	// Create test data
	now := time.Now()
	testCase := &models.Case{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}

	testScans := []models.CaseScan{
		{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			CaseID:       1,
			Hash:         []byte("hash1"),
			Status:       models.ScanStatusClean,
			Type:         "text/plain",
			ScheduledFor: now,
			FileInfo: map[string]interface{}{
				"filename": "file1.txt",
			},
		},
		{
			Model: gorm.Model{
				ID:        2,
				CreatedAt: now,
				UpdatedAt: now,
			},
			CaseID:       1,
			Hash:         []byte("hash2"),
			Status:       models.ScanStatusFlagged,
			Type:         "image/jpeg",
			ScheduledFor: now,
			FileInfo: map[string]interface{}{
				"filename": "file2.jpg",
			},
		},
	}

	// Setup mock expectations
	mockCaseService.On("GetByID", uint(1)).Return(testCase, nil)
	mockScanService.On("GetScansForCase", uint(1), mock.Anything).
		Return(testScans, int64(2), nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE: mockCaseService,
		typesSvc.SCAN_SERVICE: mockScanService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/cases/1/scans", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var responseData struct {
		Data []map[string]interface{} `json:"data"`
	}
	parseResponse(t, rr, &responseData)

	// Verify response
	assert.Len(t, responseData.Data, 2)
	assert.Equal(t, float64(1), responseData.Data[0]["id"])
	assert.Equal(t, float64(1), responseData.Data[0]["case_id"])
	assert.Equal(t, "clean", responseData.Data[0]["status"])
	assert.Equal(t, "text/plain", responseData.Data[0]["type"])

	assert.Equal(t, float64(2), responseData.Data[1]["id"])
	assert.Equal(t, float64(1), responseData.Data[1]["case_id"])
	assert.Equal(t, "flagged", responseData.Data[1]["status"])
	assert.Equal(t, "image/jpeg", responseData.Data[1]["type"])

	// Verify mock was called
	mockCaseService.AssertExpectations(t)
	mockScanService.AssertExpectations(t)
}

// TestGetScan tests retrieving a scan by ID
func TestGetScan(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockScanService := &MockScanService{}

	// Create test data
	now := time.Now()
	testScan := models.CaseScan{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		CaseID:       1,
		Hash:         []byte("hash1"),
		Status:       models.ScanStatusClean,
		Type:         "text/plain",
		ScheduledFor: now,
		FileInfo: map[string]interface{}{
			"filename": "file1.txt",
		},
	}

	// Setup mock expectations
	mockScanService.On("GetScansForCase", uint(0), mock.Anything).
		Return([]models.CaseScan{testScan}, int64(1), nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.SCAN_SERVICE: mockScanService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/scans/1", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response map[string]interface{}
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, float64(1), response["id"])
	assert.Equal(t, float64(1), response["case_id"])
	assert.Equal(t, "clean", response["status"])
	assert.Equal(t, "text/plain", response["type"])
	assert.Equal(t, "file1.txt", response["file_info"].(map[string]interface{})["filename"])
}

// TestGetScanResults tests retrieving scan results
func TestGetScanResults(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockScanService := &MockScanService{}

	// Create test data
	now := time.Now()
	testResults := []models.ScanResult{
		{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			CaseScanID: 1,
			ScannerID:  "scanner1",
			Passed:     true,
			Reason:     "Clean content",
			Timestamp:  now,
			Metadata: map[string]interface{}{
				"confidence": 0.95,
			},
		},
		{
			Model: gorm.Model{
				ID:        2,
				CreatedAt: now,
				UpdatedAt: now,
			},
			CaseScanID: 1,
			ScannerID:  "scanner2",
			Passed:     false,
			Reason:     "Flagged content",
			Timestamp:  now,
			Metadata: map[string]interface{}{
				"confidence": 0.85,
			},
		},
	}

	// No need to create the scan in the test DB since we're mocking

	// Setup mock expectations
	mockScanService.On("GetScanResults", uint(1)).Return(testResults, nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.SCAN_SERVICE: mockScanService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/scans/1/results", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response []map[string]interface{}
	parseResponse(t, rr, &response)

	// Verify response
	assert.Len(t, response, 2)
	assert.Equal(t, float64(1), response[0]["id"])
	assert.Equal(t, "scanner1", response[0]["scanner_id"])
	assert.Equal(t, true, response[0]["passed"])
	assert.Equal(t, "Clean content", response[0]["reason"])

	assert.Equal(t, float64(2), response[1]["id"])
	assert.Equal(t, "scanner2", response[1]["scanner_id"])
	assert.Equal(t, false, response[1]["passed"])
	assert.Equal(t, "Flagged content", response[1]["reason"])

	// Verify mock was called
	mockScanService.AssertExpectations(t)
}

// TestUploadScan tests uploading a file for scanning
func TestUploadScan(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockCaseService := &MockCaseService{}
	mockScanService := &MockScanService{}

	// Create test data
	now := time.Now()
	testCase := &models.Case{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}

	// Setup mock expectations
	mockCaseService.On("GetByID", uint(1)).Return(testCase, nil)
	mockScanService.On("CreateScanFromData", uint(1), mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.CASE_SERVICE: mockCaseService,
		typesSvc.SCAN_SERVICE: mockScanService,
	})

	// Create a multipart form buffer
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Create a form file field
	fileContents := []byte("test file contents")
	fileField, err := w.CreateFormFile("file", "test.txt")
	require.NoError(t, err)

	// Write the file contents
	_, err = fileField.Write(fileContents)
	require.NoError(t, err)

	// Close the multipart writer
	err = w.Close()
	require.NoError(t, err)

	// Create the request
	req, err := http.NewRequest("POST", "/abuse/cases/1/scans", &b)
	require.NoError(t, err)

	// Set the content type to the multipart form's content type
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Execute the request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusCreated, rr.Code)

	// Parse response
	var response map[string]interface{}
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, "File uploaded and queued for scanning", response["message"])
	assert.Equal(t, "test.txt", response["filename"])

	// Verify mock was called
	mockCaseService.AssertExpectations(t)
	mockScanService.AssertExpectations(t)
}

// TestSearchCases tests searching for cases
func TestSearchCases(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	defer func() {
		if testCtx, ok := ctx.(coreTesting.TestContext); ok {
			testCtx.Teardown()
		}
	}()
	mockSearchService := &MockSearchService{}

	// Create test data
	now := time.Now()
	testCases := []models.Case{
		{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			ReferenceNumber: "CASE-123456",
			Type:            models.CaseTypeSpam,
			Status:          models.CaseStatusNew,
			Priority:        models.CasePriorityMedium,
			Description:     "Test case with bitcoin",
			Source:          models.ReportSourceWebForm,
			NeedsReview:     true,
			ReporterID:      1,
			SubjectID:       1,
		},
	}

	// Setup mock expectations
	mockSearchService.On("SearchCases", mock.Anything, "bitcoin", mock.Anything, mock.Anything).
		Return(testCases, int64(1), nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.SEARCH_SERVICE: mockSearchService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/search/cases?q=bitcoin", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var responseData struct {
		Data []dto.CaseResponse `json:"data"`
	}
	parseResponse(t, rr, &responseData)

	response := responseData.Data

	// Verify response
	assert.Len(t, response, 1)
	assert.Equal(t, uint(1), response[0].ID)
	assert.Equal(t, "CASE-123456", response[0].ReferenceNumber)
	assert.Contains(t, response[0].Description, "bitcoin")

	// Verify mock was called
	mockSearchService.AssertExpectations(t)
}

// TestGlobalSearch tests global search across multiple entities
func TestGlobalSearch(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockSearchService := &MockSearchService{}

	// Create test data
	now := time.Now()
	testCases := []models.Case{
		{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			ReferenceNumber: "CASE-123456",
			Type:            models.CaseTypeSpam,
			Status:          models.CaseStatusNew,
			Priority:        models.CasePriorityMedium,
			Description:     "Test case with bitcoin",
			Source:          models.ReportSourceWebForm,
			NeedsReview:     true,
			ReporterID:      1,
			SubjectID:       1,
		},
	}

	testReporters := []models.Reporter{
		{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			Email: "bitcoin@example.com",
			Name:  "Bitcoin Reporter",
		},
	}

	// Setup mock expectations
	mockSearchService.On("GlobalSearch", mock.Anything, "bitcoin", 10).
		Return(map[string]interface{}{
			"cases":     testCases,
			"reporters": testReporters,
		}, nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.SEARCH_SERVICE: mockSearchService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/abuse/search/global?q=bitcoin", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response map[string]interface{}
	parseResponse(t, rr, &response)

	// Verify response
	if cases, ok := response["cases"].([]interface{}); ok {
		assert.Len(t, cases, 1)
	}
	if reporters, ok := response["reporters"].([]interface{}); ok {
		assert.Len(t, reporters, 1)
	}

	// Verify mock was called
	mockSearchService.AssertExpectations(t)
}

// TestUploadPublicFile tests uploading a file from a reporter
func TestUploadPublicFile(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockCaseService := &MockCaseService{}
	mockScanService := &MockScanService{}
	mockCommunicationService := &MockCommunicationService{}
	mockTokenService := &MockTokenService{}

	// Create test data
	now := time.Now()
	testCase := &models.Case{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}

	testCommunication := &models.Communication{
		CaseID:    1,
		SenderID:  1,
		Type:      models.CommunicationType("note"),
		Direction: models.CommunicationDirectionExternal,
		Content:   "File uploaded: test.txt",
	}

	// Set gorm.Model fields
	testCommunication.ID = 1
	testCommunication.CreatedAt = now
	testCommunication.UpdatedAt = now

	// Setup mock expectations
	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockCaseService.On("GetByID", uint(1)).Return(testCase, nil)
	mockScanService.On("CreateScanFromData", uint(1), mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockCommunicationService.On("Create", mock.AnythingOfType("*models.Communication")).Return(testCommunication, nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE:         mockTokenService,
		typesSvc.CASE_SERVICE:          mockCaseService,
		typesSvc.SCAN_SERVICE:          mockScanService,
		typesSvc.COMMUNICATION_SERVICE: mockCommunicationService,
	})

	// Create a multipart form buffer
	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// Create a form file field
	fileContents := []byte("test file contents")
	fileField, err := w.CreateFormFile("file", "test.txt")
	require.NoError(t, err)

	// Write the file contents
	_, err = fileField.Write(fileContents)
	require.NoError(t, err)

	// Close the multipart writer
	err = w.Close()
	require.NoError(t, err)

	// Create the request
	req, err := http.NewRequest("POST", "/api/abuse/public/cases/CASE-123456/upload?token=test-token", &b)
	require.NoError(t, err)

	// Set the content type to the multipart form's content type
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Execute the request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusCreated, rr.Code)

	// Parse response
	var response map[string]interface{}
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, "File uploaded successfully", response["message"])
	assert.Equal(t, "test.txt", response["filename"])

	// Verify mocks were called
	mockTokenService.AssertExpectations(t)
	mockCaseService.AssertExpectations(t)
	mockScanService.AssertExpectations(t)
	mockCommunicationService.AssertExpectations(t)

	// Test error cases

	// Test 1: Reference number mismatch
	mockTokenService = &MockTokenService{}
	mockCaseService = &MockCaseService{}

	differentCase := &models.Case{
		Model: gorm.Model{
			ID: 1,
		},
		ReferenceNumber: "CASE-DIFFERENT",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		ReporterID:      1,
		SubjectID:       1,
	}

	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockCaseService.On("GetByID", uint(1)).Return(differentCase, nil)

	router = configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE: mockTokenService,
		typesSvc.CASE_SERVICE:  mockCaseService,
	})

	// Create a new multipart form buffer
	b = bytes.Buffer{}
	w = multipart.NewWriter(&b)

	fileField, err = w.CreateFormFile("file", "test.txt")
	require.NoError(t, err)

	_, err = fileField.Write(fileContents)
	require.NoError(t, err)

	err = w.Close()
	require.NoError(t, err)

	req, err = http.NewRequest("POST", "/api/abuse/public/cases/CASE-123456/upload?token=test-token", &b)
	require.NoError(t, err)

	req.Header.Set("Content-Type", w.FormDataContentType())

	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "case reference mismatch")

	// Test 2: Reporter ID mismatch
	mockTokenService = &MockTokenService{}
	mockCaseService = &MockCaseService{}

	unauthorizedCase := &models.Case{
		Model: gorm.Model{
			ID: 1,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		ReporterID:      2, // Different reporter ID
		SubjectID:       1,
	}

	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockCaseService.On("GetByID", uint(1)).Return(unauthorizedCase, nil)

	router = configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE: mockTokenService,
		typesSvc.CASE_SERVICE:  mockCaseService,
	})

	// Create a new multipart form buffer
	b = bytes.Buffer{}
	w = multipart.NewWriter(&b)

	fileField, err = w.CreateFormFile("file", "test.txt")
	require.NoError(t, err)

	_, err = fileField.Write(fileContents)
	require.NoError(t, err)

	err = w.Close()
	require.NoError(t, err)

	req, err = http.NewRequest("POST", "/api/abuse/public/cases/CASE-123456/upload?token=test-token", &b)
	require.NoError(t, err)

	req.Header.Set("Content-Type", w.FormDataContentType())

	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "unauthorized access")

	// Test 3: Missing file in form
	mockTokenService = &MockTokenService{}
	mockCaseService = &MockCaseService{}

	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockCaseService.On("GetByID", uint(1)).Return(testCase, nil)

	router = configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE: mockTokenService,
		typesSvc.CASE_SERVICE:  mockCaseService,
	})

	// Create a form without a file
	b = bytes.Buffer{}
	w = multipart.NewWriter(&b)

	// Add a text field instead of a file
	w.WriteField("text_field", "not a file")

	err = w.Close()
	require.NoError(t, err)

	req, err = http.NewRequest("POST", "/api/abuse/public/cases/CASE-123456/upload?token=test-token", &b)
	require.NoError(t, err)

	req.Header.Set("Content-Type", w.FormDataContentType())

	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "failed to get file")

	// Test 4: Scan service error
	mockTokenService = &MockTokenService{}
	mockCaseService = &MockCaseService{}
	mockScanService = &MockScanService{}

	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockCaseService.On("GetByID", uint(1)).Return(testCase, nil)
	mockScanService.On("CreateScanFromData", uint(1), mock.Anything, mock.Anything, mock.Anything).
		Return(fmt.Errorf("scan service error"))

	router = configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE: mockTokenService,
		typesSvc.CASE_SERVICE:  mockCaseService,
		typesSvc.SCAN_SERVICE:  mockScanService,
	})

	// Create a new multipart form buffer
	b = bytes.Buffer{}
	w = multipart.NewWriter(&b)

	fileField, err = w.CreateFormFile("file", "test.txt")
	require.NoError(t, err)

	_, err = fileField.Write(fileContents)
	require.NoError(t, err)

	err = w.Close()
	require.NoError(t, err)

	req, err = http.NewRequest("POST", "/api/abuse/public/cases/CASE-123456/upload?token=test-token", &b)
	require.NoError(t, err)

	req.Header.Set("Content-Type", w.FormDataContentType())

	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "failed to process file")
}

// TestHandleInboundEmail has been removed as the endpoint is no longer implemented

// TestValidateTokenMiddleware tests the token validation middleware
func TestValidateTokenMiddleware(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	defer func() {
		if testCtx, ok := ctx.(coreTesting.TestContext); ok {
			testCtx.Teardown()
		}
	}()
	mockTokenService := &MockTokenService{}

	// Setup mock expectations for valid token
	mockTokenService.On("ValidateToken", "valid-token").Return(uint(1), uint(1), true)
	// Setup mock expectations for invalid token
	mockTokenService.On("ValidateToken", "invalid-token").Return(uint(0), uint(0), false)

	// Setup router with mocks
	router := mux.NewRouter()
	adminExt := NewAdminExtension(ctx)

	// Add the token service to context
	testCtx := ctx.(coreTesting.TestContext)
	testCtx.RegisterService(typesSvc.TOKEN_SERVICE, mockTokenService)

	// Create a test handler that will be wrapped by the middleware
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if context values were set correctly
		caseID := service.GetCaseIDFromContext(r.Context())
		reporterID := service.GetReporterIDFromContext(r.Context())

		// Write the IDs to the response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("caseID=%d,reporterID=%d", caseID, reporterID)))
	})

	// Apply the middleware to the test handler
	wrappedHandler := adminExt.validateTokenMiddleware(testHandler)

	// Register the wrapped handler - convert http.Handler to http.HandlerFunc
	router.HandleFunc("/test-middleware", wrappedHandler.ServeHTTP).Methods("GET")

	// Test 1: Valid token in query parameter
	req := createTestRequest(t, "GET", "/test-middleware?token=valid-token", nil)
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "caseID=1,reporterID=1", rr.Body.String())

	// Test 2: Valid token in Authorization header
	req = createTestRequest(t, "GET", "/test-middleware", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "caseID=1,reporterID=1", rr.Body.String())

	// Test 3: Invalid token
	req = createTestRequest(t, "GET", "/test-middleware?token=invalid-token", nil)
	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid or expired token")

	// Test 4: Missing token
	req = createTestRequest(t, "GET", "/test-middleware", nil)
	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "no access token provided")

	// Verify mock was called
	mockTokenService.AssertExpectations(t)
}

// TestValidateToken tests token validation
func TestValidateToken(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	defer func() {
		if testCtx, ok := ctx.(coreTesting.TestContext); ok {
			testCtx.Teardown()
		}
	}()
	mockTokenService := &MockTokenService{}
	mockCaseService := &MockCaseService{}
	mockReporterService := &MockReporterService{}

	// Create test data
	now := time.Now()
	testCase := &models.Case{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}

	testReporter := &models.Reporter{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Email: "test@example.com",
		Name:  "Test Reporter",
	}

	// Setup mock expectations
	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockCaseService.On("GetByID", uint(1)).Return(testCase, nil)
	mockReporterService.On("GetByID", uint(1)).Return(testReporter, nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE:    mockTokenService,
		typesSvc.CASE_SERVICE:     mockCaseService,
		typesSvc.REPORTER_SERVICE: mockReporterService,
	})

	// Create request
	reqBody := ValidateTokenRequest{
		Token: "test-token",
	}
	req := createTestRequest(t, "POST", "/api/abuse/public/validate-token", reqBody)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response map[string]interface{}
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, true, response["valid"])
	assert.Equal(t, float64(1), response["case_id"])
	assert.Equal(t, float64(1), response["reporter_id"])
	assert.Equal(t, "CASE-123456", response["reference"])

	// Verify mock was called
	mockTokenService.AssertExpectations(t)
	mockCaseService.AssertExpectations(t)
	mockReporterService.AssertExpectations(t)

	// Test with invalid token
	mockTokenService = &MockTokenService{}
	mockTokenService.On("ValidateToken", "invalid-token").Return(uint(0), uint(0), false)

	router = configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE: mockTokenService,
	})

	reqBody = ValidateTokenRequest{
		Token: "invalid-token",
	}
	req = createTestRequest(t, "POST", "/api/abuse/public/validate-token", reqBody)

	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid or expired token")

	// Test with case retrieval error
	mockTokenService = &MockTokenService{}
	mockCaseService = &MockCaseService{}

	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockCaseService.On("GetByID", uint(1)).Return(nil, fmt.Errorf("database error"))

	router = configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE: mockTokenService,
		typesSvc.CASE_SERVICE:  mockCaseService,
	})

	reqBody = ValidateTokenRequest{
		Token: "test-token",
	}
	req = createTestRequest(t, "POST", "/api/abuse/public/validate-token", reqBody)

	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "failed to get case")
}

// TestRefreshToken tests token refreshing
func TestRefreshToken(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockTokenService := &MockTokenService{}

	// Setup mock expectations
	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockTokenService.On("GenerateToken", uint(1), uint(1), 90).Return("new-test-token", nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE: mockTokenService,
	})

	// Create request
	reqBody := RefreshTokenRequest{
		Token: "test-token",
	}
	req := createTestRequest(t, "POST", "/api/abuse/public/refresh-token", reqBody)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response map[string]interface{}
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, "new-test-token", response["token"])
	assert.Equal(t, float64(90), response["valid_days"])
	assert.Equal(t, float64(1), response["case_id"])
	assert.Equal(t, float64(1), response["reporter_id"])

	// Verify mock was called
	mockTokenService.AssertExpectations(t)
}

// TestGetPublicCase tests retrieving a case for public access
func TestGetPublicCase(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	mockCaseService := &MockCaseService{}
	mockCommunicationService := &MockCommunicationService{}
	mockScanService := &MockScanService{}
	mockTokenService := &MockTokenService{}

	// Create test data
	now := time.Now()
	testCase := &models.Case{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}

	testCommunications := []models.Communication{
		{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			CaseID:    1,
			SenderID:  1,
			Type:      models.CommunicationTypeEmail,
			Direction: models.CommunicationDirectionOutgoing,
			Content:   "Test email 1",
			ThreadID:  "thread-1",
		},
	}

	testScans := []models.CaseScan{
		{
			Model: gorm.Model{
				ID:        1,
				CreatedAt: now,
				UpdatedAt: now,
			},
			CaseID:       1,
			Hash:         []byte("hash1"),
			Status:       models.ScanStatusClean,
			Type:         "text/plain",
			ScheduledFor: now,
			FileInfo: map[string]interface{}{
				"filename": "file1.txt",
			},
		},
	}

	// Setup mock expectations
	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockCaseService.On("GetByID", uint(1)).Return(testCase, nil)
	mockCommunicationService.On("GetByCaseID", uint(1), mock.Anything).
		Return(testCommunications, int64(1), nil).Maybe()
	mockScanService.On("GetScansForCase", uint(1), mock.Anything).
		Return(testScans, int64(1), nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE:         mockTokenService,
		typesSvc.CASE_SERVICE:          mockCaseService,
		typesSvc.COMMUNICATION_SERVICE: mockCommunicationService,
		typesSvc.SCAN_SERVICE:          mockScanService,
	})

	// Create request
	req := createTestRequest(t, "GET", "/api/abuse/public/cases/CASE-123456?token=test-token", nil)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	var response map[string]interface{}
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, float64(1), response["id"])
	assert.Equal(t, "CASE-123456", response["reference_number"])
	assert.Equal(t, "spam", response["type"])
	assert.Equal(t, "new", response["status"])
	assert.Equal(t, "Test case", response["description"])

	// Verify communications
	communications := response["communications"].([]interface{})
	assert.Len(t, communications, 1)

	// Verify scans
	scans := response["scans"].([]interface{})
	assert.Len(t, scans, 1)

	// Verify mocks were called
	mockTokenService.AssertExpectations(t)
	mockCaseService.AssertExpectations(t)
	mockCommunicationService.AssertExpectations(t)
	mockScanService.AssertExpectations(t)

	// Test error cases

	// Test 1: Reference number mismatch
	mockTokenService = &MockTokenService{}
	mockCaseService = &MockCaseService{}

	differentCase := &models.Case{
		Model: gorm.Model{
			ID: 1,
		},
		ReferenceNumber: "CASE-DIFFERENT",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		ReporterID:      1,
		SubjectID:       1,
	}

	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockCaseService.On("GetByID", uint(1)).Return(differentCase, nil)

	router = configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE: mockTokenService,
		typesSvc.CASE_SERVICE:  mockCaseService,
	})

	req = createTestRequest(t, "GET", "/api/abuse/public/cases/CASE-123456?token=test-token", nil)
	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "case reference mismatch")

	// Test 2: Reporter ID mismatch
	mockTokenService = &MockTokenService{}
	mockCaseService = &MockCaseService{}

	unauthorizedCase := &models.Case{
		Model: gorm.Model{
			ID: 1,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		ReporterID:      2, // Different reporter ID
		SubjectID:       1,
	}

	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockCaseService.On("GetByID", uint(1)).Return(unauthorizedCase, nil)

	router = configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE: mockTokenService,
		typesSvc.CASE_SERVICE:  mockCaseService,
	})

	req = createTestRequest(t, "GET", "/api/abuse/public/cases/CASE-123456?token=test-token", nil)
	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.Contains(t, rr.Body.String(), "unauthorized access")

	// Test 3: Case retrieval error
	mockTokenService = &MockTokenService{}
	mockCaseService = &MockCaseService{}

	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockCaseService.On("GetByID", uint(1)).Return(nil, fmt.Errorf("database error"))

	router = configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE: mockTokenService,
		typesSvc.CASE_SERVICE:  mockCaseService,
	})

	req = createTestRequest(t, "GET", "/api/abuse/public/cases/CASE-123456?token=test-token", nil)
	rr = executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "failed to get case")

	// Test 4: Communications retrieval error
	mockTokenService = &MockTokenService{}
	mockCaseService = &MockCaseService{}
	mockCommunicationService = &MockCommunicationService{}
	mockScanService = &MockScanService{}

	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockCaseService.On("GetByID", uint(1)).Return(testCase, nil)
	mockCommunicationService.On("GetByCaseID", uint(1), mock.Anything).
		Return(nil, int64(0), fmt.Errorf("database error"))
	mockScanService.On("GetScansForCase", uint(1), mock.Anything).
		Return(testScans, int64(1), nil)

	router = configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE:         mockTokenService,
		typesSvc.CASE_SERVICE:          mockCaseService,
		typesSvc.COMMUNICATION_SERVICE: mockCommunicationService,
		typesSvc.SCAN_SERVICE:          mockScanService,
	})

	req = createTestRequest(t, "GET", "/api/abuse/public/cases/CASE-123456?token=test-token", nil)
	rr = executeRequest(router, req)

	// Assert response - should still succeed but with empty communications
	assert.Equal(t, http.StatusOK, rr.Code)

	// Parse response
	parseResponse(t, rr, &response)

	// Verify communications are empty
	communications = response["communications"].([]interface{})
	assert.Len(t, communications, 0)
}

// TestAddPublicComment tests adding a comment to a case from a reporter
func TestAddPublicComment(t *testing.T) {
	// Setup test context and mocks
	ctx := createTestContext(t)
	defer func() {
		if testCtx, ok := ctx.(coreTesting.TestContext); ok {
			testCtx.Teardown()
		}
	}()
	mockCaseService := &MockCaseService{}
	mockCommunicationService := &MockCommunicationService{}
	mockEmailService := &MockEmailService{}
	mockTokenService := &MockTokenService{}

	// Create test data
	now := time.Now()
	testCase := &models.Case{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		ReferenceNumber: "CASE-123456",
		Type:            models.CaseTypeSpam,
		Status:          models.CaseStatusNew,
		Priority:        models.CasePriorityMedium,
		Description:     "Test case",
		Source:          models.ReportSourceWebForm,
		NeedsReview:     true,
		ReporterID:      1,
		SubjectID:       1,
	}

	testCommunication := &models.Communication{
		Model: gorm.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		CaseID:    1,
		SenderID:  1,
		Type:      models.CommunicationTypeResponse,
		Direction: models.CommunicationDirectionExternal,
		Content:   "Test comment",
		ThreadID:  "thread-id",
	}

	// Setup mock expectations
	mockTokenService.On("ValidateToken", "test-token").Return(uint(1), uint(1), true)
	mockCaseService.On("GetByID", uint(1)).Return(testCase, nil)
	mockEmailService.On("GenerateCaseThreadID", uint(1), "CASE-123456").Return("thread-id")
	mockCommunicationService.On("Create", mock.AnythingOfType("*models.Communication")).Return(testCommunication, nil)

	// Setup router with mocks
	router := configureTestRouter(t, ctx, map[string]interface{}{
		typesSvc.TOKEN_SERVICE:         mockTokenService,
		typesSvc.CASE_SERVICE:          mockCaseService,
		typesSvc.EMAIL_SERVICE:         mockEmailService,
		typesSvc.COMMUNICATION_SERVICE: mockCommunicationService,
	})

	// Create request
	reqBody := PublicCommentRequest{
		Content: "Test comment",
	}
	req := createTestRequest(t, "POST", "/api/abuse/public/cases/CASE-123456/comment?token=test-token", reqBody)

	// Execute request
	rr := executeRequest(router, req)

	// Assert response
	assert.Equal(t, http.StatusCreated, rr.Code)

	// Parse response
	var response map[string]interface{}
	parseResponse(t, rr, &response)

	// Verify response
	assert.Equal(t, float64(1), response["id"])
	assert.Equal(t, "response", response["type"])
	assert.Equal(t, "external", response["direction"])
	assert.Equal(t, "Test comment", response["content"])

	// Verify mocks were called
	mockTokenService.AssertExpectations(t)
	mockCaseService.AssertExpectations(t)
	mockEmailService.AssertExpectations(t)
	mockCommunicationService.AssertExpectations(t)
}
