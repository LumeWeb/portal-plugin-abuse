package api

import (
	"context"
	"fmt"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	svcTypes "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/queryutil"
	queryutilHttp "go.lumeweb.com/queryutil/http"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal/config"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"

	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	pluginConfig "go.lumeweb.com/portal-plugin-abuse/internal/config"
	"go.lumeweb.com/portal-plugin-abuse/internal/validation"
)

// AbuseAPI handles abuse report endpoints
type AbuseAPI struct {
	ctx                core.Context
	logger             *core.Logger
	abuseReportService svcTypes.AbuseReportService
	emailService       svcTypes.EmailService
	tokenSvc           svcTypes.TokenService
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

			api.tokenSvc = core.GetService[svcTypes.TokenService](ctx, "abuse.token_service")
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
	return ""
}

// Configure sets up the API routes
func (a *AbuseAPI) Configure(router *mux.Router, _ core.AccessService) error {
	// Create a subrouter for abuse API endpoints
	abuseRouter := router.PathPrefix("/api/abuse").Subrouter()

	// Register routes
	abuseRouter.HandleFunc("/report", a.submitReport).Methods(http.MethodPost)
	abuseRouter.HandleFunc("/report/{confirmationNumber}", a.getReportStatus).Methods(http.MethodGet)

	// Token-protected endpoints
	protectedRouter := abuseRouter.PathPrefix("/cases").Subrouter()
	protectedRouter.Use(a.tokenMiddleware)
	protectedRouter.HandleFunc("/{reference}", a.getCase).Methods("GET")
	protectedRouter.HandleFunc("/{reference}/comment", a.addComment).Methods("POST")
	protectedRouter.HandleFunc("/{reference}/upload", a.uploadFile).Methods("POST")

	// Token management
	abuseRouter.HandleFunc("/tokens/validate", a.validateToken).Methods("POST")
	abuseRouter.HandleFunc("/tokens/refresh", a.refreshToken).Methods("POST")

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
func (a *AbuseAPI) submitReport(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use the proper DTO instead of an anonymous struct
	var requestDto dto.AbuseReportRequest

	// Validate the request using Zog
	caseModel, ok := httputil.DecodeAndValidateRequest[*models.Case, *dto.AbuseReportRequest](ctx, &requestDto)
	if !ok {
		return
	}

	// Submit report
	caseModel, err := a.abuseReportService.SubmitReport(r.Context(), caseModel)
	if err != nil {
		a.logger.Error("Failed to submit report", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, fmt.Sprintf("Failed to submit report: %v", err))
		return
	}

	var responseDto dto.AbuseReportResponse

	ctx.Response.WriteHeader(http.StatusCreated)
	err = httputil.EncodeResponse[*models.Case, *dto.AbuseReportResponse](ctx, caseModel, &responseDto)
	if err != nil {
		a.logger.Error("Failed to submit report", zap.Error(err))
	}
}

// getReportStatus handles retrieving the status of a report
func (a *AbuseAPI) tokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := httputil.Context(r, w)
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		caseID, reporterID, valid := a.tokenSvc.ValidateToken(token)
		if !valid {
			sendErrorResponse(&ctx, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		// Store IDs in context
		hctx := context.WithValue(r.Context(), "caseID", caseID)
		hctx = context.WithValue(ctx, "reporterID", reporterID)
		next.ServeHTTP(w, r.WithContext(hctx))
	})
}

// getCase handles GET /cases/{reference}
func (a *AbuseAPI) getCase(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)
	vars := mux.Vars(r)
	reference := vars["reference"]

	// Get the validated case and reporter IDs from the context
	caseID, _ := r.Context().Value("caseID").(uint)
	reporterID, _ := r.Context().Value("reporterID").(uint)

	// Get the case service from context
	caseService := core.GetService[svcTypes.CaseService](a.ctx, svcTypes.CASE_SERVICE)

	// Get the case by ID
	caseModel, err := caseService.GetByID(caseID)
	if err != nil {
		a.logger.Error("Failed to get case", zap.Error(err))
		_ = ctx.Error(fmt.Errorf("failed to get case: %w", err), http.StatusInternalServerError)
		return
	}

	// Verify that the reference number matches
	if caseModel.ReferenceNumber != reference {
		_ = ctx.Error(fmt.Errorf("case reference mismatch"), http.StatusForbidden)
		return
	}

	// Verify that the reporter ID matches
	if caseModel.ReporterID != reporterID {
		_ = ctx.Error(fmt.Errorf("unauthorized access"), http.StatusForbidden)
		return
	}

	// Get communications using queryutil helper
	var commDTOs []dto.CommunicationResponse
	if err := queryutilHttp.ProcessListRequest(
		ctx.Response, r,
		"communications",
		func(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Communication, int64, error) {
			// Add filter for only outgoing/external communications
			// Add separate filters for each direction we want to include
			filters = append(filters, queryutil.Filter{
				Field:    "direction",
				Operator: queryutil.OperatorEquals,
				Value:    string(models.CommunicationDirectionOutgoing),
			})
			filters = append(filters, queryutil.Filter{
				Field:    "direction",
				Operator: queryutil.OperatorEquals,
				Value:    string(models.CommunicationDirectionExternal),
			})
			return core.GetService[svcTypes.CommunicationService](a.ctx, svcTypes.COMMUNICATION_SERVICE).
				ListByCaseID(caseID, filters, sorts, pagination)
		},
		func(comm models.Communication) dto.CommunicationResponse {
			var dtoComm dto.CommunicationResponse
			if err := dtoComm.FromModel(&comm); err != nil {
				a.logger.Error("Failed to convert communication", zap.Error(err))
				return dto.CommunicationResponse{}
			}
			return dtoComm
		},
	); err != nil {
		a.logger.Error("Failed to process communications", zap.Error(err))
	}

	// Get scans for the case
	scanService := core.GetService[svcTypes.ScanService](a.ctx, svcTypes.SCAN_SERVICE)
	scans, _, err := scanService.GetScansForCase(caseID, defaultPagination())
	if err != nil {
		a.logger.Error("Failed to get scans", zap.Error(err))
		// Not a critical error, continue with empty scans
	}

	publicScans := make([]dto.ScanResponse, len(scans))
	for i, scan := range scans {
		var scanDTO dto.ScanResponse
		if err := scanDTO.FromModel(&scan); err != nil {
			a.logger.Error("Failed to convert scan", zap.Error(err))
			continue
		}
		publicScans[i] = scanDTO
	}

	// Build response using existing DTO
	response := dto.CaseResponse{
		BaseResponse: dto.BaseResponse{
			ID:        caseModel.ID,
			CreatedAt: caseModel.CreatedAt,
			UpdatedAt: caseModel.UpdatedAt,
		},
		ReferenceNumber: caseModel.ReferenceNumber,
		Type:            string(caseModel.Type),
		Status:          string(caseModel.Status),
		Description:     caseModel.Description,
		Priority:        string(caseModel.Priority),
		Source:          string(caseModel.Source),
		NeedsReview:     caseModel.NeedsReview,
		ReporterID:      caseModel.ReporterID,
		SubjectID:       caseModel.SubjectID,
		Communications:  commDTOs,
		Scans:           publicScans,
	}

	ctx.Encode(response)
}

// addComment handles POST /cases/{reference}/comment
func (a *AbuseAPI) addComment(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)
	vars := mux.Vars(r)
	reference := vars["reference"]

	// Get the validated case and reporter IDs from the context
	caseID, _ := r.Context().Value("caseID").(uint)
	reporterID, _ := r.Context().Value("reporterID").(uint)

	// Parse the request
	var req struct {
		Content string `json:"content"`
	}
	if err := ctx.Decode(&req); err != nil {
		return
	}

	// Get case service
	caseService := core.GetService[svcTypes.CaseService](a.ctx, svcTypes.CASE_SERVICE)
	caseModel, err := caseService.GetByID(caseID)
	if err != nil {
		a.logger.Error("Failed to get case", zap.Error(err))
		_ = ctx.Error(fmt.Errorf("failed to get case: %w", err), http.StatusInternalServerError)
		return
	}

	// Verify reference match
	if caseModel.ReferenceNumber != reference {
		_ = ctx.Error(fmt.Errorf("case reference mismatch"), http.StatusForbidden)
		return
	}

	// Verify reporter match
	if caseModel.ReporterID != reporterID {
		_ = ctx.Error(fmt.Errorf("unauthorized access"), http.StatusForbidden)
		return
	}

	// Create communication
	comm := &models.Communication{
		CaseID:    caseID,
		SenderID:  reporterID,
		Type:      models.CommunicationTypeResponse,
		Direction: models.CommunicationDirectionExternal,
		Content:   req.Content,
		ThreadID:  a.emailService.GenerateCaseThreadID(caseID, caseModel.ReferenceNumber),
	}

	// Save communication
	commService := core.GetService[svcTypes.CommunicationService](a.ctx, svcTypes.COMMUNICATION_SERVICE)
	created, err := commService.Create(comm)
	if err != nil {
		a.logger.Error("Failed to create communication", zap.Error(err))
		_ = ctx.Error(fmt.Errorf("failed to add comment: %w", err), http.StatusInternalServerError)
		return
	}

	ctx.Response.WriteHeader(http.StatusCreated)
	ctx.Encode(map[string]interface{}{
		"id":         created.ID,
		"type":       string(created.Type),
		"direction":  string(created.Direction),
		"content":    created.Content,
		"created_at": created.CreatedAt.Format(time.RFC3339),
	})
}

// uploadFile handles POST /cases/{reference}/upload
func (a *AbuseAPI) uploadFile(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)
	vars := mux.Vars(r)
	reference := vars["reference"]

	// Get validated IDs from context
	caseID, _ := r.Context().Value("caseID").(uint)
	reporterID, _ := r.Context().Value("reporterID").(uint)

	// Get case details
	caseService := core.GetService[svcTypes.CaseService](a.ctx, svcTypes.CASE_SERVICE)
	caseModel, err := caseService.GetByID(caseID)
	if err != nil {
		a.logger.Error("Failed to get case", zap.Error(err))
		_ = ctx.Error(fmt.Errorf("failed to get case: %w", err), http.StatusInternalServerError)
		return
	}

	// Verify reference match
	if caseModel.ReferenceNumber != reference {
		_ = ctx.Error(fmt.Errorf("case reference mismatch"), http.StatusForbidden)
		return
	}

	// Verify reporter match
	if caseModel.ReporterID != reporterID {
		_ = ctx.Error(fmt.Errorf("unauthorized access"), http.StatusForbidden)
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		_ = ctx.Error(fmt.Errorf("failed to parse form: %w", err), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		_ = ctx.Error(fmt.Errorf("failed to get file: %w", err), http.StatusBadRequest)
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			a.logger.Error("Failed to close file", zap.Error(err))
		}
	}(file)

	fileData, err := io.ReadAll(file)
	if err != nil {
		_ = ctx.Error(fmt.Errorf("failed to read file: %w", err), http.StatusInternalServerError)
		return
	}

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(fileData)
	}

	// Create comment
	comm := &models.Communication{
		CaseID:    caseID,
		SenderID:  reporterID,
		Type:      models.CommunicationTypeNote,
		Direction: models.CommunicationDirectionExternal,
		Content:   fmt.Sprintf("File uploaded: %s", header.Filename),
	}

	if _, err := core.GetService[svcTypes.CommunicationService](a.ctx, svcTypes.COMMUNICATION_SERVICE).Create(comm); err != nil {
		a.logger.Error("Failed to create comment", zap.Error(err))
	}

	ctx.Response.WriteHeader(http.StatusCreated)
	ctx.Encode(map[string]interface{}{
		"message":      "File uploaded successfully",
		"filename":     header.Filename,
		"size":         len(fileData),
		"content_type": contentType,
	})
}

// validateToken handles POST /tokens/validate
func (a *AbuseAPI) validateToken(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	var req struct {
		Token string `json:"token"`
	}
	if err := ctx.Decode(&req); err != nil {
		return
	}

	caseID, reporterID, valid := a.tokenSvc.ValidateToken(req.Token)
	if !valid {
		_ = ctx.Error(fmt.Errorf("invalid or expired token"), http.StatusUnauthorized)
		return
	}

	// Verify case exists
	caseService := core.GetService[svcTypes.CaseService](a.ctx, svcTypes.CASE_SERVICE)
	caseModel, err := caseService.GetByID(caseID)
	if err != nil {
		a.logger.Error("Failed to get case", zap.Error(err))
		_ = ctx.Error(fmt.Errorf("failed to get case: %w", err), http.StatusInternalServerError)
		return
	}

	// Verify reporter exists
	reporterService := core.GetService[svcTypes.ReporterService](a.ctx, svcTypes.REPORTER_SERVICE)
	if _, err := reporterService.GetByID(reporterID); err != nil {
		a.logger.Error("Failed to get reporter", zap.Error(err))
		_ = ctx.Error(fmt.Errorf("failed to get reporter: %w", err), http.StatusInternalServerError)
		return
	}

	ctx.Encode(map[string]interface{}{
		"valid":       true,
		"case_id":     caseID,
		"reporter_id": reporterID,
		"reference":   caseModel.ReferenceNumber,
	})
}

// refreshToken handles POST /tokens/refresh
func (a *AbuseAPI) refreshToken(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	var req struct {
		Token string `json:"token"`
	}
	if err := ctx.Decode(&req); err != nil {
		return
	}

	caseID, reporterID, valid := a.tokenSvc.ValidateToken(req.Token)
	if !valid {
		_ = ctx.Error(fmt.Errorf("invalid or expired token"), http.StatusUnauthorized)
		return
	}

	newToken, err := a.tokenSvc.GenerateToken(caseID, reporterID, 90)
	if err != nil {
		a.logger.Error("Failed to generate token", zap.Error(err))
		_ = ctx.Error(fmt.Errorf("failed to refresh token: %w", err), http.StatusInternalServerError)
		return
	}

	ctx.Encode(map[string]interface{}{
		"token":       newToken,
		"valid_days":  90,
		"case_id":     caseID,
		"reporter_id": reporterID,
	})
}

func (a *AbuseAPI) getReportStatus(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Get confirmation number from URL
	vars := mux.Vars(r)
	confirmationNumber := vars["confirmationNumber"]

	if confirmationNumber == "" || !validation.IsValidConfirmationNumber(confirmationNumber) {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid confirmation number format")
		return
	}

	// Get report status
	caseModel, err := a.abuseReportService.GetReportStatus(r.Context(), confirmationNumber)
	if err != nil {
		a.logger.Error("Failed to get report status", zap.Error(err), zap.String("confirmationNumber", confirmationNumber))
		sendErrorResponse(&ctx, http.StatusNotFound, "Report not found")
		return
	}

	var responseDto dto.AbuseReportResponse
	err = httputil.EncodeResponse[*models.Case, *dto.AbuseReportResponse](ctx, caseModel, &responseDto)
	if err != nil {
		a.logger.Error("Failed to submit report", zap.Error(err))
	}
}
