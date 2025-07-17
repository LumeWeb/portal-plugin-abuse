package api

import (
	"bytes"
	"context"
	"fmt"
	gjwt "github.com/golang-jwt/jwt/v5"
	"go.lumeweb.com/gswagger"
	"go.lumeweb.com/portal-middleware/middleware"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"go.lumeweb.com/queryutil"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-middleware/auth"
	"go.lumeweb.com/portal-middleware/auth/adapter"
	"go.lumeweb.com/portal-middleware/auth/jwt"
	"go.lumeweb.com/portal-middleware/cors"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	pluginConfig "go.lumeweb.com/portal-plugin-abuse/internal/config"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	pjwt "go.lumeweb.com/portal-plugin-abuse/internal/types/jwt"
	tjwt "go.lumeweb.com/portal-plugin-abuse/internal/types/jwt"
	svcTypes "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal-router"
	"go.lumeweb.com/portal/config"
	"go.lumeweb.com/portal/core"
	portal_abuse_report "go.lumeweb.com/web/go/portal-abuse-report"
	"go.uber.org/zap"
)

type contextKey string

const (
	contextKeyCaseID     contextKey = "caseID"
	contextKeyReporterID contextKey = "reporterID"
)

// AbuseAPI handles abuse report endpoints
type AbuseAPI struct {
	ctx                core.Context
	logger             *core.Logger
	abuseReportService svcTypes.AbuseReportService
	emailService       svcTypes.EmailService
	tokenSvc           svcTypes.TokenService
}

// caseToResponseModel converts a Case model to a AbuseReportResponseModel with auth token and expiration
func caseToResponseModel(caseModel *models.Case, token string, expiresAt time.Time) *dto.AbuseReportResponseModel {
	return &dto.AbuseReportResponseModel{
		Case:        caseModel,
		AccessToken: token,
		ExpiresAt:   expiresAt,
	}
}

// NewAbuseAPI creates a new instance of AbuseAPI
func NewAbuseAPI() (core.API, []core.ContextBuilderOption, error) {
	api := &AbuseAPI{}

	options := []core.ContextBuilderOption{
		core.ContextWithStartupFunc(func(ctx core.Context) error {
			api.ctx = ctx
			api.logger = ctx.NamedLogger("abuse.api")

			// Get required services using the generic GetService function
			api.abuseReportService = core.GetService[svcTypes.AbuseReportService](ctx, svcTypes.ABUSE_REPORT_SERVICE)
			if api.abuseReportService == nil {
				return fmt.Errorf("abuse report service not available")
			}

			api.emailService = core.GetService[svcTypes.EmailService](ctx, svcTypes.EMAIL_SERVICE)
			if api.emailService == nil {
				return fmt.Errorf("email service not available")
			}

			api.tokenSvc = core.GetService[svcTypes.TokenService](ctx, svcTypes.TOKEN_SERVICE)
			if api.tokenSvc == nil {
				return fmt.Errorf("token service not available")
			}

			return nil
		}),
	}

	return api, options, nil
}

// Name returns the API name
func (a *AbuseAPI) Name() string {
	return "abuse"
}

// Subdomain returns the API subdomain
func (a *AbuseAPI) Subdomain() string {
	return "abuse"
}

// OpenAPIInfo returns the OpenAPI information for this API
func (a *AbuseAPI) OpenAPIInfo() router.APIInfoDefinition {
	return router.APIInfo().
		Title("Abuse Report API").
		Description("Public API for user submission and tracking of abuse reports.")
}

// Configure sets up the API routes
func (a *AbuseAPI) Configure(r router.Router, accessSvc core.AccessService) error {
	// Setup static file serving
	router.MustDefaultStaticSetup(r, portal_abuse_report.GetFS())

	// Create an API group
	apiGroup, err := r.Group("/api")
	if err != nil {
		return fmt.Errorf("failed to create api group: %w", err)
	}

	// Apply common middleware to the API group
	apiGroup.Use(
		echo.WrapMiddleware(cors.NewWithDefaults(cors.Config{})),
	)

	// Define routes using portal-router.RouteDefinition
	routes := router.DefineRoutes(
		// Public Endpoints
		router.NewRoute(http.MethodPost, "/reports", a.submitReport,
			router.WithSwagger(
				router.WithSummary("Submit a new abuse report"),
				router.WithDescription("Allows users to submit a new abuse report with details and attachments."),
				router.WithRequestBody(&dto.AbuseReportRequest{}, "Abuse report details", true),
				router.WithSuccessResponse(http.StatusCreated, "Report submitted successfully",
					router.WithJSONContent(&dto.AbuseReportResponse{}),
				),

				router.WithErrorResponses(router.DefineSwaggerErrorResponses(
					router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid request payload"),
					router.DefineSwaggerErrorResponse(http.StatusInternalServerError, "Internal server error"),
				)),
			),
		),

		// Token Management
		router.NewRoute(http.MethodPost, "/tokens/validate", a.validateToken,
			router.WithSwagger(
				router.WithSummary("Validate an abuse report access token"),
				router.WithDescription("Checks if a given access token is valid and returns the associated case reference."),
				router.WithRequestBody(&dto.TokenRefreshRequest{}, "Token validation request", true),
				router.WithSuccessResponse(http.StatusOK, "Token is valid",
					router.WithJSONContent(&dto.ValidateTokenResponse{}),
				),
				router.WithErrorResponses(router.DefineSwaggerErrorResponses(
					router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid request payload"),
					router.DefineSwaggerErrorResponse(http.StatusUnauthorized, "Invalid or expired token"),
					router.DefineSwaggerErrorResponse(http.StatusInternalServerError, "Internal server error"),
				)),
			),
		),
		router.NewRoute(http.MethodPost, "/tokens/refresh", a.refreshToken,
			router.WithSwagger(
				router.WithSummary("Refresh an abuse report access token"),
				router.WithDescription("Refreshes an existing access token, returning a new token with extended validity."),
				router.WithRequestBody(&dto.TokenRefreshRequest{}, "Token refresh request", true),
				router.WithSuccessResponse(http.StatusOK, "Token refreshed successfully",
					router.WithJSONContent(&dto.TokenRefreshResponse{}),
				),
				router.WithErrorResponses(router.DefineSwaggerErrorResponses(
					router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid request payload"),
					router.DefineSwaggerErrorResponse(http.StatusUnauthorized, "Invalid access token"),
					router.DefineSwaggerErrorResponse(http.StatusInternalServerError, "Internal server error"),
				)),
			),
		),
		router.NewRoute(http.MethodPost, "/tokens/jwt", a.exchangeToken,
			router.WithSwagger(
				router.WithSummary("Exchange access token for JWT"),
				router.WithDescription("Exchanges a valid abuse report access token for a JWT, setting an authentication cookie."),
				router.WithRequestBody(&dto.TokenRefreshRequest{}, "Token exchange request", true),
				router.WithSuccessResponse(http.StatusOK, "JWT generated and cookie set",
					router.WithJSONContent(&dto.JWTResponse{}),
				),
				router.WithErrorResponses(router.DefineSwaggerErrorResponses(
					router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid request payload"),
					router.DefineSwaggerErrorResponse(http.StatusUnauthorized, "Invalid access token"),
					router.DefineSwaggerErrorResponse(http.StatusInternalServerError, "Internal server error"),
				)),
			),
		),
	)

	// Register public routes
	if err := router.RegisterRoutes(apiGroup, accessSvc, a.Subdomain(), routes, router.WithCors()); err != nil {
		return fmt.Errorf("failed to register public routes: %w", err)
	}

	// JWT-protected endpoints group
	protectedGroup, err := apiGroup.Group("/cases")
	if err != nil {
		return fmt.Errorf("failed to create protected group: %w", err)
	}

	protectedRoutes := router.DefineRoutes(
		router.NewRoute(http.MethodGet, "/:reference", a.getCase,
			router.WithSwagger(
				router.WithSummary("Get abuse case details"),
				router.WithDescription("Retrieves the details of a specific abuse case, including communications, scans, and evidence."),
				router.WithPathParam("reference", "The case reference number", "string"),
				router.WithSuccessResponse(http.StatusOK, "Case details retrieved successfully",
					router.WithJSONContent(&dto.PublicCaseResponse{}),
				),
				router.WithErrorResponses(make(map[int]swagger.ContentValue)),
			),
		),
		router.NewRoute(http.MethodPost, "/:reference/communications", a.addComment,
			router.WithSwagger(
				router.WithSummary("Add a comment to an abuse case"),
				router.WithDescription("Adds a new communication (comment) to a specific abuse case."),
				router.WithPathParam("reference", "The case reference number", "string"),
				router.WithRequestBody(struct{ Content string }{}, "Comment content", true),
				router.WithSuccessResponse(http.StatusCreated, "Comment added successfully",
					router.WithJSONContent(&dto.CommunicationResponse{}),
				),
			),
		),
		router.NewRoute(http.MethodPost, "/:reference/attachments", a.uploadFile,
			router.WithSwagger(
				router.WithSummary("Upload an attachment to an abuse case"),
				router.WithDescription("Uploads a file attachment as evidence for a specific abuse case."),
				router.WithPathParam("reference", "The case reference number", "string"),
				// Use WithFileUpload instead of WithRequestBody
				router.WithFileUpload("File upload", true),
				router.WithSuccessResponse(http.StatusCreated, "File uploaded successfully",
					router.WithJSONContent(&dto.AttachmentUploadResponse{}),
				),
				// Add default error responses if not already covered by WithSwagger defaults
				router.WithErrorResponses(make(map[int]swagger.ContentValue)), // Or specific errors if needed
			),
		),
	)

	// Register protected routes
	if err := router.RegisterRoutes(protectedGroup, accessSvc, a.Subdomain(), protectedRoutes, router.WithMiddlewares(middleware.AuthMiddleware(
		a.ctx,
		jwt.PurposeLogin,
		middleware.WithAuthEmptyAllowed(false),
		middleware.WithAuthJWTOptions(jwt.WithClaims(pjwt.NewAbuseJWTClaims(0, 0))),
	),
		caseIDMiddleware()), router.WithCors()); err != nil {
		return fmt.Errorf("failed to register protected routes: %w", err)
	}

	return nil
}

// AuthTokenName returns the name of the token used for this API
func (a *AbuseAPI) AuthTokenName() string {
	return "abuse_token"
}

// Config returns the API configuration
func (a *AbuseAPI) Config() config.APIConfig {
	return &pluginConfig.APIConfig{
		Enabled: true,
	}
}

// submitReport handles the submission of a new abuse report
func (a *AbuseAPI) submitReport(c echo.Context) error {
	ctx := httputil.Context(c)

	var requestDto dto.AbuseReportRequest

	caseModel, ok := httputil.DecodeAndValidateRequest[*models.Case, *dto.AbuseReportRequest](ctx, &requestDto)
	if !ok {
		return nil // Error handled by DecodeAndValidateRequest
	}

	caseModel, err := a.abuseReportService.SubmitReport(c.Request().Context(), caseModel)
	if err != nil {
		a.logger.Error("Failed to submit report", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to submit report: %w", err), http.StatusInternalServerError)
	}

	token, expiresAt, err := a.tokenSvc.GenerateToken(caseModel.ID, caseModel.ReporterID, 90)
	if err != nil {
		a.logger.Error("Failed to generate access token", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to generate access credentials: %w", err), http.StatusInternalServerError)
	}

	responseDtoModel := caseToResponseModel(caseModel, token, expiresAt)

	var responseDto dto.AbuseReportResponse

	c.Response().WriteHeader(http.StatusCreated)
	if err := httputil.EncodeResponse[*dto.AbuseReportResponseModel](ctx, responseDtoModel, &responseDto); err != nil {
		a.logger.Error("Failed to encode response", zap.Error(err))
		return err // httputil.EncodeResponse returns an error
	}

	return nil
}

// getCase handles GET /cases/{reference}
func (a *AbuseAPI) getCase(c echo.Context) error {
	ctx := httputil.Context(c)
	reference := c.Param("reference")

	caseID, _ := c.Request().Context().Value(contextKeyCaseID).(uint)
	reporterID, _ := c.Request().Context().Value(contextKeyReporterID).(uint)

	caseService := core.GetService[svcTypes.CaseService](a.ctx, svcTypes.CASE_SERVICE)
	if caseService == nil {
		return ctx.Error(fmt.Errorf("case service not available"), http.StatusInternalServerError)
	}

	caseModel, err := caseService.GetByID(caseID)
	if err != nil {
		a.logger.Error("Failed to get case", zap.Error(err))
		if err == db.ErrRecordNotFound {
			return ctx.Error(fmt.Errorf("case not found"), http.StatusNotFound)
		}
		return ctx.Error(fmt.Errorf("failed to get case: %w", err), http.StatusInternalServerError)
	}

	if caseModel.ReferenceNumber != reference {
		return ctx.Error(fmt.Errorf("case reference mismatch"), http.StatusForbidden)
	}

	if caseModel.ReporterID != reporterID {
		return ctx.Error(fmt.Errorf("unauthorized access"), http.StatusForbidden)
	}

	commService := core.GetService[svcTypes.CommunicationService](a.ctx, svcTypes.COMMUNICATION_SERVICE)
	if commService == nil {
		a.logger.Error("Communication service not available")
		// Continue without communications if service is missing, or return error?
		// Returning error for now for clarity.
		return ctx.Error(fmt.Errorf("communication service not available"), http.StatusInternalServerError)
	}

	comms, _, err := commService.ListByCaseID(caseID, []queryutil.CrudFilter{
		queryutil.Or(
			queryutil.StringField("direction").Eq(string(models.CommunicationDirectionOutgoing)),
			queryutil.StringField("direction").Eq(string(models.CommunicationDirectionExternal)),
		),
	}, nil, queryutil.Pagination{})
	if err != nil {
		a.logger.Error("Failed to get communications", zap.Error(err))
		// Decide if this is a fatal error or if we can return the case without comms
		// Returning error for now.
		return ctx.Error(fmt.Errorf("failed to get communications: %w", err), http.StatusInternalServerError)
	}

	scanService := core.GetService[svcTypes.ScanService](a.ctx, svcTypes.SCAN_SERVICE)
	if scanService == nil {
		a.logger.Error("Scan service not available")
		// Decide if this is a fatal error
		return ctx.Error(fmt.Errorf("scan service not available"), http.StatusInternalServerError)
	}
	scans, _, err := scanService.GetScansForCase(caseID, queryutil.Pagination{})
	if err != nil {
		a.logger.Error("Failed to get scans", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to get scans: %w", err), http.StatusInternalServerError)
	}

	evidenceService := core.GetService[svcTypes.EvidenceService](a.ctx, svcTypes.EVIDENCE_SERVICE)
	if evidenceService == nil {
		a.logger.Error("Evidence service not available")
		return ctx.Error(fmt.Errorf("evidence service not available"), http.StatusInternalServerError)
	}
	evidences, _, err := evidenceService.GetByCaseID(caseID, queryutil.Pagination{})
	if err != nil {
		a.logger.Error("Failed to get evidence", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to get evidence: %w", err), http.StatusInternalServerError)
	}

	caseModel.Communications = comms
	caseModel.CaseScans = scans
	caseModel.Evidence = evidences

	var response dto.PublicCaseResponse
	if err := response.FromModel(caseModel); err != nil {
		a.logger.Error("Failed to convert case model", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to process case data: %w", err), http.StatusInternalServerError)
	}

	if err := httputil.EncodeResponse[*models.Case](ctx, caseModel, &response); err != nil {
		a.logger.Error("Failed to encode response", zap.Error(err))
		return err
	}

	return nil
}

// addComment handles POST /cases/{reference}/comment
func (a *AbuseAPI) addComment(c echo.Context) error {
	ctx := httputil.Context(c)
	reference := c.Param("reference")

	caseID, _ := c.Request().Context().Value(contextKeyCaseID).(uint)
	reporterID, _ := c.Request().Context().Value(contextKeyReporterID).(uint)

	var req struct {
		Content string `json:"content"`
	}
	if err := ctx.Decode(&req); err != nil {
		return nil // Error handled by ctx.Decode
	}

	caseService := core.GetService[svcTypes.CaseService](a.ctx, svcTypes.CASE_SERVICE)
	if caseService == nil {
		return ctx.Error(fmt.Errorf("case service not available"), http.StatusInternalServerError)
	}
	caseModel, err := caseService.GetByID(caseID)
	if err != nil {
		a.logger.Error("Failed to get case", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to get case: %w", err), http.StatusInternalServerError)
	}

	if caseModel.ReferenceNumber != reference {
		return ctx.Error(fmt.Errorf("case reference mismatch"), http.StatusForbidden)
	}

	if caseModel.ReporterID != reporterID {
		return ctx.Error(fmt.Errorf("unauthorized access"), http.StatusForbidden)
	}

	comm := &models.Communication{
		CaseID:    caseID,
		SenderID:  reporterID,
		Type:      models.CommunicationTypeResponse,
		Direction: models.CommunicationDirectionExternal,
		Content:   req.Content,
		ThreadID:  a.emailService.GenerateCaseThreadID(caseModel.ReferenceNumber),
	}

	commService := core.GetService[svcTypes.CommunicationService](a.ctx, svcTypes.COMMUNICATION_SERVICE)
	if commService == nil {
		return ctx.Error(fmt.Errorf("communication service not available"), http.StatusInternalServerError)
	}
	created, err := commService.Create(comm)
	if err != nil {
		a.logger.Error("Failed to create communication", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to create communication: %w", err), http.StatusInternalServerError)
	}

	var responseDto dto.CommunicationResponse
	if err := responseDto.FromModel(created); err != nil {
		a.logger.Error("Failed to convert communication model", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to process communication data: %w", err), http.StatusInternalServerError)
	}

	c.Response().WriteHeader(http.StatusCreated)
	if err := httputil.EncodeResponse[*models.Communication, *dto.CommunicationResponse](ctx, created, &responseDto); err != nil {
		a.logger.Error("Failed to encode response", zap.Error(err))
		return err
	}

	return nil
}

// uploadFile handles POST /cases/{reference}/upload
func (a *AbuseAPI) uploadFile(c echo.Context) error {
	ctx := httputil.Context(c)
	reference := c.Param("reference")

	caseID, _ := c.Request().Context().Value(contextKeyCaseID).(uint)
	reporterID, _ := c.Request().Context().Value(contextKeyReporterID).(uint)

	caseService := core.GetService[svcTypes.CaseService](a.ctx, svcTypes.CASE_SERVICE)
	if caseService == nil {
		return ctx.Error(fmt.Errorf("case service not available"), http.StatusInternalServerError)
	}
	caseModel, err := caseService.GetByID(caseID)
	if err != nil {
		a.logger.Error("Failed to get case", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to get case: %w", err), http.StatusInternalServerError)
	}

	if caseModel.ReferenceNumber != reference {
		return ctx.Error(fmt.Errorf("case reference mismatch"), http.StatusForbidden)
	}

	if caseModel.ReporterID != reporterID {
		return ctx.Error(fmt.Errorf("unauthorized access"), http.StatusForbidden)
	}

	file, header, err := c.Request().FormFile("file")
	if err != nil {
		return ctx.Error(fmt.Errorf("failed to get file: %w", err), http.StatusBadRequest)
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			a.logger.Error("Failed to close file", zap.Error(err))
		}
	}(file)

	fileData, err := io.ReadAll(file)
	if err != nil {
		return ctx.Error(fmt.Errorf("failed to read file: %w", err), http.StatusInternalServerError)
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(fileData)
	}

	evidence := &models.Evidence{
		CaseID:      caseID,
		SubmitterID: reporterID,
		FileName:    header.Filename,
		ContentType: contentType,
		FileSize:    int64(len(fileData)),
		Source:      models.EvidenceSourceWebUpload,
		Description: "File uploaded via web interface",
	}

	evidenceService := core.GetService[svcTypes.EvidenceService](a.ctx, svcTypes.EVIDENCE_SERVICE)
	if evidenceService == nil {
		return ctx.Error(fmt.Errorf("evidence service not available"), http.StatusInternalServerError)
	}

	_, err = evidenceService.CreateFromData(io.NopCloser(bytes.NewReader(fileData)), evidence)
	if err != nil {
		a.logger.Error("Failed to create evidence", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to store evidence: %w", err), http.StatusInternalServerError)
	}

	responseDto := &dto.AttachmentUploadResponse{
		Message:     "File uploaded successfully",
		FileName:    header.Filename,
		Size:        int64(len(fileData)),
		ContentType: contentType,
	}

	comm := &models.Communication{
		CaseID:    caseID,
		SenderID:  reporterID,
		Type:      models.CommunicationTypeNote,
		Direction: models.CommunicationDirectionExternal,
		Content:   fmt.Sprintf("Evidence uploaded: %s (%.2f MB)", header.Filename, float64(len(fileData))/1024/1024),
		ThreadID:  a.emailService.GenerateCaseThreadID(caseModel.ReferenceNumber),
	}

	commService := core.GetService[svcTypes.CommunicationService](a.ctx, svcTypes.COMMUNICATION_SERVICE)
	if commService == nil {
		return ctx.Error(fmt.Errorf("communication service not available"), http.StatusInternalServerError)
	}

	_, err = commService.Create(comm)
	if err != nil {
		a.logger.Error("Failed to create communication for evidence upload", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to log file upload: %w", err), http.StatusInternalServerError)
	}

	c.Response().WriteHeader(http.StatusCreated)
	if err := httputil.EncodeResponse[any](ctx, nil, responseDto); err != nil {
		a.logger.Error("Failed to encode attachment upload response", zap.Error(err))
		return err
	}

	return nil
}

// validateToken handles POST /tokens/validate
func (a *AbuseAPI) validateToken(c echo.Context) error {
	ctx := httputil.Context(c)

	var reqDto dto.TokenRefreshRequest
	_, ok := httputil.DecodeAndValidateRequest[*models.Token, *dto.TokenRefreshRequest](ctx, &reqDto)
	if !ok {
		return nil // Error handled by DecodeAndValidateRequest
	}

	caseID, reporterID, valid := a.tokenSvc.ValidateToken(reqDto.Token)
	if !valid {
		return ctx.Error(fmt.Errorf("invalid or expired token"), http.StatusUnauthorized)
	}

	caseService := core.GetService[svcTypes.CaseService](a.ctx, svcTypes.CASE_SERVICE)
	if caseService == nil {
		return ctx.Error(fmt.Errorf("case service not available"), http.StatusInternalServerError)
	}

	caseModel, err := caseService.GetByID(caseID)
	if err != nil {
		a.logger.Error("Failed to get case", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to get case: %w", err), http.StatusInternalServerError)
	}

	reporterService := core.GetService[svcTypes.ReporterService](a.ctx, svcTypes.REPORTER_SERVICE)
	if reporterService == nil {
		return ctx.Error(fmt.Errorf("reporter service not available"), http.StatusInternalServerError)
	}

	if _, err := reporterService.GetByID(reporterID); err != nil {
		a.logger.Error("Failed to get reporter", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to get reporter: %w", err), http.StatusInternalServerError)
	}

	responseModel := &dto.ValidateTokenResponse{
		Valid:     true,
		Reference: caseModel.ReferenceNumber,
	}
	var responseDto dto.ValidateTokenResponse
	if err := httputil.EncodeResponse(ctx, responseModel, &responseDto); err != nil {
		a.logger.Error("Failed to encode validation response", zap.Error(err))
		return err
	}

	return nil
}

// refreshToken handles POST /tokens/refresh
func (a *AbuseAPI) refreshToken(c echo.Context) error {
	ctx := httputil.Context(c)

	var reqDto dto.TokenRefreshRequest
	_, ok := httputil.DecodeAndValidateRequest[*models.Token, *dto.TokenRefreshRequest](ctx, &reqDto)
	if !ok {
		return nil // Error handled by DecodeAndValidateRequest
	}

	caseID, reporterID, valid := a.tokenSvc.ValidateToken(reqDto.Token)
	if !valid {
		return ctx.Error(fmt.Errorf("invalid access token"), http.StatusUnauthorized)
	}

	configProvider := adapter.NewFromCore(a.ctx)
	cookieSetter := adapter.NewCookieSetter(configProvider)

	newToken, err := cookieSetter.SetJWTCookie(c.Response(), fmt.Sprintf("%d", reporterID), jwt.PurposeLogin, 90*24*time.Hour,
		jwt.WithClaims(tjwt.NewAbuseJWTClaims(caseID, reporterID)),
		jwt.WithModifiers(jwt.ClaimModifier(func(claims gjwt.Claims) {
			if abuseClaims, ok := claims.(*tjwt.AbuseJWTClaims); ok {
				abuseClaims.CaseID = caseID
				abuseClaims.ReporterID = reporterID
			}
		})),
	)
	if err != nil {
		a.logger.Error("Failed to set JWT cookie", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to set authentication cookie: %w", err), http.StatusInternalServerError)
	}

	responseModel := &dto.TokenRefreshResponse{
		Token:     newToken,
		ValidDays: 90,
	}
	var responseDto dto.TokenRefreshResponse
	if err := httputil.EncodeResponse(ctx, responseModel, &responseDto); err != nil {
		a.logger.Error("Failed to encode refresh response", zap.Error(err))
		return err
	}

	return nil
}

// exchangeToken handles POST /api/tokens/jwt - exchanges access token for JWT
func (a *AbuseAPI) exchangeToken(c echo.Context) error {
	ctx := httputil.Context(c)

	var reqDto dto.TokenRefreshRequest
	_, ok := httputil.DecodeAndValidateRequest[*models.Token, *dto.TokenRefreshRequest](ctx, &reqDto)
	if !ok {
		return nil // Error handled by DecodeAndValidateRequest
	}

	caseID, reporterID, valid := a.tokenSvc.ValidateToken(reqDto.Token)
	if !valid {
		return ctx.Error(fmt.Errorf("invalid access token"), http.StatusUnauthorized)
	}

	configProvider := adapter.NewFromCore(a.ctx)
	cookieSetter := adapter.NewCookieSetter(configProvider)

	tokenString, err := cookieSetter.SetJWTCookie(c.Response(), fmt.Sprintf("%d", reporterID), jwt.PurposeLogin, 90*24*time.Hour,
		jwt.WithClaims(tjwt.NewAbuseJWTClaims(caseID, reporterID)),
		jwt.WithModifiers(jwt.ClaimModifier(func(claims gjwt.Claims) {
			if abuseClaims, ok := claims.(*tjwt.AbuseJWTClaims); ok {
				abuseClaims.CaseID = caseID
				abuseClaims.ReporterID = reporterID
			}
		})),
	)
	if err != nil {
		a.logger.Error("Failed to set JWT cookie", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to set authentication cookie: %w", err), http.StatusInternalServerError)
	}

	responseModel := &dto.JWTResponse{
		AccessToken: tokenString,
		ExpiresAt:   time.Now().Add(90 * 24 * time.Hour),
	}
	var responseDto dto.JWTResponse
	if err := httputil.EncodeResponse(ctx, responseModel, &responseDto); err != nil {
		a.logger.Error("Failed to encode JWT response", zap.Error(err))
		return err
	}

	return nil
}

// caseIDMiddleware extracts case ID from JWT custom claims
func caseIDMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {

			ctx := httputil.Context(c)
			hctx := c.Request().Context()

			claims, ok := auth.GetClaims[*tjwt.AbuseJWTClaims](hctx)
			if !ok {
				return ctx.Error(fmt.Errorf("missing required claims"), http.StatusUnauthorized)
			}

			if !jwt.PurposeEqual(claims.Audience, jwt.PurposeLogin) {
				return ctx.Error(fmt.Errorf("invalid token purpose"), http.StatusUnauthorized)
			}

			newCtx := context.WithValue(hctx, contextKeyCaseID, claims.CaseID)
			newCtx = context.WithValue(newCtx, contextKeyReporterID, claims.ReporterID)

			// Call the next handler with the new context
			c.SetRequest(c.Request().WithContext(newCtx))
			return next(c)
		}
	}
}
