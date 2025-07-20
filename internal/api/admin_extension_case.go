package api

import (
	"errors"
	"go.lumeweb.com/portal-middleware/auth/jwt"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-router"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	queryutilHttp "go.lumeweb.com/queryutil/http"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// registerCaseHandlers registers all case-related route handlers using portal-router.
func (e *AdminExtension) registerCaseHandlers(gRouter router.Router, accessSvc core.AccessService) error {
	schema := queryutil.NewSchemaProvider().ForType(dto.CaseStatusResponse{})

	routes := router.DefineRoutes(
		// List Cases
		router.NewRoute(http.MethodGet, "/cases", e.listCases,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithListEndpoint(
					"List Cases",
					"Retrieves a list of abuse cases with filtering and pagination",
					jwt.PurposeLogin,
					dto.CaseResponse{},
					schema,
					schema.SortableFields(),
					nil,
					router.WithFilterParamsFromSchema(schema),
					router.WithErrorResponses( // Use this wrapper
						router.DefineSwaggerErrorResponses(
							router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid filter parameters"),
						),
					),
				),
			),
		),

		// Create Case
		router.NewRoute(http.MethodPost, "/cases", e.createCase,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Create Case"),
				router.WithDescription("Creates a new abuse case"),
				router.WithTags("Cases"),
				router.WithRequestBody(&dto.CreateCaseRequest{}, "Case details", true),
				router.WithSuccessResponse(
					http.StatusCreated,
					"Case created successfully",
					router.WithJSONContent(dto.CaseResponse{}),
					router.WithHeader("Location", "URL of the newly created case"),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponse(http.StatusUnprocessableEntity, "Validation Error"),
				),
			),
		),

		// Get Case
		router.NewRoute(http.MethodGet, "/cases/:id", e.getCase,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get Case"),
				router.WithDescription("Retrieves a specific abuse case by ID"),
				router.WithTags("Cases"),
				router.WithPathParam("id", "Case ID", 1),
				router.WithSuccessResponse(
					http.StatusOK,
					"Case details",
					router.WithJSONContent(dto.CaseResponse{}),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponse(http.StatusNotFound, "Case not found"),
				),
			),
		),

		// Update Case
		router.NewRoute(http.MethodPatch, "/cases/:id", e.updateCase,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Update Case"),
				router.WithDescription("Updates an existing abuse case by ID"),
				router.WithTags("Cases"),
				router.WithPathParam("id", "Case ID", 1),
				router.WithRequestBody(&dto.UpdateCaseRequest{}, "Updated case details", true),
				router.WithSuccessResponse(
					http.StatusOK,
					"Case updated successfully",
					router.WithJSONContent(dto.CaseResponse{}),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusNotFound, "Case not found"),
						router.DefineSwaggerErrorResponse(http.StatusUnprocessableEntity, "Validation Error"),
					),
				),
			),
		),
	)

	// Register routes with the router and access service
	// The path registered with accessSvc should be the full path including the subdomain prefix if applicable.
	// Assuming the router is already grouped under the subdomain, the path here is relative to the group.
	return router.RegisterRoutes(gRouter, accessSvc, core.GetAPI("admin").Subdomain(), routes, router.WithCors())
}

// createCase handles the creation of a new abuse case
func (e *AdminExtension) createCase(c echo.Context) error {
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.caseService == nil {
		e.logger.Error("Case service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError)
	}

	// Validate the request using DTO and convert to model
	// Preserving the original pattern if it used httputil helpers for this
	var requestDto dto.CreateCaseRequest
	caseModel, ok := httputil.DecodeAndValidateRequest[*models.Case, *dto.CreateCaseRequest](ctx, &requestDto)
	if !ok {
		// httputil.DecodeAndValidateRequest handles the error response internally
		return nil // Return nil as the error is already handled
	}

	// Create the case using the service
	createdCase, err := e.caseService.Create(caseModel)
	if err != nil {
		e.logger.Error("Failed to create case", zap.Error(err))
		return ctx.Error(errors.New("failed to create case"), http.StatusInternalServerError)
	}

	// Prepare and send response
	var responseDto dto.CaseResponse

	c.Response().Before(func() {
		c.Response().Status = http.StatusCreated
	})

	if err = httputil.EncodeResponse(ctx, createdCase, &responseDto); err != nil {
		e.logger.Error("Failed to convert case to DTO", zap.Error(err))
	}

	return nil
}

// getCase retrieves a specific case by ID
func (e *AdminExtension) getCase(c echo.Context) error {
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.caseService == nil {
		e.logger.Error("Case service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError)
	}

	// Extract ID from path using Echo context
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid case ID format"), http.StatusBadRequest)
	}

	// Get the case using the service
	caseModel, err := e.caseService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Error(errors.New("case not found"), http.StatusNotFound)
		} else {
			e.logger.Error("Failed to fetch case", zap.Error(err))
			return ctx.Error(errors.New("failed to fetch case"), http.StatusInternalServerError)
		}
	}

	// Prepare and send response
	var responseDto dto.CaseResponse

	if err = httputil.EncodeResponse(ctx, caseModel, &responseDto); err != nil {
		e.logger.Error("Failed to convert case to DTO", zap.Error(err))
	}

	return nil
}

// listCases returns a list of cases with filtering and pagination
func (e *AdminExtension) listCases(c echo.Context) error {
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.caseService == nil {
		e.logger.Error("Case service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError)
	}

	// Use queryutilHttp.ProcessListRequest with Echo context
	if err := queryutilHttp.ProcessListRequest(
		c.Response(), c.Request(), // Pass raw http.ResponseWriter and http.Request
		"cases",
		func(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Case, int64, error) {
			return e.caseService.List(filters, sorts, pagination)
		},
		func(c models.Case) dto.CaseResponse {
			var response dto.CaseResponse
			err := response.FromModel(&c)
			if err != nil {
				e.logger.Error("Failed to convert case", zap.Error(err))
				// In a real scenario, you might want to handle this conversion error more gracefully
				// or log it and return a generic error DTO. For this example, returning an empty DTO.
				return dto.CaseResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list cases", zap.Error(err))
		// queryutilHttp.ProcessListRequest handles writing the error response internally,
		// but we still return the error for Echo's error handler chain.
		return err // Return the error from ProcessListRequest
	}

	return nil // Return nil on success
}

// updateCase handles updates to an existing case
func (e *AdminExtension) updateCase(c echo.Context) error {
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.caseService == nil {
		e.logger.Error("Case service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError)
	}

	// Extract ID from path using Echo context
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid case ID format"), http.StatusBadRequest)
	}

	// Fetch existing case
	existingCase, err := e.caseService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Error(errors.New("case not found"), http.StatusNotFound)
		} else {
			e.logger.Error("Failed to fetch case", zap.Error(err))
			return ctx.Error(errors.New("failed to fetch case"), http.StatusInternalServerError)
		}
	}

	// Validate the request using DTO and update the model
	var requestDto dto.UpdateCaseRequest
	updatedCase, ok := httputil.DecodeAndValidateRequest[*models.Case, *dto.UpdateCaseRequest](ctx, &requestDto)
	if !ok {
		// httputil.DecodeAndValidateRequest handles the error response internally
		return nil // Return nil as the error is already handled
	}

	// Apply each updated field from the DTO to the existing case while preserving existing data
	if updatedCase != nil {
		if requestDto.Description != nil {
			existingCase.Description = *requestDto.Description
		}
		if requestDto.Type != nil {
			existingCase.Type = *requestDto.Type
		}
		if requestDto.Priority != nil {
			existingCase.Priority = *requestDto.Priority
		}
		if requestDto.Source != nil {
			existingCase.Source = *requestDto.Source
		}
		if requestDto.Status != nil {
			existingCase.Status = *requestDto.Status
		}
		if requestDto.NeedsReview != nil {
			existingCase.NeedsReview = *requestDto.NeedsReview
		}
		if requestDto.ReporterID != nil {
			existingCase.ReporterID = uint(*requestDto.ReporterID)
		}
		if requestDto.SubjectID != nil {
			existingCase.SubjectID = uint(*requestDto.SubjectID)
		}
		if requestDto.Status != nil {
			existingCase.Status = *requestDto.Status
		}
	}

	// Update the case
	if err = e.caseService.Update(existingCase); err != nil {
		e.logger.Error("Failed to update case", zap.Error(err))
		return ctx.Error(errors.New("failed to update case"), http.StatusInternalServerError)
	}

	// Prepare and send response
	var responseDto dto.CaseResponse

	if err = httputil.EncodeResponse(ctx, existingCase, &responseDto); err != nil {
		e.logger.Error("Failed to convert case to DTO", zap.Error(err))
	}

	return nil
}
