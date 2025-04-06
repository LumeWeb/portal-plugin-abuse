package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-router"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// registerStatusUpdateHandlers registers the status update handler using portal-router.
func (e *AdminExtension) registerStatusUpdateHandlers(gRouter router.Router, accessSvc core.AccessService) error {
	routes := router.DefineRoutes(
		router.NewRoute(http.MethodPut, "/cases/:id/status", e.updateCaseStatus,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Update case status"),
				router.WithDescription("Update the status of an abuse case"),
				router.WithTags("Cases"),
				router.WithPathParam("id", "Case ID", 1),
				router.WithRequestBody(&dto.CaseStatusUpdateRequest{}, "New status details", true),
				router.WithSuccessResponse(
					http.StatusOK,
					"Case status updated successfully",
					router.WithJSONContent(dto.CaseStatusResponse{}),
				),
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid request payload or case ID format"),
						router.DefineSwaggerErrorResponse(http.StatusNotFound, "Case not found"),
						router.DefineSwaggerErrorResponse(http.StatusUnprocessableEntity, "Validation error"),
					),
				),
			),
		),
	)

	// Register routes with the router and access service
	// The base path for this extension is "/abuse", so the route "/cases/:id/status" becomes "/abuse/cases/:id/status".
	// The access service registration needs the full path relative to the admin API root.
	// core.GetAPI("admin").Subdomain() is "admin", so the full path is "/admin/abuse" + route.Path
	return router.RegisterRoutes(gRouter, accessSvc, core.GetAPI("admin").Subdomain(), routes, router.WithCors())
}

// updateCaseStatus handles updates to a case's status
func (e *AdminExtension) updateCaseStatus(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Use injected service
	if e.caseService == nil {
		e.logger.Error("Case service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Extract ID from path using Echo context
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return ctx.Error(errors.New("invalid case ID format"), http.StatusBadRequest) // Use ctx.Error
	}

	// Get the existing case to track status change
	existingCase, err := e.caseService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Error(errors.New("case not found"), http.StatusNotFound) // Use ctx.Error
		} else {
			e.logger.Error("Failed to fetch case", zap.Error(err))
			return ctx.Error(errors.New("failed to fetch case"), http.StatusInternalServerError) // Use ctx.Error
		}
	}

	// Save the old status
	oldStatus := existingCase.Status

	// Validate the request using DTO
	var requestDto dto.CaseStatusUpdateRequest
	// httputil.DecodeAndValidateRequest now takes Echo context
	if _, ok := httputil.DecodeAndValidateRequest[*models.Case, *dto.CaseStatusUpdateRequest](ctx, &requestDto); !ok {
		// Error handled by DecodeAndValidateRequest
		return nil // Return nil as the error is already handled
	}

	// Update status using the service
	err = e.caseService.UpdateStatus(uint(id), models.CaseStatus(requestDto.Status))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Error(errors.New("case not found"), http.StatusNotFound) // Use ctx.Error
		} else {
			e.logger.Error("Failed to update case status", zap.Error(err))
			return ctx.Error(errors.New("failed to update case status"), http.StatusInternalServerError) // Use ctx.Error
		}
	}

	// Get updated case for response
	updatedCase, err := e.caseService.GetByID(uint(id))
	if err != nil {
		e.logger.Error("Failed to fetch updated case", zap.Error(err))
		return ctx.Error(errors.New("failed to fetch updated case"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Prepare and send response
	var responseDto dto.CaseStatusResponse
	if err := responseDto.FromModel(updatedCase); err != nil {
		e.logger.Error("Failed to convert case status response", zap.Error(err))
		return ctx.Error(errors.New("failed to prepare response"), http.StatusInternalServerError) // Use ctx.Error
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

	// Use ctx.Encode for encoding
	if err := ctx.Encode(responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
		return err // ctx.Encode returns an error
	}

	return nil // Return nil on success
}
