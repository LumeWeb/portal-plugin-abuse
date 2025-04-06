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

// registerSubjectHandlers registers all subject-related route handlers using portal-router.
func (e *AdminExtension) registerSubjectHandlers(gRouter router.Router, accessSvc core.AccessService) error {
	schema := queryutil.NewSchemaProvider().ForType(dto.SubjectResponse{})

	routes := router.DefineRoutes(
		// List Subjects
		router.NewRoute(http.MethodGet, "/subjects", e.listSubjects,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithListEndpoint(
					"List subjects",
					"Retrieve a list of subjects with optional filtering and pagination",
					jwt.PurposeLogin, // Assuming admin endpoints use login JWT
					dto.SubjectResponse{},
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
				router.WithSuccessResponse(http.StatusOK, "List of subjects", router.WithTotalCountHeader()),
			),
		),

		// Create Subject
		router.NewRoute(http.MethodPost, "/subjects", e.createSubject,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Create subject"),
				router.WithDescription("Create a new subject"),
				router.WithTags("Subjects"),
				router.WithRequestBody(&dto.SubjectCreateRequest{}, "Subject details", true),
				router.WithSuccessResponse(
					http.StatusCreated,
					"Subject created successfully",
					router.WithJSONContent(dto.SubjectResponse{}),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusUnprocessableEntity, "Validation error"),
					),
				),
			),
		),

		// Get Subject
		router.NewRoute(http.MethodGet, "/subjects/:id", e.getSubject,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get subject"),
				router.WithDescription("Retrieve a specific subject by ID"),
				router.WithTags("Subjects"),
				router.WithPathParam("id", "Subject ID", 1),
				router.WithSuccessResponse(
					http.StatusOK,
					"Subject details",
					router.WithJSONContent(dto.SubjectResponse{}),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid subject ID format"),
						router.DefineSwaggerErrorResponse(http.StatusNotFound, "Subject not found"),
					),
				),
			),
		),
	)

	// Register routes with the router and access service
	// The base path for this extension is "/abuse", so routes like "/subjects" become "/abuse/subjects".
	// The access service registration needs the full path relative to the admin API root.
	// core.GetAPI("admin").Subdomain() is "admin", so the full path is "/admin/abuse" + route.Path
	return router.RegisterRoutes(gRouter, accessSvc, core.GetAPI("admin").Subdomain(), routes, router.WithCors())
}

// createSubject handles the creation of a new subject
func (e *AdminExtension) createSubject(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.subjectService == nil {
		e.logger.Error("Subject service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Validate the request using DTO and convert to model
	var requestDto dto.SubjectCreateRequest
	subjectModel, ok := httputil.DecodeAndValidateRequest[*models.Subject, *dto.SubjectCreateRequest](ctx, &requestDto)
	if !ok {
		// httputil.DecodeAndValidateRequest handles the error response internally
		return nil // Return nil as the error is already handled
	}

	// Create the subject using the service
	createdSubject, err := e.subjectService.Create(subjectModel)
	if err != nil {
		e.logger.Error("Failed to create subject", zap.Error(err))
		return ctx.Error(errors.New("failed to create subject"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Prepare and send response
	var responseDto dto.SubjectResponse
	c.Response().WriteHeader(http.StatusCreated) // Set status code using Echo context
	// Use httputil.EncodeResponse for encoding
	if err := httputil.EncodeResponse[*models.Subject, *dto.SubjectResponse](ctx, createdSubject, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
		return err // httputil.EncodeResponse returns an error
	}

	return nil // Return nil on success
}

// getSubject retrieves a specific subject by ID
func (e *AdminExtension) getSubject(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.subjectService == nil {
		e.logger.Error("Subject service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Extract ID from path using Echo context
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid subject ID format"), http.StatusBadRequest) // Use ctx.Error
	}

	// Get the subject using the service
	subject, err := e.subjectService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Error(errors.New("subject not found"), http.StatusNotFound) // Use ctx.Error
		} else {
			e.logger.Error("Failed to fetch subject", zap.Error(err))
			return ctx.Error(errors.New("failed to fetch subject"), http.StatusInternalServerError) // Use ctx.Error
		}
	}

	// Prepare and send response
	var responseDto dto.SubjectResponse
	// Use httputil.EncodeResponse for encoding
	if err := httputil.EncodeResponse[*models.Subject, *dto.SubjectResponse](ctx, subject, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
		return err // httputil.EncodeResponse returns an error
	}

	return nil // Return nil on success
}

// listSubjects returns a list of subjects with filtering and pagination
func (e *AdminExtension) listSubjects(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.subjectService == nil {
		e.logger.Error("Subject service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Handle list request processing using Echo context
	if err := queryutilHttp.ProcessListRequest(
		c.Response(), c.Request(), // Pass raw http.ResponseWriter and http.Request
		"subjects",
		func(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Subject, int64, error) {
			return e.subjectService.List(filters, sorts, pagination)
		},
		func(s models.Subject) dto.SubjectResponse {
			var response dto.SubjectResponse
			if err := response.FromModel(&s); err != nil {
				e.logger.Error("Failed to convert subject", zap.Error(err))
				// In a real scenario, you might want to handle this conversion error more gracefully
				// or log it and return a generic error DTO. For this example, returning an empty DTO.
				return dto.SubjectResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list subjects", zap.Error(err))
		// queryutilHttp.ProcessListRequest handles writing the error response internally,
		// but we still return the error for Echo's error handler chain.
		return err // Return the error from ProcessListRequest
	}

	return nil // Return nil on success
}
