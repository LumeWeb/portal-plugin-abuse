package api

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/util"
	"go.lumeweb.com/portal-router"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"go.uber.org/zap"
	"net/http"
)

func (e *AdminExtension) registerCommunicationAnalyticsHandlers(gRouter router.Router, accessSvc core.AccessService) error {
	routes := router.DefineRoutes(
		router.NewRoute(http.MethodGet, "/analytics/communications/timeline", e.getCommunicationsTimeline,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get communication timeline"),
				router.WithDescription("Retrieve a timeline of communication counts per hour over a specified time range."),
				router.WithTags("Analytics"),
				router.WithQueryParam("time_range", "Time range", map[string]interface{}{
					"type": "string",
					"enum": []string{"7d", "30d", "90d", "24h"},
				}),
				router.WithSuccessResponse(
					http.StatusOK,
					"Communication timeline data",
					router.WithJSONContent(dto.CommunicationTimelineResponse{}),
				),
				router.WithErrorResponses(router.DefineSwaggerErrorResponses(
					router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid parameters"),
					router.DefineSwaggerErrorResponse(http.StatusInternalServerError, "Failed to retrieve timeline"),
				)),
			),
		),
	)

	return router.RegisterRoutes(gRouter, accessSvc, core.GetAPI("admin").Subdomain(), routes, router.WithCors())
}

func (e *AdminExtension) getCommunicationsTimeline(c echo.Context) error {
	ctx := httputil.Context(c)

	// Parse all filters from request
	filters, _, _, err := queryutil.ParseRequest(c.Request())
	if err != nil {
		return ctx.Error(fmt.Errorf("invalid filters: %w", err), http.StatusBadRequest)
	}

	// Extract time_range from filters
	timeRangeFilter := queryutil.FindFilter(filters, "time_range")
	if timeRangeFilter == nil {
		return ctx.Error(util.ErrInvalidTimeRange, http.StatusBadRequest)
	}

	timeRange, ok := timeRangeFilter.GetValue().(string)
	if !ok || timeRange == "" {
		return ctx.Error(util.ErrInvalidTimeRange, http.StatusBadRequest)
	}

	// Call the service to get the timeline data
	timelineData, err := e.commService.GetCommunicationTimeline(timeRange, filters)
	if err != nil {
		e.logger.Error("Failed to get communication timeline", zap.Error(err), zap.String("timeRange", timeRange))
		return ctx.Error(fmt.Errorf("failed to retrieve communication timeline: %w", err), http.StatusBadRequest)
	}

	// Convert and encode the response using httputil helper
	response := &dto.CommunicationTimelineResponse{}
	if err := httputil.EncodeResponse[[]models.CommunicationHourlyCount, *dto.CommunicationTimelineResponse](
		ctx,
		timelineData,
		response,
	); err != nil {
		e.logger.Error("Failed to encode communication timeline response", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to process response"), http.StatusInternalServerError)
	}

	return nil
}
