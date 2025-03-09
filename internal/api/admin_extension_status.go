package api

import (
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
	"strconv"
)

// registerStatusUpdateHandlers adds the status update handlers
func (e *AdminExtension) registerStatusUpdateHandlers(router *mux.Router, accessSvc core.AccessService) error {
	router.HandleFunc("/cases/{id}/status", e.updateCaseStatus).Methods("PUT")
	if err := accessSvc.RegisterRoute("admin", "/admin/abuse/cases/{id}/status", "PUT", core.ACCESS_ADMIN_ROLE); err != nil {
		return fmt.Errorf("failed to register route: %w", err)
	}
	return nil
}

// updateCaseStatus handles updates to a case's status
func (e *AdminExtension) updateCaseStatus(w http.ResponseWriter, r *http.Request) {
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

	// Get the existing case to track status change
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

	// Save the old status
	oldStatus := existingCase.Status

	// Validate the request using DTO
	var requestDto dto.CaseStatusUpdateRequest
	if _, ok := httputil.DecodeAndValidateRequest[*models.Case, *dto.CaseStatusUpdateRequest](ctx, &requestDto); !ok {
		return
	}

	// Update status using the service
	err = e.caseService.UpdateStatus(uint(id), models.CaseStatus(requestDto.Status))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sendErrorResponse(&ctx, http.StatusNotFound, "Case not found")
		} else {
			e.logger.Error("Failed to update case status", zap.Error(err))
			sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to update case status")
		}
		return
	}

	// Get updated case for response
	updatedCase, err := e.caseService.GetByID(uint(id))
	if err != nil {
		e.logger.Error("Failed to fetch updated case", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to fetch updated case")
		return
	}

	// Prepare and send response
	var responseDto dto.CaseStatusResponse
	if err := responseDto.FromModel(updatedCase); err != nil {
		e.logger.Error("Failed to convert case status response", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to prepare response")
		return
	}
	responseDto.OldStatus = string(oldStatus)

	// Send notification email if status changed
	if oldStatus != updatedCase.Status {
		go func() {
			if err := e.caseService.SendStatusUpdateNotification(uint(id), oldStatus, updatedCase.Status); err != nil {
				e.logger.Error("Failed to send status update notification", zap.Error(err))
			}
		}()
	}

	if err := httputil.EncodeResponse[*models.Case, *dto.CaseStatusResponse](ctx, updatedCase, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}
