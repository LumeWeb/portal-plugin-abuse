package api

import (
	"fmt"
	"go.lumeweb.com/portal-plugin-abuse/internal/util"
	"go.lumeweb.com/queryutil"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	pluginDb "go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal-router"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
)

// generateDateRangeFilters generates date range filters for the given start and end dates.
func generateDateRangeFilters(startDate, endDate time.Time) []queryutil.CrudFilter {
	return []queryutil.CrudFilter{
		queryutil.And(
			queryutil.FieldGte("transition_date", startDate),
			queryutil.FieldLte("transition_date", endDate),
		),
	}
}

func (e *AdminExtension) registerAnalyticsHandlers(gRouter router.Router, accessSvc core.AccessService) error {
	routes := router.DefineRoutes(
		router.NewRoute(http.MethodGet, "/analytics/cases", e.handleCaseAnalytics,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get case analytics"),
				router.WithDescription("Retrieve overall case analytics with optional filtering"),
				router.WithTags("Analytics"),
				router.WithPaginationParams(), // Add pagination params if needed for analytics endpoints
				router.WithSuccessResponse(
					http.StatusOK,
					"Case analytics",
					router.WithJSONContent(dto.CaseAnalyticsResponse{}),
				),
			),
		),
		router.NewRoute(http.MethodGet, "/analytics/cases/7d", e.handle7DayCaseAnalytics,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get 7-day case analytics"),
				router.WithDescription("Retrieve case analytics for the last 7 days"),
				router.WithTags("Analytics"),
				router.WithSuccessResponse(
					http.StatusOK,
					"Case analytics for the last 7 days",
					router.WithJSONContent(dto.CaseAnalyticsResponse{}),
				),
			),
		),
		router.NewRoute(http.MethodGet, "/analytics/cases/30d", e.handle30DayCaseAnalytics,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get 30-day case analytics"),
				router.WithDescription("Retrieve case analytics for the last 30 days"),
				router.WithTags("Analytics"),
				router.WithSuccessResponse(
					http.StatusOK,
					"Case analytics for the last 30 days",
					router.WithJSONContent(dto.CaseAnalyticsResponse{}),
				),
			),
		),
		router.NewRoute(http.MethodGet, "/analytics/cases/24h", e.handle24HourCaseAnalytics,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get 24-hour case analytics"),
				router.WithDescription("Retrieve case analytics for the last 24 hours"),
				router.WithTags("Analytics"),
				router.WithSuccessResponse(
					http.StatusOK,
					"Case analytics for the last 24 hours",
					router.WithJSONContent(dto.CaseAnalyticsResponse{}),
				),
			),
		),
		router.NewRoute(http.MethodGet, "/analytics/cases/status-flow", e.getStatusFlow,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get case status flow"),
				router.WithDescription("Retrieve status transition data for visualization"),
				router.WithTags("Analytics"),
				router.WithQueryParam("time_range", "Time range", map[string]interface{}{
					"type": "string",
					"enum": []string{"7d", "30d", "90d"},
				}),
				router.WithSuccessResponse(
					http.StatusOK,
					"Status flow data",
					router.WithJSONContent(typesSvc.StatusFlowGraph{}),
				),
			),
		),
		router.NewRoute(http.MethodGet, "/analytics/cases/type-source-matrix", e.getTypeSourceMatrix,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get case type and source matrix"),
				router.WithDescription("Retrieve a breakdown of case counts by case type and report source over a specified time range."),
				router.WithTags("Analytics"),
				router.WithQueryParam("time_range", "Time range", map[string]interface{}{
					"type": "string",
					"enum": []string{"7d", "30d", "90d", "24h"},
				}),
				router.WithSuccessResponse(
					http.StatusOK,
					"Case type and source matrix data",
					router.WithJSONContent(dto.CaseTypeSourceMatrixResponse{}),
				),
				router.WithErrorResponses(router.DefineSwaggerErrorResponses(
					router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid parameters"),
					router.DefineSwaggerErrorResponse(http.StatusInternalServerError, "Failed to retrieve matrix"),
				)),
			),
		),
		router.NewRoute(http.MethodGet, "/analytics/cases/time-series", e.handleCaseTimeSeriesAnalytics,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get case time-series metrics"),
				router.WithDescription("Retrieve time-series metrics for cases (open, new, resolved) with optional filtering"),
				router.WithTags("Analytics"),
				router.WithQueryParam("metric", "Metric to retrieve", map[string]interface{}{
					"type": "string",
					"enum": []string{"open_cases", "new_cases", "resolved_cases"},
				}),
				router.WithQueryParam("time_range", "Time range", map[string]interface{}{
					"type": "string",
					"enum": []string{"7d", "30d", "90d"},
				}),
				router.WithQueryParam("case_type", "Filter by case type", pluginDb.ValidCaseTypes),
				router.WithQueryParam("priority", "Filter by priority", pluginDb.ValidCasePriorities),
				router.WithSuccessResponse(
					http.StatusOK,
					"Time-series metrics data",
					router.WithJSONContent([]int64{}),
				),
				router.WithErrorResponses(router.DefineSwaggerErrorResponses(
					router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid parameters"),
					router.DefineSwaggerErrorResponse(http.StatusInternalServerError, "Failed to retrieve metrics"),
				)),
			),
		),
	)

	// Register routes with the router and access service
	// The base path for this extension is "/abuse", so routes like "/analytics/cases" become "/abuse/analytics/cases".
	// The access service registration needs the full path relative to the admin API root.
	// core.GetAPI("admin").Subdomain() is "admin", so the full path is "/admin/abuse" + route.Path
	return router.RegisterRoutes(gRouter, accessSvc, core.GetAPI("admin").Subdomain(), routes, router.WithCors())
}

func (e *AdminExtension) getStatusFlow(c echo.Context) error {
	ctx := httputil.Context(c)
	// Extract filters from query params
	filters, _, _, err := queryutil.ParseRequest(c.Request())
	if err != nil {
		return c.JSON(http.StatusBadRequest, router.AsErrorResponse(err))
	}

	// Extract time range from filters using queryutil helpers
	var startDate, endDate time.Time
	if gteFilter := queryutil.FindFilterWithOperator(filters, "transition_date", queryutil.OpGte); gteFilter != nil {
		if t, ok := gteFilter.GetValue().(time.Time); ok {
			startDate = t
		}
	}
	if lteFilter := queryutil.FindFilterWithOperator(filters, "transition_date", queryutil.OpLte); lteFilter != nil {
		if t, ok := lteFilter.GetValue().(time.Time); ok {
			endDate = t
		}
	}

	// Default to last 30 days if no date range specified
	if startDate.IsZero() && endDate.IsZero() {
		endDate = time.Now()
		startDate = endDate.AddDate(0, 0, -30)

		// If no date range is provided, create default filters
		// If no date range is provided, create default filters and append them
		filters = append(filters, generateDateRangeFilters(startDate, endDate)...)
	} else if startDate.IsZero() {
		startDate = endDate.AddDate(0, 0, -30)
		filters = append(filters, generateDateRangeFilters(startDate, endDate)...)
	} else if endDate.IsZero() {
		endDate = startDate.AddDate(0, 0, 30)
		filters = append(filters, generateDateRangeFilters(startDate, endDate)...)
	}

	// Validate date range
	if startDate.After(endDate) {
		return c.JSON(http.StatusBadRequest, router.AsErrorResponse(fmt.Errorf("Start date cannot be after end date")))
	}

	flowData, err := e.caseService.GetStatusFlowData(filters)
	if err != nil {
		e.logger.Error("Failed to get status flow data",
			zap.Error(err),
			zap.Time("startDate", startDate),
			zap.Time("endDate", endDate))
		return ctx.Error(fmt.Errorf("failed to retrieve status flow data"), http.StatusInternalServerError)
	}

	var responseDto dto.StatusFlowGraphResponse
	if err := httputil.EncodeResponse[*typesSvc.StatusFlowGraph, *dto.StatusFlowGraphResponse](ctx, flowData, &responseDto); err != nil {
		e.logger.Error("Failed to encode status flow response", zap.Error(err))
		return err
	}

	return nil
}

func (e *AdminExtension) handleCaseAnalytics(c echo.Context) error {
	ctx := httputil.Context(c)

	filters, _, _, err := queryutil.ParseRequest(c.Request())
	if err != nil {
		return ctx.Error(fmt.Errorf("invalid filters: %w", err), http.StatusBadRequest)
	}

	analytics, err := e.caseService.GetAnalytics(filters)
	if err != nil {
		e.logger.Error("Failed to get case analytics", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to generate analytics: %w", err), http.StatusInternalServerError)
	}

	var responseDto dto.CaseAnalyticsResponse
	// Use httputil.EncodeResponse with the source model and target DTO
	if err := httputil.EncodeResponse[*typesSvc.CaseAnalytics, *dto.CaseAnalyticsResponse](ctx, analytics, &responseDto); err != nil {
		e.logger.Error("Failed to encode analytics response", zap.Error(err))
		return err // httputil.EncodeResponse returns an error
	}

	return nil // Return nil on success
}

func (e *AdminExtension) handle7DayCaseAnalytics(c echo.Context) error {
	ctx := httputil.Context(c)

	analytics, err := e.caseService.Get7DayAnalytics()
	if err != nil {
		e.logger.Error("Failed to get 7-day analytics", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to generate analytics: %w", err), http.StatusInternalServerError)
	}

	var responseDto dto.CaseAnalyticsResponse
	if err := httputil.EncodeResponse[*typesSvc.CaseAnalytics, *dto.CaseAnalyticsResponse](ctx, analytics, &responseDto); err != nil {
		e.logger.Error("Failed to encode analytics response", zap.Error(err))
		return err
	}

	return nil
}

func (e *AdminExtension) handle30DayCaseAnalytics(c echo.Context) error {
	ctx := httputil.Context(c)

	analytics, err := e.caseService.Get30DayAnalytics()
	if err != nil {
		e.logger.Error("Failed to get 30-day analytics", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to generate analytics: %w", err), http.StatusInternalServerError)
	}

	var responseDto dto.CaseAnalyticsResponse
	if err := httputil.EncodeResponse[*typesSvc.CaseAnalytics, *dto.CaseAnalyticsResponse](ctx, analytics, &responseDto); err != nil {
		e.logger.Error("Failed to encode analytics response", zap.Error(err))
		return err
	}

	return nil
}

func (e *AdminExtension) handleCaseTimeSeriesAnalytics(c echo.Context) error {
	ctx := httputil.Context(c)

	// Parse all filters from request
	filters, _, _, err := queryutil.ParseRequest(c.Request())
	if err != nil {
		return ctx.Error(fmt.Errorf("invalid filters: %w", err), http.StatusBadRequest)
	}

	// Extract required parameters from filters
	metricFilter := queryutil.FindFilter(filters, "metric")
	timeRangeFilter := queryutil.FindFilter(filters, "time_range")

	if metricFilter == nil || metricFilter.GetValue() == "" {
		return ctx.Error(typesSvc.ErrMetricRequired, http.StatusBadRequest)
	}
	if timeRangeFilter == nil || timeRangeFilter.GetValue() == "" {
		return ctx.Error(typesSvc.ErrTimeRangeRequired, http.StatusBadRequest)
	}

	metric := fmt.Sprintf("%v", metricFilter.GetValue())
	timeRange := fmt.Sprintf("%v", timeRangeFilter.GetValue())

	// Remove the special filters we processed from the main filters
	filters = lo.Filter(filters, func(f queryutil.CrudFilter, _ int) bool {
		return f.GetField() != "metric" && f.GetField() != "time_range"
	})

	// Get time-series data
	data, err := e.caseService.GetTimeSeriesMetrics(metric, timeRange, filters)
	if err != nil {
		e.logger.Error("Failed to get time-series metrics",
			zap.String("metric", metric),
			zap.String("timeRange", timeRange),
			zap.Error(err))

		switch err {
		case typesSvc.ErrMetricRequired, typesSvc.ErrTimeRangeRequired, typesSvc.ErrInvalidMetric, util.ErrInvalidTimeRange:
			return ctx.Error(err, http.StatusBadRequest)
		default:
			return ctx.Error(fmt.Errorf("failed to get metrics: %w", err), http.StatusInternalServerError)
		}
	}

	return ctx.Encode(data)
}

func (e *AdminExtension) getTypeSourceMatrix(c echo.Context) error {
	ctx := httputil.Context(c)

	// Parse all filters from request
	filters, _, _, err := queryutil.ParseRequest(c.Request())
	if err != nil {
		return ctx.Error(fmt.Errorf("invalid filters: %w", err), http.StatusBadRequest)
	}

	// Extract time range from filters and remove it from the filters list
	var timeRange string
	filters = lo.Filter(filters, func(f queryutil.CrudFilter, _ int) bool {
		if f.GetField() == "time_range" {
			timeRange = fmt.Sprintf("%v", f.GetValue())
			return false
		}
		return true
	})

	if timeRange == "" {
		return ctx.Error(util.ErrInvalidTimeRange, http.StatusBadRequest)
	}

	results, err := e.caseService.GetTypeSourceMatrix(timeRange, filters)
	if err != nil {
		return ctx.Error(err, http.StatusInternalServerError)
	}

	// Convert results to DTO using httputil.EncodeResponse
	var responseDto dto.CaseTypeSourceMatrixResponse
	if err := httputil.EncodeResponse[[]pluginDb.CaseTypeSourceBreakdown, *dto.CaseTypeSourceMatrixResponse](ctx, results, &responseDto); err != nil {
		e.logger.Error("Failed to encode case type source matrix response", zap.Error(err))
		return err
	}

	return nil
}

func (e *AdminExtension) handle24HourCaseAnalytics(c echo.Context) error {
	ctx := httputil.Context(c)

	analytics, err := e.caseService.Get24HourAnalytics()
	if err != nil {
		e.logger.Error("Failed to get 24-hour analytics", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to generate analytics: %w", err), http.StatusInternalServerError)
	}

	var responseDto dto.CaseAnalyticsResponse
	if err := httputil.EncodeResponse[*typesSvc.CaseAnalytics, *dto.CaseAnalyticsResponse](ctx, analytics, &responseDto); err != nil {
		e.logger.Error("Failed to encode analytics response", zap.Error(err))
		return err
	}

	return nil
}
