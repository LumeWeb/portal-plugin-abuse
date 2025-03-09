package api

import (
	"fmt"
	"github.com/gorilla/mux"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	queryutilHttp "go.lumeweb.com/queryutil/http"
	"go.uber.org/zap"
	"net/http"
)

// registerSearchHandlers registers search-related route handlers
func (e *AdminExtension) registerSearchHandlers(router *mux.Router, accessSvc core.AccessService) error {
	routes := []struct {
		Path    string
		Method  string
		Handler http.HandlerFunc
		Access  string
	}{
		{"/search/cases", "GET", e.searchCases, core.ACCESS_ADMIN_ROLE},
		{"/search/global", "GET", e.globalSearch, core.ACCESS_ADMIN_ROLE},
	}

	for _, route := range routes {
		router.HandleFunc(route.Path, route.Handler).Methods(route.Method)
		if err := accessSvc.RegisterRoute("admin", "/admin/abuse"+route.Path, route.Method, route.Access); err != nil {
			return fmt.Errorf("failed to register route %s: %w", route.Path, err)
		}
	}

	return nil
}

// searchCases handles advanced search for cases
func (e *AdminExtension) searchCases(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.searchService == nil {
		e.logger.Error("Search service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	query := r.URL.Query().Get("q")

	// Handle list request processing
	if err := queryutilHttp.ProcessListRequest(
		ctx.Response, r,
		"cases",
		func(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Case, int64, error) {
			return e.searchService.SearchCases(r.Context(), query, filters, pagination)
		},
		func(c models.Case) dto.CaseResponse {
			var response dto.CaseResponse
			if err := response.FromModel(&c); err != nil {
				e.logger.Error("Failed to convert case", zap.Error(err))
				return dto.CaseResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to search cases", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to search cases")
	}
}

// globalSearch performs a search across multiple entities
func (e *AdminExtension) globalSearch(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Get pagination
	_, _, pagination, err := queryutil.ParseRequest(r)
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid pagination parameters")
		return
	}

	// Use injected service
	if e.searchService == nil {
		e.logger.Error("Search service unavailable")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Execute search
	result, err := e.searchService.GlobalSearch(r.Context(), r.URL.Query().Get("q"), pagination)
	if err != nil {
		e.logger.Error("Global search failed", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Search failed")
		return
	}

	// Convert to DTOs
	_dto := &dto.GlobalSearchResponse{}

	// Set Content-Range using original pagination window
	ctx.Response.Header().Set("Content-Range",
		fmt.Sprintf("items %d-%d/%d",
			pagination.Start,
			pagination.End-1, // End is exclusive in queryutil
			_dto.TotalCount,
		))

	// Encode response
	if err := httputil.EncodeResponse[*typesSvc.GlobalSearchResult, *dto.GlobalSearchResponse](ctx, result, _dto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}
