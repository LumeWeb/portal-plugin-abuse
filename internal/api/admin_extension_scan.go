package api

import (
	"errors"
	"go.lumeweb.com/portal-plugin-abuse/internal/db"
	"net/http"
	"strconv"

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

// registerScanHandlers registers all scan-related route handlers using portal-router.
func (e *AdminExtension) registerScanHandlers(gRouter router.Router, accessSvc core.AccessService) error {
	scanSchema := queryutil.NewSchemaProvider().ForType(dto.ScanResponse{})
	scanResultSchema := queryutil.NewSchemaProvider().ForType(dto.ScanResultResponse{})

	routes := router.DefineRoutes(
		// List Case Scans
		router.NewRoute(http.MethodGet, "/cases/:caseId/scans", e.listCaseScans,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithListEndpoint(
					"List case scans",
					"Retrieve a list of scans for a specific case",
					jwt.PurposeLogin, // Assuming admin endpoints use login JWT
					dto.ScanResponse{},
					scanSchema, // Use schema for pagination/sorting/filtering
					scanSchema.SortableFields(),
					nil, // No specific filter params defined in original mux code, rely on schema
					router.WithFilterParamsFromSchema(scanSchema),
					router.WithPathParam("caseId", "Case ID", 1),
					router.WithErrorResponses(
						router.DefineSwaggerErrorResponses(
							router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid case ID format or filter parameters"),
							router.DefineSwaggerErrorResponse(http.StatusNotFound, "Case not found"), // Assuming case ID validation happens
						),
					),
				),
				router.WithSuccessResponse(http.StatusOK, "List of scans", router.WithTotalCountHeader()),
			),
		),

		// Create Scan Request for Case Subject
		router.NewRoute(http.MethodPost, "/cases/:caseId/scans", e.createScanRequest,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Create scan request for case subject"),
				router.WithDescription("Initiates a scan for the subject associated with the case."),
				router.WithTags("Scans"),
				router.WithPathParam("caseId", "Case ID", 1),
				router.WithRequestBody(&dto.ManualScanRequest{}, "Manual scan request details", true),
				router.WithSuccessResponse(
					http.StatusAccepted,
					"Scan initiated successfully",
					router.WithJSONContent(map[string]string{"message": "Scan request accepted"}),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid request payload or case ID format"),
						router.DefineSwaggerErrorResponse(http.StatusNotFound, "Case not found"),
						router.DefineSwaggerErrorResponse(http.StatusUnprocessableEntity, "Validation Error"),
					),
				),
			),
		),

		// Get Scan
		router.NewRoute(http.MethodGet, "/cases/:caseId/scans/:scanId", e.getScan,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get scan"),
				router.WithDescription("Retrieve details of a specific scan"),
				router.WithTags("Scans"),
				router.WithPathParam("caseId", "Case ID", 1),
				router.WithPathParam("scanId", "Scan ID", 1),
				router.WithSuccessResponse(
					http.StatusOK,
					"Scan details",
					router.WithJSONContent(dto.ScanResponse{}),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid scan ID format"),
						router.DefineSwaggerErrorResponse(http.StatusNotFound, "Scan not found"),
					),
				),
			),
		),

		// Get Scan Results
		router.NewRoute(http.MethodGet, "/cases/:caseId/scans/:scanId/results", e.getScanResults,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithListEndpoint( // Use WithListEndpoint for scan results
					"Get scan results",
					"Retrieve results of a specific scan with filtering, sorting, and pagination",
					jwt.PurposeLogin, // Assuming admin endpoints use login JWT
					dto.ScanResultResponse{},
					scanResultSchema, // Use scanResultSchema for parameters and response structure
					scanResultSchema.SortableFields(),
					nil, // No specific filter params defined, rely on schema
					router.WithFilterParamsFromSchema(scanResultSchema),
					router.WithPathParam("caseId", "Case ID", 1),
					router.WithPathParam("scanId", "Scan ID", 1),
					router.WithErrorResponses(
						router.DefineSwaggerErrorResponses(
							router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid scan ID format or filter parameters"),
							router.DefineSwaggerErrorResponse(http.StatusNotFound, "Scan not found"),
						),
					),
				),
				router.WithSuccessResponse(http.StatusOK, "Scan results", router.WithTotalCountHeader()), // Add total count header
			),
		),
	)

	// Register routes with the router and access service
	// The base path for this extension is "/abuse", so routes like "/cases/:caseId/scans" become "/abuse/cases/:caseId/scans".
	// The access service registration needs the full path relative to the admin API root.
	// core.GetAPI("admin").Subdomain() is "admin", so the full path is "/admin/abuse" + route.Path
	return router.RegisterRoutes(gRouter, accessSvc, core.GetAPI("admin").Subdomain(), routes, router.WithCors())
}

// listCaseScans returns all scans for a case using queryutil list API
func (e *AdminExtension) listCaseScans(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Get case ID from path using Echo context
	caseIdStr := c.Param("caseId")
	caseId, err := strconv.ParseUint(caseIdStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid case ID format"), http.StatusBadRequest) // Use ctx.Error
	}

	// Use injected service
	if e.caseService == nil {
		e.logger.Error("Case service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Verify case exists
	if _, err := e.caseService.GetByID(uint(caseId)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Error(errors.New("case not found"), http.StatusNotFound) // Use ctx.Error
		} else {
			e.logger.Error("Failed to fetch case", zap.Error(err))
			return ctx.Error(errors.New("failed to fetch case"), http.StatusInternalServerError) // Use ctx.Error
		}
	}

	// Use injected service
	if e.scanService == nil {
		e.logger.Error("Scan service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Use standardized list processing with Echo context
	if err := queryutilHttp.ProcessListRequest(
		c.Response(), c.Request(), // Pass raw http.ResponseWriter and http.Request
		"scans",
		func(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.CaseScan, int64, error) {
			// The service method GetScansForCase is expected to handle applying the case ID filter
			// along with the filters parsed from the request.
			return e.scanService.GetScansForCase(uint(caseId), pagination)
		},
		func(scan models.CaseScan) dto.ScanResponse {
			var response dto.ScanResponse
			if err := response.FromModel(&scan); err != nil {
				e.logger.Error("Failed to convert scan", zap.Error(err))
				// In a real scenario, you might want to handle this conversion error more gracefully
				// or log it and return a generic error DTO. For this example, returning an empty DTO.
				return dto.ScanResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list scans", zap.Error(err))
		// queryutilHttp.ProcessListRequest handles writing the error response internally,
		// but we still return the error for Echo's error handler chain.
		return err // Return the error from ProcessListRequest
	}

	return nil // Return nil on success
}

// createScanRequest initiates a scan for an existing subject
func (e *AdminExtension) createScanRequest(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Get case ID from path using Echo context
	caseIdStr := c.Param("caseId")
	caseId, err := strconv.ParseUint(caseIdStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid case ID format"), http.StatusBadRequest) // Use ctx.Error
	}

	// Validate request DTO using Echo context
	var req dto.ManualScanRequest
	// httputil.DecodeAndValidateRequest now takes Echo context
	if _, ok := httputil.DecodeAndValidateRequest[*models.CaseScan, *dto.ManualScanRequest](ctx, &req); !ok {
		// Error handled by DecodeAndValidateRequest
		return nil // Return nil as the error is already handled
	}

	// Use injected services
	if e.scanService == nil || e.caseService == nil {
		e.logger.Error("Required service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Verify case exists
	if _, err := e.caseService.GetByID(uint(caseId)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Error(errors.New("case not found"), http.StatusNotFound) // Use ctx.Error
		} else {
			e.logger.Error("Failed to fetch case", zap.Error(err))
			return ctx.Error(errors.New("failed to fetch case"), http.StatusInternalServerError) // Use ctx.Error
		}
	}

	// Create scan request
	if err := e.scanService.CreateScanRequest(uint(caseId)); err != nil {
		e.logger.Error("Failed to create scan request", zap.Error(err))
		return ctx.Error(errors.New("failed to initiate scan"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Send success response using Echo context
	return c.JSON(http.StatusAccepted, map[string]string{"message": "Scan request accepted"})
}

// getScan retrieves a specific scan by ID
func (e *AdminExtension) getScan(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.scanService == nil {
		e.logger.Error("Scan service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Extract scanId from path using Echo context
	scanIdStr := c.Param("scanId")
	scanId, err := strconv.ParseUint(scanIdStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid scan ID format"), http.StatusBadRequest) // Use ctx.Error
	}

	// Get the scan using the service
	scan, err := e.scanService.GetScanById(uint(scanId))
	if err != nil {
		if errors.Is(err, db.ErrRecordNotFound) {
			return ctx.Error(errors.New("scan not found"), http.StatusNotFound) // Use ctx.Error
		} else {
			e.logger.Error("Failed to fetch scan", zap.Error(err))
			return ctx.Error(errors.New("failed to fetch scan"), http.StatusInternalServerError) // Use ctx.Error
		}
	}

	// Prepare and send response
	var responseDto dto.ScanResponse
	// Use httputil.EncodeResponse for encoding
	if err := httputil.EncodeResponse[*models.CaseScan, *dto.ScanResponse](ctx, scan, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
		return err // httputil.EncodeResponse returns an error
	}

	return nil // Return nil on success
}

// getScanResults returns results for a specific scan
func (e *AdminExtension) getScanResults(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.scanService == nil {
		e.logger.Error("Scan service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Extract scanId from path using Echo context
	scanIdStr := c.Param("scanId")
	scanId, err := strconv.ParseUint(scanIdStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid scan ID format"), http.StatusBadRequest) // Use ctx.Error
	}

	// Use standardized list processing with Echo context
	// ProcessListRequest will parse filters, sorts, and pagination from the request
	if err := queryutilHttp.ProcessListRequest(
		c.Response(), c.Request(), // Pass raw http.ResponseWriter and http.Request
		"scan_results",
		func(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]*core.ScanResult, int64, error) {
			// Call the updated service method with all parameters
			results, total, err := e.scanService.GetScanResults(uint(scanId), filters, sorts, pagination)
			if err != nil {
				// Check for not found specifically for the scan itself
				if errors.Is(err, db.ErrRecordNotFound) {
					// Return a specific error that ProcessListRequest can map to 404
					return nil, 0, gorm.ErrRecordNotFound
				}
				return nil, 0, err
			}
			return results, total, nil
		},
		func(result *core.ScanResult) dto.ScanResultResponse {
			var response dto.ScanResultResponse
			if err := response.FromModel(result); err != nil {
				e.logger.Error("Failed to convert scan result", zap.Error(err))
				// In a real scenario, you might want to handle this conversion error more gracefully
				// or log it and return a generic error DTO. For this example, returning an empty DTO.
				return dto.ScanResultResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list scan results", zap.Error(err))
		// queryutilHttp.ProcessListRequest handles writing the error response internally,
		// but we still return the error for Echo's error handler chain.
		// Check if the error is the "scan not found" error we returned from the data func
		if err.Error() == "scan not found" {
			return ctx.Error(errors.New("scan not found"), http.StatusNotFound) // Use ctx.Error for Echo
		}
		return err // Return the error from ProcessListRequest
	}

	return nil // Return nil on success
}
