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

// registerSubjectHandlers registers all subject-related route handlers
func (e *AdminExtension) registerSubjectHandlers(router *mux.Router, accessSvc core.AccessService) error {
	routes := []struct {
		Path    string
		Method  string
		Handler http.HandlerFunc
		Access  string
	}{
		{"/subjects", "GET", e.listSubjects, core.ACCESS_ADMIN_ROLE},
		{"/subjects", "POST", e.createSubject, core.ACCESS_ADMIN_ROLE},
		{"/subjects/{id}", "GET", e.getSubject, core.ACCESS_ADMIN_ROLE},
	}

	for _, route := range routes {
		router.HandleFunc(route.Path, route.Handler).Methods(route.Method)
		if err := accessSvc.RegisterRoute("admin", "/admin/abuse"+route.Path, route.Method, route.Access); err != nil {
			return fmt.Errorf("failed to register route %s: %w", route.Path, err)
		}
	}

	return nil
}

// createSubject handles the creation of a new subject
func (e *AdminExtension) createSubject(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.subjectService == nil {
		e.logger.Error("Subject service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Validate the request using DTO
	var requestDto dto.SubjectCreateRequest
	subjectModel, ok := httputil.DecodeAndValidateRequest[*models.Subject, *dto.SubjectCreateRequest](ctx, &requestDto)
	if !ok {
		return
	}

	// Create the subject using the service
	createdSubject, err := e.subjectService.Create(subjectModel)
	if err != nil {
		e.logger.Error("Failed to create subject", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to create subject")
		return
	}

	// Prepare and send response
	var responseDto dto.SubjectResponse
	ctx.Response.WriteHeader(http.StatusCreated)
	if err := httputil.EncodeResponse[*models.Subject, *dto.SubjectResponse](ctx, createdSubject, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// getSubject retrieves a specific subject by ID
func (e *AdminExtension) getSubject(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.subjectService == nil {
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Extract ID from path
	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		_ = ctx.Error(fmt.Errorf("invalid subject ID: %w", err), http.StatusBadRequest)
		return
	}

	// Get the subject using the service
	subject, err := e.subjectService.GetByID(uint(id))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sendErrorResponse(&ctx, http.StatusNotFound, "Subject not found")
		} else {
			e.logger.Error("Failed to fetch subject", zap.Error(err))
			sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to fetch subject")
		}
		return
	}

	// Prepare and send response
	var responseDto dto.SubjectResponse
	if err := httputil.EncodeResponse[*models.Subject, *dto.SubjectResponse](ctx, subject, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// listSubjects returns a list of subjects with filtering and pagination
func (e *AdminExtension) listSubjects(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.subjectService == nil {
		e.logger.Error("Subject service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Handle list request processing
	if err := queryutilHttp.ProcessListRequest(
		ctx.Response, r,
		"subjects",
		func(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.Subject, int64, error) {
			return e.subjectService.List(filters, sorts, pagination)
		},
		func(s models.Subject) dto.SubjectResponse {
			var response dto.SubjectResponse
			if err := response.FromModel(&s); err != nil {
				e.logger.Error("Failed to convert subject", zap.Error(err))
				return dto.SubjectResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list subjects", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to list subjects")
	}
}
