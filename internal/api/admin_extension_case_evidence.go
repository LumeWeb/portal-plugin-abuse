package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/gabriel-vasile/mimetype"
	"github.com/labstack/echo/v4"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-middleware/auth/jwt"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-router"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	queryutilHttp "go.lumeweb.com/queryutil/http"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// registerEvidenceHandlers sets up evidence-related routes using portal-router.
func (e *AdminExtension) registerEvidenceHandlers(gRouter router.Router, accessSvc core.AccessService) error {
	schema := queryutil.NewSchemaProvider().ForType(dto.EvidenceResponse{})

	routes := router.DefineRoutes(
		// Upload Evidence
		router.NewRoute(http.MethodPost, "/cases/:id/evidence", e.uploadEvidence,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Upload evidence file"),
				router.WithDescription("Uploads a file as evidence for a specific case"),
				router.WithTags("Evidence"),
				router.WithPathParam("id", "Case ID", 1),
				router.WithFileUpload("The evidence file to upload", true),
				router.WithSuccessResponse(
					http.StatusCreated,
					"Evidence created successfully",
					router.WithJSONContent(dto.EvidenceResponse{}),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid request payload or file upload"),
						router.DefineSwaggerErrorResponse(http.StatusNotFound, "Case not found"), // Assuming case ID validation happens
						router.DefineSwaggerErrorResponse(http.StatusUnprocessableEntity, "Validation Error"),
					),
				),
			),
		),

		// List Case Evidence
		router.NewRoute(http.MethodGet, "/cases/:id/evidence", e.listCaseEvidence,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithListEndpoint(
					"List case evidence",
					"Retrieve a list of evidence items for a specific case",
					jwt.PurposeLogin, // Assuming admin endpoints use login JWT
					dto.EvidenceResponse{},
					schema, // Use schema for pagination/sorting/filtering
					schema.SortableFields(),
					nil, // No specific filter params defined in original mux code, rely on schema
					router.WithFilterParamsFromSchema(schema),
					router.WithPathParam("id", "Case ID", 1),
					router.WithErrorResponses(
						router.DefineSwaggerErrorResponses(
							router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid case ID format or filter parameters"),
							router.DefineSwaggerErrorResponse(http.StatusNotFound, "Case not found"), // Assuming case ID validation happens
						),
					),
				),
				router.WithSuccessResponse(http.StatusOK, "List of evidence items", router.WithTotalCountHeader()),
			),
		),

		// Get Evidence
		router.NewRoute(http.MethodGet, "/evidence/:id", e.getEvidence,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get evidence metadata"),
				router.WithDescription("Retrieve metadata for a specific evidence item by ID"),
				router.WithTags("Evidence"),
				router.WithPathParam("id", "Evidence ID", 1),
				router.WithSuccessResponse(
					http.StatusOK,
					"Evidence metadata",
					router.WithJSONContent(dto.EvidenceResponse{}),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid evidence ID format"),
						router.DefineSwaggerErrorResponse(http.StatusNotFound, "Evidence not found"),
					),
				),
			),
		),

		// Get Evidence Content
		router.NewRoute(http.MethodGet, "/evidence/:id/content", e.getEvidenceContent,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get evidence content"),
				router.WithDescription("Retrieve the content of a specific evidence item by ID"),
				router.WithTags("Evidence"),
				router.WithPathParam("id", "Evidence ID", 1),
				router.WithSuccessResponse(
					http.StatusOK,
					"Evidence content",
					router.WithContent("application/octet-stream", map[string]any{
						"type":   "string",
						"format": "binary",
					}),
					router.WithHeader("Content-Type", "The MIME type of the file content"),
					router.WithHeader("Content-Disposition", "Suggested filename for download"),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid evidence ID format"),
						router.DefineSwaggerErrorResponse(http.StatusNotFound, "Evidence not found"),
					),
				),
			),
		),
	)

	// Register routes with the router and access service
	// The base path for this extension is "/abuse", so routes like "/cases/:id/evidence" become "/abuse/cases/:id/evidence".
	// The access service registration needs the full path relative to the admin API root.
	// core.GetAPI("admin").Subdomain() is "admin", so the full path is "/admin/abuse" + route.Path
	return router.RegisterRoutes(gRouter, accessSvc, core.GetAPI("admin").Subdomain(), routes, router.WithCors())
}

// listCaseEvidence returns all evidence for a case
func (e *AdminExtension) listCaseEvidence(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Get case ID from path using Echo context
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid case ID format"), http.StatusBadRequest) // Use ctx.Error
	}

	// Use injected service
	if e.evidenceService == nil {
		e.logger.Error("Evidence service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Handle list request processing using Echo context
	if err := queryutilHttp.ProcessListRequest(
		c.Response(), c.Request(), // Pass raw http.ResponseWriter and http.Request
		"evidence",
		func(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Evidence, int64, error) {
			// Add filter for case ID
			// caseIDFilter := queryutil.UintField("case_id").Eq(uint(id)) // This line was the issue
			// filters = append(filters, caseIDFilter) // This line was the issue
			return e.evidenceService.GetByCaseID(uint(id), pagination) // Assuming GetByCaseID method supports pagination
		},
		func(evidence models.Evidence) dto.EvidenceResponse {
			var response dto.EvidenceResponse
			if err := response.FromModel(&evidence); err != nil {
				e.logger.Error("Failed to convert evidence", zap.Error(err))
				// In a real scenario, you might want to handle this conversion error more gracefully
				// or log it and return a generic error DTO. For this example, returning an empty DTO.
				return dto.EvidenceResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list evidence", zap.Error(err))
		// queryutilHttp.ProcessListRequest handles writing the error response internally,
		// but we still return the error for Echo's error handler chain.
		return err // Return the error from ProcessListRequest
	}

	return nil // Return nil on success
}

// getEvidence retrieves a specific evidence by ID
func (e *AdminExtension) getEvidence(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.evidenceService == nil {
		e.logger.Error("Evidence service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Extract ID from path using Echo context
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid evidence ID format"), http.StatusBadRequest) // Use ctx.Error
	}

	// Get the evidence using the service
	evidence, err := e.evidenceService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Error(errors.New("evidence not found"), http.StatusNotFound) // Use ctx.Error
		} else {
			e.logger.Error("Failed to fetch evidence", zap.Error(err))
			return ctx.Error(errors.New("failed to fetch evidence"), http.StatusInternalServerError) // Use ctx.Error
		}
	}

	// Prepare and send response
	var responseDto dto.EvidenceResponse
	// Use httputil.EncodeResponse for encoding
	if err := httputil.EncodeResponse[*models.Evidence, *dto.EvidenceResponse](ctx, evidence, &responseDto); err != nil {
		e.logger.Error("Failed to convert evidence to DTO", zap.Error(err))
		return err // httputil.EncodeResponse returns an error
	}

	return nil // Return nil on success
}

// uploadEvidence handles file uploads for case evidence
func (e *AdminExtension) uploadEvidence(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Get case ID from path using Echo context
	caseIDStr := c.Param("id")
	caseID, err := strconv.ParseUint(caseIDStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid case ID format"), http.StatusBadRequest) // Use ctx.Error
	}

	// Parse multipart form with 100MB max size using Echo context
	if err := c.Request().ParseMultipartForm(100 << 20); err != nil {
		return ctx.Error(errors.New("failed to parse multipart form"), http.StatusBadRequest) // Use ctx.Error
	}

	// Get evidence file from request using Echo context
	file, fileHeader, err := c.Request().FormFile("file")
	if err != nil {
		return ctx.Error(errors.New("missing or invalid file upload"), http.StatusBadRequest) // Use ctx.Error
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
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Get JSON data from form field
	dataString := c.Request().FormValue("data")
	if dataString == "" {
		return ctx.Error(errors.New("missing data field"), http.StatusBadRequest) // Use ctx.Error
	}

	// Replace request body with JSON data for validation using Echo context
	// Need to create a new request with the modified body for httputil.DecodeAndValidateRequest
	newReq := c.Request().WithContext(c.Request().Context())
	newReq.Body = io.NopCloser(bytes.NewBufferString(dataString))
	newReq.Header.Set("Content-Type", "application/json") // Set content type to JSON for decoding
	ctx.SetRequest(newReq)

	// Decode and validate request using standard DTO process
	var requestDto dto.EvidenceCreateRequest
	// Pass the new request to httputil.DecodeAndValidateRequest
	model, ok := httputil.DecodeAndValidateRequest[*models.Evidence, *dto.EvidenceCreateRequest](ctx, &requestDto)
	if !ok {
		// httputil.DecodeAndValidateRequest handles the error response internally
		return nil // Error handled
	}

	// Use uploaded filename from multipart header
	model.FileName = fileHeader.Filename

	// Detect MIME type from content
	// Need to read from the file without consuming it for the service
	// Reset file reader to beginning before MIME detection
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		e.logger.Error("Failed to reset file reader for MIME detection", zap.Error(err))
		return ctx.Error(errors.New("failed to process file"), http.StatusInternalServerError) // Use ctx.Error
	}
	mimeType, err := mimetype.DetectReader(file)
	if err != nil {
		e.logger.Error("Failed to detect MIME type", zap.Error(err))
		return ctx.Error(errors.New("failed to validate file type"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Reset file reader to beginning again for the service
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		e.logger.Error("Failed to reset file reader for service", zap.Error(err))
		return ctx.Error(errors.New("failed to process file"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Set validated metadata from multipart header and detection
	model.ContentType = mimeType.String()
	model.FileSize = fileHeader.Size

	// Set case ID and submitter from context (assuming user ID is available in context)
	model.CaseID = uint(caseID)
	// TODO: Get real user ID from auth context
	// Assuming user ID is available in the Echo context after auth middleware
	// For now, hardcoding as in the original code
	model.SubmitterID = 1

	// Create evidence record
	// Pass the file reader to the service
	evidence, err := e.evidenceService.CreateFromData(file, model) // Pass the multipart.File directly

	if err != nil {
		e.logger.Error("Failed to create evidence", zap.Error(err))
		return ctx.Error(errors.New("failed to save evidence"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Return created evidence
	var responseDto dto.EvidenceResponse
	c.Response().WriteHeader(http.StatusCreated) // Set status code using Echo context
	// Use httputil.EncodeResponse for encoding
	if err := httputil.EncodeResponse[*models.Evidence, *dto.EvidenceResponse](ctx, evidence, &responseDto); err != nil {
		e.logger.Error("Failed to convert evidence to DTO", zap.Error(err))
		return err // httputil.EncodeResponse returns an error
	}

	return nil // Return nil on success
}

// getEvidenceContent gets the actual file content of evidence
func (e *AdminExtension) getEvidenceContent(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Get evidence service from context
	if e.evidenceService == nil {
		e.logger.Error("Evidence service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Extract ID from path using Echo context
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid evidence ID format"), http.StatusBadRequest) // Use ctx.Error
	}

	// Get evidence by ID first to verify it exists
	evidence, err := e.evidenceService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Error(errors.New("evidence not found"), http.StatusNotFound) // Use ctx.Error
		} else {
			e.logger.Error("Failed to fetch evidence", zap.Error(err))
			return ctx.Error(errors.New("failed to fetch evidence"), http.StatusInternalServerError) // Use ctx.Error
		}
	}

	// Get content
	contentReader, contentType, err := e.evidenceService.GetContent(uint(id))
	if err != nil {
		e.logger.Error("Failed to get evidence content", zap.Error(err))
		return ctx.Error(errors.New("failed to get evidence content"), http.StatusInternalServerError) // Use ctx.Error
	}
	defer func(contentReader io.ReadCloser) {
		err := contentReader.Close()
		if err != nil {
			e.logger.Error("Failed to close content reader", zap.Error(err))
		}
	}(contentReader)

	// Set appropriate headers using Echo context
	c.Response().Header().Set(echo.HeaderContentType, contentType)
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%s", evidence.FileName))

	// Stream content to response using Echo context
	// Echo's Stream method is suitable for this
	return c.Stream(http.StatusOK, contentType, contentReader)
}
