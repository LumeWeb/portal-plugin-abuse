package api

import (
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/samber/lo"
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

// registerCaseHandlers registers all case-related route handlers
func (e *AdminExtension) registerCaseHandlers(router *mux.Router, accessSvc core.AccessService) error {
	routes := []struct {
		Path    string
		Method  string
		Handler http.HandlerFunc
		Access  string
	}{
		{"/cases", "GET", e.listCases, core.ACCESS_ADMIN_ROLE},
		{"/cases", "POST", e.createCase, core.ACCESS_ADMIN_ROLE},
		{"/cases/{id}", "GET", e.getCase, core.ACCESS_ADMIN_ROLE},
		{"/cases/{id}", "PUT", e.updateCase, core.ACCESS_ADMIN_ROLE},
	}

	for _, route := range routes {
		router.HandleFunc(route.Path, route.Handler).Methods(route.Method)
		if err := accessSvc.RegisterRoute("admin", "/admin/abuse"+route.Path, route.Method, route.Access); err != nil {
			return fmt.Errorf("failed to register route %s: %w", route.Path, err)
		}
	}

	return nil
}

// createCase handles the creation of a new abuse case
func (e *AdminExtension) createCase(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.caseService == nil {
		e.logger.Error("Case service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Validate the request using DTO
	var requestDto dto.CreateCaseRequest
	caseModel, ok := httputil.DecodeAndValidateRequest[*models.Case, *dto.CreateCaseRequest](ctx, &requestDto)
	if !ok {
		return
	}

	// Create the case using the service
	createdCase, err := e.caseService.Create(caseModel)
	if err != nil {
		e.logger.Error("Failed to create case", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to create case")
		return
	}

	// Prepare and send response
	var responseDto dto.CaseResponse
	ctx.Response.WriteHeader(http.StatusCreated)
	if err := httputil.EncodeResponse[*models.Case, *dto.CaseResponse](ctx, createdCase, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// getCase retrieves a specific case by ID
func (e *AdminExtension) getCase(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.caseService == nil {
		e.logger.Error("Case service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Extract ID from path
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid case ID format")
		return
	}

	// Get the case using the service
	caseModel, err := e.caseService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sendErrorResponse(&ctx, http.StatusNotFound, "Case not found")
		} else {
			e.logger.Error("Failed to fetch case", zap.Error(err))
			sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to fetch case")
		}
		return
	}

	// Prepare and send response
	var responseDto dto.CaseResponse
	if err := httputil.EncodeResponse[*models.Case, *dto.CaseResponse](ctx, caseModel, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// listCases returns a list of cases with filtering and pagination
func (e *AdminExtension) listCases(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.caseService == nil {
		e.logger.Error("Case service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Handle list request processing
	if err := queryutilHttp.ProcessListRequest(
		ctx.Response, r,
		"cases",
		func(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Case, int64, error) {
			return e.caseService.List(filters, sorts, pagination)
		},
		func(c models.Case) dto.CaseResponse {
			var response dto.CaseResponse
			err := response.FromModel(&c)
			if err != nil {
				e.logger.Error("Failed to convert case", zap.Error(err))
				return dto.CaseResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list cases", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to list cases")
	}
}

// updateCase handles updates to an existing case
func (e *AdminExtension) updateCase(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.caseService == nil {
		e.logger.Error("Case service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Extract ID from path
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid case ID format")
		return
	}

	// Fetch existing case
	existingCase, err := e.caseService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sendErrorResponse(&ctx, http.StatusNotFound, "Case not found")
		} else {
			e.logger.Error("Failed to fetch case", zap.Error(err))
			sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to fetch case")
		}
		return
	}

	// Validate the request using DTO
	var requestDto dto.UpdateCaseRequest
	if _, ok := httputil.DecodeAndValidateRequest[*models.Case, *dto.UpdateCaseRequest](ctx, &requestDto); !ok {
		return
	}

	// Update fields from DTO
	if lo.FromPtr(requestDto.Type) != "" {
		existingCase.Type = models.CaseType(lo.FromPtr(requestDto.Type))
	}
	if lo.FromPtr(requestDto.Description) != "" {
		existingCase.Description = lo.FromPtr(requestDto.Description)
	}
	if lo.FromPtr(requestDto.Priority) != "" {
		existingCase.Priority = models.CasePriority(lo.FromPtr(requestDto.Priority))
	}
	if requestDto.Priority != nil {
		existingCase.NeedsReview = lo.FromPtr(requestDto.NeedsReview)
	}

	// Update the case
	if err := e.caseService.Update(existingCase); err != nil {
		e.logger.Error("Failed to update case", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to update case")
		return
	}

	// Prepare and send response
	var responseDto dto.CaseResponse
	if err := httputil.EncodeResponse[*models.Case, *dto.CaseResponse](ctx, existingCase, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}
