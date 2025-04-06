package api

import (
	"fmt"
	"github.com/samber/lo"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"net/http"

	"github.com/labstack/echo/v4"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-router"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"go.uber.org/zap"
)

func (e *AdminExtension) registerBlocklistAnalyticsHandlers(gRouter router.Router, accessSvc core.AccessService) error {
	routes := router.DefineRoutes(
		router.NewRoute(http.MethodGet, "/analytics/blocklist/block-reasons", e.getBlockReasons,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get block reason counts"),
				router.WithDescription("Retrieve a breakdown of block counts by block reason over a specified time range."),
				router.WithTags("Analytics"),
				router.WithQueryParam("time_range", "Time range", map[string]interface{}{
					"type": "string",
					"enum": []string{"7d", "30d", "90d", "24h"},
				}),
				router.WithSuccessResponse(
					http.StatusOK,
					"Block reason counts",
					router.WithJSONContent(dto.BlockReasonListResponse{}),
				),
				router.WithErrorResponses(router.DefineSwaggerErrorResponses(
					router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid parameters"),
					router.DefineSwaggerErrorResponse(http.StatusInternalServerError, "Failed to retrieve block reason counts"),
				)),
			),
		),
	)

	return router.RegisterRoutes(gRouter, accessSvc, core.GetAPI("admin").Subdomain(), routes, router.WithCors())
}

func (e *AdminExtension) getBlockReasons(c echo.Context) error {
	ctx := httputil.Context(c)

	// Parse all filters from request
	filters, _, _, err := queryutil.ParseRequest(c.Request())
	if err != nil {
		return ctx.Error(fmt.Errorf("invalid filters: %w", err), http.StatusBadRequest)
	}

	// Extract time range from filters
	timeRangeFilter := queryutil.FindFilter(filters, "time_range")
	if timeRangeFilter != nil {
		// Remove the time_range filter since we handle it separately
		filters = lo.Filter(filters, func(f queryutil.CrudFilter, _ int) bool {
			return f.GetField() != "time_range"
		})
	}

	// Call the service to get the block reason counts
	counts, err := e.blockListService.GetBlockReasonCounts(filters)
	if err != nil {
		e.logger.Error("Failed to get block reason counts", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to retrieve block reason counts: %w", err), http.StatusInternalServerError)
	}

	// Convert the service data to DTO
	var responseDto dto.BlockReasonListResponse

	// Use httputil.EncodeResponse for encoding
	if err := httputil.EncodeResponse[[]typesSvc.BlockReasonCount, *dto.BlockReasonListResponse](ctx, counts, &responseDto); err != nil {
		e.logger.Error("Failed to encode block reason counts response", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to process response"), http.StatusInternalServerError)
	}

	return nil
}
