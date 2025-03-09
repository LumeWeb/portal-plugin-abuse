package api

import (
	"fmt"
	"github.com/gorilla/mux"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	queryutilHttp "go.lumeweb.com/queryutil/http"
	"go.uber.org/zap"
	"net/http"
)

func (e *AdminExtension) registerAnalyticsHandlers(router *mux.Router, accessSvc core.AccessService) error {
	analyticsRouter := router.PathPrefix("/analytics").Subrouter()

	routes := []struct {
		Path    string
		Method  string
		Handler http.HandlerFunc
		Access  string
	}{
		{"/cases", "GET", e.handleCaseAnalytics, core.ACCESS_ADMIN_ROLE},
		{"/cases/7d", "GET", e.handle7DayCaseAnalytics, core.ACCESS_ADMIN_ROLE},
		{"/cases/30d", "GET", e.handle30DayCaseAnalytics, core.ACCESS_ADMIN_ROLE},
	}

	for _, route := range routes {
		analyticsRouter.HandleFunc(route.Path, route.Handler).Methods(route.Method)
		if err := accessSvc.RegisterRoute("admin", "/admin/abuse/analytics"+route.Path, route.Method, route.Access); err != nil {
			return fmt.Errorf("failed to register analytics route %s: %w", route.Path, err)
		}
	}

	return nil
}

func (e *AdminExtension) handleCaseAnalytics(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	filters, _, _, err := queryutilHttp.ParseRequestHTTP(r)
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid filters: "+err.Error())
		return
	}

	analytics, err := e.caseService.GetAnalytics(filters)
	if err != nil {
		e.logger.Error("Failed to get case analytics", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to generate analytics")
		return
	}

	var responseDto dto.CaseAnalyticsResponse
	if err := responseDto.FromModel(analytics); err != nil {
		e.logger.Error("Failed to convert analytics to response", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to format analytics")
		return
	}

	if err := httputil.EncodeResponse[*typesSvc.CaseAnalytics, *dto.CaseAnalyticsResponse](ctx, analytics, &responseDto); err != nil {
		e.logger.Error("Failed to encode analytics response", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to format response")
		return
	}
}

func (e *AdminExtension) handle7DayCaseAnalytics(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	analytics, err := e.caseService.Get7DayAnalytics()
	if err != nil {
		e.logger.Error("Failed to get 7-day analytics", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to generate analytics")
		return
	}

	var responseDto dto.CaseAnalyticsResponse
	if err := httputil.EncodeResponse[*typesSvc.CaseAnalytics, *dto.CaseAnalyticsResponse](ctx, analytics, &responseDto); err != nil {
		e.logger.Error("Failed to encode analytics response", zap.Error(err))
	}
}

func (e *AdminExtension) handle30DayCaseAnalytics(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	analytics, err := e.caseService.Get30DayAnalytics()
	if err != nil {
		e.logger.Error("Failed to get 30-day analytics", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to generate analytics")
		return
	}

	var responseDto dto.CaseAnalyticsResponse
	if err := httputil.EncodeResponse[*typesSvc.CaseAnalytics, *dto.CaseAnalyticsResponse](ctx, analytics, &responseDto); err != nil {
		e.logger.Error("Failed to encode analytics response", zap.Error(err))
	}
}
