package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	gjwt "github.com/golang-jwt/jwt/v5"
	"go.lumeweb.com/portal-middleware/auth/adapter"
	"go.lumeweb.com/portal-middleware/auth/jwt"
	"go.lumeweb.com/portal-plugin-abuse/internal"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	tjwt "go.lumeweb.com/portal-plugin-abuse/internal/types/jwt"
	"go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.sia.tech/coreutils/wallet"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/service/mocks"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestSubmitReport_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockReportSvc := core.GetService[*mocks.MockAbuseReportService](ctx, service.ABUSE_REPORT_SERVICE)
		assert.NotNil(tb, mockReportSvc)

		mockEmailSvc := core.GetService[*mocks.MockEmailService](ctx, service.EMAIL_SERVICE)
		assert.NotNil(tb, mockEmailSvc)

		mockTokenSvc := core.GetService[*mocks.MockTokenService](ctx, service.TOKEN_SERVICE)
		assert.NotNil(tb, mockTokenSvc)

		mockCase := &models.Case{
			ReferenceNumber: "ABCD1234EFGH",
			Reporter: models.Reporter{
				Email: "user@lumeweb.com",
			},
			Description: "Test description of abusive content",
		}

		expiry := time.Now().Add(90 * 24 * time.Hour)
		mockTokenSvc.EXPECT().GenerateToken(mockCase.ID, mockCase.ReporterID, 90).Return("testtoken", expiry, nil).Once()

		reqBody := dto.AbuseReportRequest{
			Email:       "user@lumeweb.com",
			Location:    "https://example.com/QmSnuWmxptJZdLJpKRarxBMS2Ju2oANVrgbr2xWbie9b2D",
			AbuseType:   "spam",
			Description: "Test description of abusive content",
		}

		mockReportSvc.EXPECT().SubmitReport(mock.Anything, mock.MatchedBy(func(c *models.Case) bool {
			return c.Description == reqBody.Description &&
				c.Subject.SourceURL == reqBody.Location && c.Reporter.Email == reqBody.Email
		})).Return(mockCase, nil).Once()

		body, err := json.Marshal(reqBody)
		assert.NoError(tb, err)

		req := ctx.NewAPIRequest(http.MethodPost, "/api/reports", body)
		w := httptest.NewRecorder()

		// Act
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusCreated, w.Code)

		var response dto.AbuseReportResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, "CASE-ABCD1234EFGH", response.CaseReference)
	}, coreTesting.WithAPIID(internal.PLUGIN_NAME))
}

func TestSubmitReport_ValidationFailure(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		reqBody := `{"email": "invalid-email", "location": "not-a-url"}`
		req := ctx.NewAPIRequest(http.MethodPost, "/api/reports", bytes.NewBufferString(reqBody).Bytes())
		w := httptest.NewRecorder()

		// Act
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusUnprocessableEntity, w.Code)
	}, coreTesting.WithAPIID(internal.PLUGIN_NAME))
}

func TestGetAbuseCase_NotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, service.CASE_SERVICE)
		assert.NotNil(tb, mockCaseSvc)

		// Create valid JWT token
		seed := wallet.NewSeedPhrase()
		if err := ctx.Config().Update("core.identity", seed); err != nil {
			tb.Error(err)
		}
		if err := ctx.Config().Update("core.domain", "example.com"); err != nil {
			tb.Error(err)
		}

		pk := ctx.Config().Config().Core.Identity.PrivateKey()
		jwtToken, err := jwt.CreateToken(pk, "example.com", "2", jwt.PurposeLogin, 90*24*time.Hour,
			jwt.WithClaims(tjwt.NewAbuseJWTClaims(1, 2)),
		)
		assert.NoError(tb, err)

		// Mock expectations - use uint(0) since that's what the JWT claims will have
		mockCaseSvc.EXPECT().GetByID(uint(1)).Return(nil, db.ErrRecordNotFound)

		// Create request with valid JWT
		req := ctx.NewAPIRequest(http.MethodGet, "/api/cases/ABCD1234", nil)
		req.Header.Set("Authorization", "Bearer "+jwtToken)
		w := httptest.NewRecorder()

		// Act
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusNotFound, w.Code)
	}, coreTesting.WithAPIID(internal.PLUGIN_NAME))
}

func TestGetCase_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, service.CASE_SERVICE)
		assert.NotNil(tb, mockCaseSvc)

		mockCommSvc := core.GetService[*mocks.MockCommunicationService](ctx, service.COMMUNICATION_SERVICE)
		assert.NotNil(tb, mockCommSvc)

		mockScanSvc := core.GetService[*mocks.MockScanService](ctx, service.SCAN_SERVICE)
		assert.NotNil(tb, mockScanSvc)

		mockEvidenceSvc := core.GetService[*mocks.MockEvidenceService](ctx, service.EVIDENCE_SERVICE)
		assert.NotNil(tb, mockEvidenceSvc)

		// Mock case data
		mockCase := &models.Case{
			Model:           gorm.Model{ID: 1, CreatedAt: time.Now(), UpdatedAt: time.Now()},
			ReferenceNumber: "ABCD1234",
			Status:          models.CaseStatusInProgress,
			Description:     "Test case",
			ReporterID:      2,
			SubjectID:       3,
		}

		// Create valid JWT token
		seed := wallet.NewSeedPhrase()
		if err := ctx.Config().Update("core.identity", seed); err != nil {
			tb.Error(err)
		}
		if err := ctx.Config().Update("core.domain", "example.com"); err != nil {
			tb.Error(err)
		}

		pk := ctx.Config().Config().Core.Identity.PrivateKey()
		jwtToken, err := jwt.CreateToken(pk, "example.com", "2", jwt.PurposeLogin, 90*24*time.Hour,
			jwt.WithClaims(tjwt.NewAbuseJWTClaims(1, 2)),
		)
		assert.NoError(tb, err)

		// Mock expectations
		mockCaseSvc.EXPECT().GetByID(uint(1)).Return(mockCase, nil)

		mockCommSvc.EXPECT().ListByCaseID(uint(1),
			mock.AnythingOfType("[]filter.CrudFilter"),
			mock.AnythingOfType("[]filter.Sort"),
			mock.AnythingOfType("filter.Pagination"),
		).Return([]models.Communication{}, int64(0), nil)

		mockScanSvc.EXPECT().GetScansForCase(uint(1),
			mock.AnythingOfType("filter.Pagination"),
		).Return([]models.CaseScan{}, int64(0), nil)

		mockEvidenceSvc.EXPECT().GetByCaseID(uint(1),
			mock.AnythingOfType("filter.Pagination"),
		).Return([]models.Evidence{}, int64(0), nil)

		// Create request with valid JWT
		req := ctx.NewAPIRequest(http.MethodGet, "/api/cases/ABCD1234", nil)
		req.Header.Set("Authorization", "Bearer "+jwtToken)

		w := httptest.NewRecorder()

		// Act
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response dto.PublicCaseResponse
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)

		// Check public fields
		assert.Equal(tb, "ABCD1234", response.ReferenceNumber)
		assert.Equal(tb, "in_progress", response.Status)
		assert.Equal(tb, "Test case", response.Description)
		assert.False(tb, response.CreatedAt.IsZero())
		assert.False(tb, response.UpdatedAt.IsZero())

		// Ensure internal fields are not exposed
		assert.Empty(tb, response.Communications)
		assert.Empty(tb, response.Scans)
	}, coreTesting.WithAPIID(internal.PLUGIN_NAME))
}

func TestValidateToken_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockTokenSvc := core.GetService[*mocks.MockTokenService](ctx, service.TOKEN_SERVICE)
		assert.NotNil(tb, mockTokenSvc)

		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, service.CASE_SERVICE)
		assert.NotNil(tb, mockCaseSvc)

		mockReporterSvc := core.GetService[*mocks.MockReporterService](ctx, service.REPORTER_SERVICE)
		assert.NotNil(tb, mockReporterSvc)

		// Mock valid token
		validToken := "validtesttoken"
		caseID := uint(1)
		reporterID := uint(2)
		referenceNumber := "ABCD1234"

		// Setup mock expectations
		mockTokenSvc.EXPECT().ValidateToken(validToken).Return(caseID, reporterID, true)
		mockCaseSvc.EXPECT().GetByID(caseID).Return(&models.Case{
			Model:           gorm.Model{ID: caseID},
			ReferenceNumber: referenceNumber,
		}, nil)
		mockReporterSvc.EXPECT().GetByID(reporterID).Return(&models.Reporter{Model: gorm.Model{ID: reporterID}}, nil)

		// Create request
		reqBody := `{"token": "validtesttoken"}`
		req := ctx.NewAPIRequest(http.MethodPost, "/api/tokens/validate", bytes.NewBufferString(reqBody).Bytes())
		w := httptest.NewRecorder()

		// Act
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)

		assert.Equal(tb, true, response["valid"])
		assert.Equal(tb, referenceNumber, response["reference"])
	}, coreTesting.WithAPIID(internal.PLUGIN_NAME))
}

func TestValidateToken_Invalid(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockTokenSvc := core.GetService[*mocks.MockTokenService](ctx, service.TOKEN_SERVICE)
		assert.NotNil(tb, mockTokenSvc)

		mockTokenSvc.EXPECT().ValidateToken("invalidtoken").Return(uint(0), uint(0), false)

		reqBody := `{"token": "invalidtoken"}`
		req := ctx.NewAPIRequest(http.MethodPost, "/api/tokens/validate", bytes.NewBufferString(reqBody).Bytes())
		w := httptest.NewRecorder()

		// Act
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusUnauthorized, w.Code)
	}, coreTesting.WithAPIID(internal.PLUGIN_NAME))
}

func TestExchangeToken_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockTokenSvc := core.GetService[*mocks.MockTokenService](ctx, service.TOKEN_SERVICE)
		assert.NotNil(tb, mockTokenSvc)

		mockTokenSvc.EXPECT().ValidateToken("validtoken").Return(uint(1), uint(2), true)

		reqBody := `{"token": "validtoken"}`
		req := ctx.NewAPIRequest(http.MethodPost, "/api/tokens/jwt", bytes.NewBufferString(reqBody).Bytes())
		w := httptest.NewRecorder()

		// Act
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)
		var response dto.JWTResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.NotEmpty(tb, response.AccessToken)

		// Decode the JWT using middleware helper
		claims, err := jwt.DecodeToken(response.AccessToken, &tjwt.AbuseJWTClaims{})
		assert.NoError(tb, err)

		abuseClaims, ok := claims.(*tjwt.AbuseJWTClaims)
		assert.True(tb, ok)

		// Validate claims
		assert.Equal(tb, uint(1), abuseClaims.CaseID)
		assert.Equal(tb, uint(2), abuseClaims.ReporterID)
		assert.Contains(tb, abuseClaims.Audience, string(jwt.PurposeLogin))
		assert.Equal(tb, "example.com", abuseClaims.Issuer)
		assert.NotZero(tb, abuseClaims.ExpiresAt)
	}, coreTesting.WithAPIID(internal.PLUGIN_NAME))
}

func TestRefreshToken_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockTokenSvc := core.GetService[*mocks.MockTokenService](ctx, service.TOKEN_SERVICE)
		assert.NotNil(tb, mockTokenSvc)

		mockTokenSvc.EXPECT().ValidateToken("validtoken").Return(uint(1), uint(2), true)

		reqBody := `{"token": "validtoken"}`
		w := httptest.NewRecorder()
		req := ctx.NewAPIRequest(http.MethodPost, "/api/tokens/refresh", bytes.NewBufferString(reqBody).Bytes())

		// Use real cookie setter to generate JWT
		configProvider := adapter.NewFromCore(ctx)
		cookieSetter := adapter.NewCookieSetter(configProvider)

		seed := wallet.NewSeedPhrase()
		if err := ctx.Config().Update("core.identity", seed); err != nil {
			tb.Error(err)
		}
		if err := ctx.Config().Update("core.domain", "example.com"); err != nil {
			tb.Error(err)
		}

		tokenString, err := cookieSetter.SetJWTCookie(w, fmt.Sprintf("%d", uint(2)), jwt.PurposeLogin, 90*24*time.Hour,
			jwt.WithClaims(&tjwt.AbuseJWTClaims{
				RegisteredClaims: &gjwt.RegisteredClaims{
					Issuer:   "example.com",
					Audience: []string{string(jwt.PurposeLogin)},
				},
			}),
			jwt.WithModifiers(jwt.ClaimModifier(func(claims gjwt.Claims) {
				if abuseClaims, ok := claims.(*tjwt.AbuseJWTClaims); ok {
					abuseClaims.CaseID = 1
					abuseClaims.ReporterID = 2
				}
			})),
		)
		assert.NoError(tb, err)

		// Set the generated token in the request header
		req.Header.Set("Authorization", "Bearer "+tokenString)

		// Act
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusOK, w.Code)
		var response map[string]interface{}
		err = json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(tb, err)
		assert.Equal(tb, tokenString, response["token"])
		assert.Equal(tb, float64(90), response["valid_days"])

		// Decode the JWT using middleware helper
		claims, err := jwt.DecodeToken(tokenString, &tjwt.AbuseJWTClaims{})
		assert.NoError(tb, err)

		abuseClaims, ok := claims.(*tjwt.AbuseJWTClaims)
		assert.True(tb, ok)

		// Validate claims
		assert.Equal(tb, uint(1), abuseClaims.CaseID)
		assert.Equal(tb, uint(2), abuseClaims.ReporterID)
		assert.Contains(tb, abuseClaims.Audience, string(jwt.PurposeLogin))
		assert.Equal(tb, "example.com", abuseClaims.Issuer)
		assert.NotZero(tb, abuseClaims.ExpiresAt)
	}, coreTesting.WithAPIID(internal.PLUGIN_NAME))
}

func TestGetCase_AuthorizationFailure(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		mockCaseSvc := core.GetService[*mocks.MockCaseService](ctx, service.CASE_SERVICE)
		assert.NotNil(tb, mockCaseSvc)

		// Create invalid JWT token
		req := ctx.NewAPIRequest(http.MethodGet, "/api/cases/ABCD1234", nil)
		req.Header.Set("Authorization", "Bearer invalidtoken")
		w := httptest.NewRecorder()

		// Act
		ctx.Router().ServeHTTP(w, req)

		// Assert
		assert.Equal(tb, http.StatusUnauthorized, w.Code)
		mockCaseSvc.AssertNotCalled(tb, "GetByID")
	}, coreTesting.WithAPIID(internal.PLUGIN_NAME))
}
