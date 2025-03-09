package api

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gorilla/mux"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	queryutilHttp "go.lumeweb.com/queryutil/http"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
)

// registerEvidenceHandlers sets up evidence-related routes
func (e *AdminExtension) registerEvidenceHandlers(router *mux.Router, accessSvc core.AccessService) error {
	routes := []struct {
		Path    string
		Method  string
		Handler http.HandlerFunc
		Access  string
	}{
		{"/cases/{id}/evidence", "POST", e.uploadEvidence, core.ACCESS_ADMIN_ROLE},
		{"/cases/{id}/evidence", "GET", e.listCaseEvidence, core.ACCESS_ADMIN_ROLE},
		{"/evidence/{id}", "GET", e.getEvidence, core.ACCESS_ADMIN_ROLE},
		{"/evidence/{id}/content", "GET", e.getEvidenceContent, core.ACCESS_ADMIN_ROLE},
	}

	for _, route := range routes {
		router.HandleFunc(route.Path, route.Handler).Methods(route.Method)
		if err := accessSvc.RegisterRoute("admin", "/admin/abuse"+route.Path, route.Method, route.Access); err != nil {
			return fmt.Errorf("failed to register route %s: %w", route.Path, err)
		}
	}

	return nil
}

// listCaseEvidence returns all evidence for a case
func (e *AdminExtension) listCaseEvidence(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Get case ID from path
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid case ID format")
		return
	}

	// Use injected service
	if e.evidenceService == nil {
		e.logger.Error("Evidence service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Handle list request processing
	if err := queryutilHttp.ProcessListRequest(
		ctx.Response, r,
		"evidence",
		func(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Evidence, int64, error) {
			return e.evidenceService.GetByCaseID(uint(id), pagination)
		},
		func(evidence models.Evidence) dto.EvidenceResponse {
			var response dto.EvidenceResponse
			if err := response.FromModel(&evidence); err != nil {
				e.logger.Error("Failed to convert evidence", zap.Error(err))
				return dto.EvidenceResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list evidence", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to list evidence")
	}
}

// getEvidence retrieves a specific evidence by ID
func (e *AdminExtension) getEvidence(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.evidenceService == nil {
		e.logger.Error("Evidence service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Extract ID from path
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid evidence ID format")
		return
	}

	// Get the evidence using the service
	evidence, err := e.evidenceService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sendErrorResponse(&ctx, http.StatusNotFound, "Evidence not found")
		} else {
			e.logger.Error("Failed to fetch evidence", zap.Error(err))
			sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to fetch evidence")
		}
		return
	}

	// Prepare and send response
	var responseDto dto.EvidenceResponse
	if err := httputil.EncodeResponse[*models.Evidence, *dto.EvidenceResponse](ctx, evidence, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// uploadEvidence handles file uploads for case evidence
func (e *AdminExtension) uploadEvidence(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Get case ID from path
	vars := mux.Vars(r)
	caseID, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid case ID format")
		return
	}

	// Parse multipart form with 100MB max size
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Failed to parse multipart form")
		return
	}

	// Get evidence file from request
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Missing or invalid file upload")
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			e.logger.Error("Failed to close multipart file", zap.Error(err))
		}
	}(file)

	// Get services
	if e.evidenceService == nil {
		e.logger.Error("Evidence service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Get JSON data from form field and replace request body
	dataString := r.FormValue("data")
	if dataString == "" {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Missing data field")
		return
	}

	// Replace request body with JSON data for validation
	r.Body = io.NopCloser(bytes.NewBufferString(dataString))
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			e.logger.Error("Failed to close body", zap.Error(err))
		}
	}(r.Body)

	// Decode and validate request using standard DTO process
	var requestDto dto.EvidenceCreateRequest
	model, ok := httputil.DecodeAndValidateRequest[*models.Evidence, *dto.EvidenceCreateRequest](ctx, &requestDto)
	if !ok {
		return
	}

	// Use uploaded filename from multipart header
	model.FileName = fileHeader.Filename

	// Detect MIME type from content
	mimeType, err := mimetype.DetectReader(file)
	if err != nil {
		e.logger.Error("Failed to detect MIME type", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to validate file type")
		return
	}

	// Reset file reader to beginning after MIME detection
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		e.logger.Error("Failed to reset file reader", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to process file")
		return
	}

	// Set validated metadata from multipart header and detection
	model.ContentType = mimeType.String()
	model.FileSize = fileHeader.Size

	// Set case ID and submitter from context
	model.CaseID = uint(caseID)
	model.SubmitterID = 1 // TODO: Get real user ID from auth context

	// Create evidence record
	evidence, err := e.evidenceService.CreateFromData(file, model)

	if err != nil {
		e.logger.Error("Failed to create evidence", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to save evidence")
		return
	}

	// Return created evidence
	var responseDto dto.EvidenceResponse
	ctx.Response.WriteHeader(http.StatusCreated)
	if err := httputil.EncodeResponse[*models.Evidence, *dto.EvidenceResponse](ctx, evidence, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// getEvidenceContent gets the actual file content of evidence
func (e *AdminExtension) getEvidenceContent(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Get evidence service from context
	if e.evidenceService == nil {
		e.logger.Error("Evidence service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Extract ID from path
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid evidence ID format")
		return
	}

	// Get evidence by ID first to verify it exists
	evidence, err := e.evidenceService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sendErrorResponse(&ctx, http.StatusNotFound, "Evidence not found")
		} else {
			e.logger.Error("Failed to fetch evidence", zap.Error(err))
			sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to fetch evidence")
		}
		return
	}

	// Get content
	contentReader, contentType, err := e.evidenceService.GetContent(uint(id))
	if err != nil {
		e.logger.Error("Failed to get evidence content", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to get evidence content")
		return
	}

	// Set appropriate headers
	ctx.Response.Header().Set("Content-Type", contentType)
	ctx.Response.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", evidence.FileName))

	// Stream content to response
	if _, err := io.Copy(ctx.Response, contentReader); err != nil {
		e.logger.Error("Failed to write evidence content to response", zap.Error(err))
	}
}
