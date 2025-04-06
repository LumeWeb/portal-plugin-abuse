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

// registerCommunicationHandlers registers all communication-related route handlers using portal-router.
func (e *AdminExtension) registerCommunicationHandlers(gRouter router.Router, accessSvc core.AccessService) error {
	schema := queryutil.NewSchemaProvider().ForType(dto.CommunicationResponse{})

	routes := router.DefineRoutes(
		// List Case Communications
		router.NewRoute(http.MethodGet, "/cases/:id/communications", e.listCaseCommunications,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithListEndpoint(
					"List case communications",
					"Retrieve a list of communications for a specific case",
					jwt.PurposeLogin, // Assuming admin endpoints use login JWT
					dto.CommunicationResponse{},
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
				router.WithSuccessResponse(http.StatusOK, "List of communications", router.WithTotalCountHeader()),
			),
		),

		// Add Case Communication
		router.NewRoute(http.MethodPost, "/cases/:id/communications", e.addCaseCommunication,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Add case communication"),
				router.WithDescription("Add a new communication to a case"),
				router.WithTags("Communications"),
				router.WithPathParam("id", "Case ID", 1),
				router.WithRequestBody(&dto.CommunicationCreateRequest{}, "Communication details", true),
				router.WithSuccessResponse(
					http.StatusCreated,
					"Communication added successfully",
					router.WithJSONContent(dto.CommunicationResponse{}),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid request payload"),
						router.DefineSwaggerErrorResponse(http.StatusNotFound, "Case not found"),
						router.DefineSwaggerErrorResponse(http.StatusUnprocessableEntity, "Validation Error"),
					),
				),
			),
		),
	)

	// Register routes with the router and access service
	// The base path for this extension is "/abuse", so routes like "/cases/:id/communications" become "/abuse/cases/:id/communications".
	// The access service registration needs the full path relative to the admin API root.
	// core.GetAPI("admin").Subdomain() is "admin", so the full path is "/admin/abuse" + route.Path
	return router.RegisterRoutes(gRouter, accessSvc, core.GetAPI("admin").Subdomain(), routes, router.WithCors())
}

// listCaseCommunications returns all communications for a case
func (e *AdminExtension) listCaseCommunications(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Get case ID from path using Echo context
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid case ID format"), http.StatusBadRequest) // Use ctx.Error
	}

	// Use injected service
	if e.commService == nil {
		e.logger.Error("Communication service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Handle list request processing using Echo context
	if err := queryutilHttp.ProcessListRequest(
		c.Response(), c.Request(), // Pass raw http.ResponseWriter and http.Request
		"communications",
		func(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Communication, int64, error) {
			// The service method ListByCaseID is expected to handle applying the case ID filter
			// along with the filters parsed from the request.
			return e.commService.ListByCaseID(uint(id), filters, sorts, pagination)
		},
		func(comm models.Communication) dto.CommunicationResponse {
			var response dto.CommunicationResponse
			if err := response.FromModel(&comm); err != nil {
				e.logger.Error("Failed to convert communication", zap.Error(err))
				// In a real scenario, you might want to handle this conversion error more gracefully
				// or log it and return a generic error DTO. For this example, returning an empty DTO.
				return dto.CommunicationResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list communications", zap.Error(err))
		// queryutilHttp.ProcessListRequest handles writing the error response internally,
		// but we still return the error for Echo's error handler chain.
		return err // Return the error from ProcessListRequest
	}

	return nil // Return nil on success
}

// addCaseCommunication adds a communication to a case
func (e *AdminExtension) addCaseCommunication(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected services
	if e.caseService == nil || e.commService == nil {
		e.logger.Error("Required service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Get case ID from path using Echo context
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid case ID format"), http.StatusBadRequest) // Use ctx.Error
	}

	// Validate the request using DTO and convert to model
	var requestDto dto.CommunicationCreateRequest
	commModel, ok := httputil.DecodeAndValidateRequest[*models.Communication, *dto.CommunicationCreateRequest](ctx, &requestDto)
	if !ok {
		// httputil.DecodeAndValidateRequest handles the error response internally
		return nil // Return nil as the error is already handled
	}

	// Verify case exists
	if _, err := e.caseService.GetByID(uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Error(errors.New("case not found"), http.StatusNotFound) // Use ctx.Error
		} else {
			e.logger.Error("Failed to fetch case", zap.Error(err))
			return ctx.Error(errors.New("failed to fetch case"), http.StatusInternalServerError) // Use ctx.Error
		}
	}

	// Set case ID and sender (placeholder until auth implemented)
	commModel.CaseID = uint(id)
	commModel.SenderID = 1 // TODO: Replace with actual user ID from auth context

	// Create the communication using the service
	createdComm, err := e.commService.Create(commModel)
	if err != nil {
		e.logger.Error("Failed to create communication", zap.Error(err))
		return ctx.Error(errors.New("failed to create communication"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Prepare and send response
	var responseDto dto.CommunicationResponse
	c.Response().WriteHeader(http.StatusCreated) // Set status code using Echo context
	// Use httputil.EncodeResponse for encoding
	if err := httputil.EncodeResponse[*models.Communication, *dto.CommunicationResponse](ctx, createdComm, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
		return err // httputil.EncodeResponse returns an error
	}

	return nil // Return nil on success
}
