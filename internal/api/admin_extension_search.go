package api

import (
	"errors"
	"fmt"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"net/http"

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
)

// registerSearchHandlers registers search-related route handlers using portal-router.
func (e *AdminExtension) registerSearchHandlers(gRouter router.Router, accessSvc core.AccessService) error {
	caseSchema := queryutil.NewSchemaProvider().ForType(dto.CaseResponse{})

	routes := router.DefineRoutes(
		// Search Cases
		router.NewRoute(http.MethodGet, "/search/cases", e.searchCases,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithListEndpoint(
					"Search cases",
					"Search for abuse cases by query string with pagination and sorting",
					jwt.PurposeLogin, // Assuming admin endpoints use login JWT
					dto.CaseResponse{},
					caseSchema, // Use caseSchema for sorting/filtering parameters
					caseSchema.SortableFields(),
					nil, // No specific filter params defined, rely on schema
					router.WithFilterParamsFromSchema(caseSchema),
					router.WithGlobalSearchParam(), // Add global search param 'q'
					router.WithErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid request parameters"),
					),
				),
				router.WithSuccessResponse(http.StatusOK, "List of matching cases", router.WithTotalCountHeader()),
			),
		),

		// Global Search
		router.NewRoute(http.MethodGet, "/search/global", e.globalSearch,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Global search"),
				router.WithDescription("Search across multiple entities (cases, reporters) by query string with pagination"),
				router.WithTags("Search"),
				router.WithGlobalSearchParam(), // Add global search param 'q'
				router.WithPaginationParams(),  // Add pagination params
				router.WithSuccessResponse(
					http.StatusOK,
					"Global search results",
					router.WithJSONContent(dto.GlobalSearchResponse{}),
					router.WithTotalCountHeader(), // Add total count header
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid request parameters"),
				),
			),
		),
	)

	// Register routes with the router and access service
	// The base path for this extension is "/abuse", so routes like "/search/cases" become "/abuse/search/cases".
	// The access service registration needs the full path relative to the admin API root.
	// core.GetAPI("admin").Subdomain() is "admin", so the full path is "/admin/abuse" + route.Path
	return router.RegisterRoutes(gRouter, accessSvc, core.GetAPI("admin").Subdomain(), routes, router.WithCors())
}

// searchCases handles advanced search for cases
func (e *AdminExtension) searchCases(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.searchService == nil {
		e.logger.Error("Search service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Get query string from Echo context
	query := c.QueryParam("q")

	// Use standardized list processing with Echo context
	if err := queryutilHttp.ProcessListRequest(
		c.Response(), c.Request(), // Pass raw http.ResponseWriter and http.Request
		"cases",
		func(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Case, int64, error) {
			// Pass the query string, filters, and pagination to the service
			return e.searchService.SearchCases(c.Request().Context(), query, filters, pagination)
		},
		func(c models.Case) dto.CaseResponse {
			var response dto.CaseResponse
			if err := response.FromModel(&c); err != nil {
				e.logger.Error("Failed to convert case", zap.Error(err))
				// In a real scenario, you might want to handle this conversion error more gracefully
				// or log it and return a generic error DTO. For this example, returning an empty DTO.
				return dto.CaseResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to search cases", zap.Error(err))
		// queryutilHttp.ProcessListRequest handles writing the error response internally,
		// but we still return the error for Echo's error handler chain.
		return err // Return the error from ProcessListRequest
	}

	return nil // Return nil on success
}

// globalSearch performs a search across multiple entities
func (e *AdminExtension) globalSearch(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Get query string from Echo context
	query := c.QueryParam("q")

	// Get pagination from Echo context using queryutilHttp
	_, _, pagination, err := queryutil.ParseRequest(c.Request())
	if err != nil {
		return ctx.Error(fmt.Errorf("invalid pagination parameters: %w", err), http.StatusBadRequest) // Use ctx.Error
	}

	// Use injected service
	if e.searchService == nil {
		e.logger.Error("Search service unavailable")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Execute search
	result, err := e.searchService.GlobalSearch(c.Request().Context(), query, pagination)
	if err != nil {
		e.logger.Error("Global search failed", zap.Error(err))
		return ctx.Error(fmt.Errorf("search failed: %w", err), http.StatusInternalServerError) // Use ctx.Error
	}

	// Convert to DTOs
	var responseDto dto.GlobalSearchResponse
	// Use httputil.EncodeResponse with the source model and target DTO
	if err := httputil.EncodeResponse[*typesSvc.GlobalSearchResult, *dto.GlobalSearchResponse](ctx, result, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
		return err // httputil.EncodeResponse returns an error
	}

	// Set Content-Range header manually if needed, although ProcessListRequest usually handles this.
	// If GlobalSearch doesn't return total count in the result struct, you might need to fetch it separately
	// or modify the service to return it. Assuming GlobalSearchResult includes TotalCount.
	// c.Response().Header().Set("Content-Range",
	// 	fmt.Sprintf("items %d-%d/%d",
	// 		pagination.Start,
	// 		pagination.End-1, // End is exclusive in queryutil
	// 		responseDto.TotalCount, // Assuming TotalCount is in the DTO
	// 	))

	return nil // Return nil on success
}
