package api

import (
	"errors"
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

// registerReporterHandlers registers all reporter-related route handlers using portal-router.
func (e *AdminExtension) registerReporterHandlers(gRouter router.Router, accessSvc core.AccessService) error {
	schema := queryutil.NewSchemaProvider().ForType(dto.ReporterResponse{})

	routes := router.DefineRoutes(
		// List Reporters
		router.NewRoute(http.MethodGet, "/reporters", e.listReporters,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithListEndpoint(
					"List reporters",
					"Retrieve a list of reporters with optional filtering and pagination",
					jwt.PurposeLogin, // Assuming admin endpoints use login JWT
					dto.ReporterResponse{},
					schema, // Use schema for pagination/sorting/filtering
					schema.SortableFields(),
					nil, // No specific filter params defined in original mux code, rely on schema
					router.WithFilterParamsFromSchema(schema),
					router.WithErrorResponses(
						router.DefineSwaggerErrorResponses(
							router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid filter parameters"),
						),
					),
				),
				router.WithSuccessResponse(http.StatusOK, "List of reporters", router.WithTotalCountHeader()),
			),
		),

		// Create Reporter
		router.NewRoute(http.MethodPost, "/reporters", e.createReporter,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Create reporter"),
				router.WithDescription("Create a new reporter"),
				router.WithTags("Reporters"),
				router.WithRequestBody(&dto.ReporterCreateRequest{}, "Reporter details", true),
				router.WithSuccessResponse(
					http.StatusCreated,
					"Reporter created successfully",
					router.WithJSONContent(dto.ReporterResponse{}),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponse(http.StatusUnprocessableEntity, "Validation error"),
				),
			),
		),

		// Get Reporter
		router.NewRoute(http.MethodGet, "/reporters/:id", e.getReporter,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get reporter"),
				router.WithDescription("Retrieve a specific reporter by ID"),
				router.WithTags("Reporters"),
				router.WithPathParam("id", "Reporter ID", 1),
				router.WithSuccessResponse(
					http.StatusOK,
					"Reporter details",
					router.WithJSONContent(dto.ReporterResponse{}),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid reporter ID format"),
						router.DefineSwaggerErrorResponse(http.StatusNotFound, "Reporter not found"),
					),
				),
			),
		),
	)

	// Register routes with the router and access service
	// The base path for this extension is "/abuse", so routes like "/reporters" become "/abuse/reporters".
	// The access service registration needs the full path relative to the admin API root.
	// core.GetAPI("admin").Subdomain() is "admin", so the full path is "/admin/abuse" + route.Path
	return router.RegisterRoutes(gRouter, accessSvc, core.GetAPI("admin").Subdomain(), routes, router.WithCors())
}

// createReporter handles the creation of a new reporter
func (e *AdminExtension) createReporter(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.reporterService == nil {
		e.logger.Error("Reporter service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Validate the request using DTO and convert to model
	var requestDto dto.ReporterCreateRequest
	reporterModel, ok := httputil.DecodeAndValidateRequest[*models.Reporter, *dto.ReporterCreateRequest](ctx, &requestDto)
	if !ok {
		// httputil.DecodeAndValidateRequest handles the error response internally
		return nil // Return nil as the error is already handled
	}

	// Create the reporter using the service
	createdReporter, err := e.reporterService.Create(reporterModel)
	if err != nil {
		e.logger.Error("Failed to create reporter", zap.Error(err))
		return ctx.Error(errors.New("failed to create reporter"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Prepare and send response
	var responseDto dto.ReporterResponse
	c.Response().WriteHeader(http.StatusCreated) // Set status code using Echo context
	// Use httputil.EncodeResponse for encoding
	if err := httputil.EncodeResponse[*models.Reporter, *dto.ReporterResponse](ctx, createdReporter, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
		return err // httputil.EncodeResponse returns an error
	}

	return nil // Return nil on success
}

// getReporter retrieves a specific reporter by ID
func (e *AdminExtension) getReporter(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.reporterService == nil {
		e.logger.Error("Reporter service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Extract ID from path using Echo context
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid reporter ID format"), http.StatusBadRequest) // Use ctx.Error
	}

	// Get the reporter using the service
	reporter, err := e.reporterService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Error(errors.New("reporter not found"), http.StatusNotFound) // Use ctx.Error
		} else {
			e.logger.Error("Failed to fetch reporter", zap.Error(err))
			return ctx.Error(errors.New("failed to fetch reporter"), http.StatusInternalServerError) // Use ctx.Error
		}
	}

	// Prepare and send response
	var responseDto dto.ReporterResponse
	// Use httputil.EncodeResponse for encoding
	if err := httputil.EncodeResponse[*models.Reporter, *dto.ReporterResponse](ctx, reporter, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
		return err // httputil.EncodeResponse returns an error
	}

	return nil // Return nil on success
}

// listReporters returns a list of reporters with filtering and pagination
func (e *AdminExtension) listReporters(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.reporterService == nil {
		e.logger.Error("Reporter service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Handle list request processing using Echo context
	if err := queryutilHttp.ProcessListRequest(
		c.Response(), c.Request(), // Pass raw http.ResponseWriter and http.Request
		"reporters",
		func(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Reporter, int64, error) {
			return e.reporterService.List(filters, sorts, pagination)
		},
		func(r models.Reporter) dto.ReporterResponse {
			var response dto.ReporterResponse
			if err := response.FromModel(&r); err != nil {
				e.logger.Error("Failed to convert reporter", zap.Error(err))
				// In a real scenario, you might want to handle this conversion error more gracefully
				// or log it and return a generic error DTO. For this example, returning an empty DTO.
				return dto.ReporterResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list reporters", zap.Error(err))
		// queryutilHttp.ProcessListRequest handles writing the error response internally,
		// but we still return the error for Echo's error handler chain.
		return err // Return the error from ProcessListRequest
	}

	return nil // Return nil on success
}
