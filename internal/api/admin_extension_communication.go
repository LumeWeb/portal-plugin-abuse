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

// registerCommunicationHandlers registers all communication-related route handlers
func (e *AdminExtension) registerCommunicationHandlers(router *mux.Router, accessSvc core.AccessService) error {
	routes := []struct {
		Path    string
		Method  string
		Handler http.HandlerFunc
		Access  string
	}{
		{"/cases/{id}/communications", "GET", e.listCaseCommunications, core.ACCESS_ADMIN_ROLE},
		{"/cases/{id}/communications", "POST", e.addCaseCommunication, core.ACCESS_ADMIN_ROLE},
	}

	for _, route := range routes {
		router.HandleFunc(route.Path, route.Handler).Methods(route.Method)
		if err := accessSvc.RegisterRoute("admin", "/admin/abuse"+route.Path, route.Method, route.Access); err != nil {
			return fmt.Errorf("failed to register route %s: %w", route.Path, err)
		}
	}

	return nil
}

// listCaseCommunications returns all communications for a case
func (e *AdminExtension) listCaseCommunications(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Get case ID from path
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid case ID format")
		return
	}

	// Use injected service
	if e.commService == nil {
		e.logger.Error("Communication service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Handle list request processing
	if err := queryutilHttp.ProcessListRequest(
		ctx.Response, r,
		"communications",
		func(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Communication, int64, error) {
			return e.commService.ListByCaseID(uint(id), filters, sorts, pagination)
		},
		func(comm models.Communication) dto.CommunicationResponse {
			var response dto.CommunicationResponse
			if err := response.FromModel(&comm); err != nil {
				e.logger.Error("Failed to convert communication", zap.Error(err))
				return dto.CommunicationResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list communications", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to list communications")
	}
}

// addCaseCommunication adds a communication to a case
func (e *AdminExtension) addCaseCommunication(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected services
	if e.caseService == nil || e.commService == nil {
		e.logger.Error("Required service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Get case ID from path
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid case ID format")
		return
	}

	// Validate the request using DTO
	var requestDto dto.CommunicationCreateRequest
	commModel, ok := httputil.DecodeAndValidateRequest[*models.Communication, *dto.CommunicationCreateRequest](ctx, &requestDto)
	if !ok {
		return
	}

	// Verify case exists
	if _, err := e.caseService.GetByID(uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sendErrorResponse(&ctx, http.StatusNotFound, "Case not found")
		} else {
			e.logger.Error("Failed to fetch case", zap.Error(err))
			sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to fetch case")
		}
		return
	}

	// Set case ID and sender (placeholder until auth implemented)
	commModel.CaseID = uint(id)
	commModel.SenderID = 1 // TODO: Replace with actual user ID from auth context

	// Create the communication using the service
	createdComm, err := e.commService.Create(commModel)
	if err != nil {
		e.logger.Error("Failed to create communication", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to create communication")
		return
	}

	// Prepare and send response
	var responseDto dto.CommunicationResponse
	ctx.Response.WriteHeader(http.StatusCreated)
	if err := httputil.EncodeResponse[*models.Communication, *dto.CommunicationResponse](ctx, createdComm, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}
