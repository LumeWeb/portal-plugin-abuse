package api

import (
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	queryutilHttp "go.lumeweb.com/queryutil/http"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
	"strconv"
)

// registerReporterHandlers registers all reporter-related route handlers
func (e *AdminExtension) registerReporterHandlers(router *mux.Router, accessSvc core.AccessService) error {
	routes := []struct {
		Path    string
		Method  string
		Handler http.HandlerFunc
		Access  string
	}{
		{"/reporters", "GET", e.listReporters, core.ACCESS_ADMIN_ROLE},
		{"/reporters", "POST", e.createReporter, core.ACCESS_ADMIN_ROLE},
		{"/reporters/{id}", "GET", e.getReporter, core.ACCESS_ADMIN_ROLE},
	}

	for _, route := range routes {
		router.HandleFunc(route.Path, route.Handler).Methods(route.Method)
		if err := accessSvc.RegisterRoute("admin", "/admin/abuse"+route.Path, route.Method, route.Access); err != nil {
			return fmt.Errorf("failed to register route %s: %w", route.Path, err)
		}
	}

	return nil
}

// createReporter handles the creation of a new reporter
func (e *AdminExtension) createReporter(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.reporterService == nil {
		e.logger.Error("Reporter service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Validate the request using DTO
	var requestDto dto.ReporterCreateRequest
	reporterModel, ok := httputil.DecodeAndValidateRequest[*models.Reporter, *dto.ReporterCreateRequest](ctx, &requestDto)
	if !ok {
		return
	}

	// Create the reporter using the service
	createdReporter, err := e.reporterService.Create(reporterModel)
	if err != nil {
		e.logger.Error("Failed to create reporter", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to create reporter")
		return
	}

	// Prepare and send response
	var responseDto dto.ReporterResponse
	ctx.Response.WriteHeader(http.StatusCreated)
	if err := httputil.EncodeResponse[*models.Reporter, *dto.ReporterResponse](ctx, createdReporter, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// getReporter retrieves a specific reporter by ID
func (e *AdminExtension) getReporter(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.reporterService == nil {
		e.logger.Error("Reporter service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Extract ID from path
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid reporter ID format")
		return
	}

	// Get the reporter using the service
	reporter, err := e.reporterService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sendErrorResponse(&ctx, http.StatusNotFound, "Reporter not found")
		} else {
			e.logger.Error("Failed to fetch reporter", zap.Error(err))
			sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to fetch reporter")
		}
		return
	}

	// Prepare and send response
	var responseDto dto.ReporterResponse
	if err := httputil.EncodeResponse[*models.Reporter, *dto.ReporterResponse](ctx, reporter, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// listReporters returns a list of reporters with filtering and pagination
func (e *AdminExtension) listReporters(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.reporterService == nil {
		e.logger.Error("Reporter service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Handle list request processing
	if err := queryutilHttp.ProcessListRequest(
		ctx.Response, r,
		"reporters",
		func(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Reporter, int64, error) {
			return e.reporterService.List(filters, sorts, pagination)
		},
		func(r models.Reporter) dto.ReporterResponse {
			var response dto.ReporterResponse
			if err := response.FromModel(&r); err != nil {
				e.logger.Error("Failed to convert reporter", zap.Error(err))
				return dto.ReporterResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list reporters", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to list reporters")
	}
}
