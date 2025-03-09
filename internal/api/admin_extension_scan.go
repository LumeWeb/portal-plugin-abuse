package api

import (
	"errors"
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
	"gorm.io/gorm"
	"net/http"
	"strconv"
)

// registerScanHandlers registers all scan-related route handlers
func (e *AdminExtension) registerScanHandlers(router *mux.Router, accessSvc core.AccessService) error {
	routes := []struct {
		Path    string
		Method  string
		Handler http.HandlerFunc
		Access  string
	}{
		{"/cases/{caseId}/scans", "GET", e.listCaseScans, core.ACCESS_ADMIN_ROLE},
		{"/cases/{caseId}/scans", "POST", e.createScanRequest, core.ACCESS_ADMIN_ROLE},
		{"/cases/{caseId}/scans/{scanId}", "GET", e.getScan, core.ACCESS_ADMIN_ROLE},
		{"/cases/{caseId}/scans/{scanId}/results", "GET", e.getScanResults, core.ACCESS_ADMIN_ROLE},
	}

	for _, route := range routes {
		router.HandleFunc(route.Path, route.Handler).Methods(route.Method)
		if err := accessSvc.RegisterRoute("admin", "/admin/abuse"+route.Path, route.Method, route.Access); err != nil {
			return fmt.Errorf("failed to register route %s: %w", route.Path, err)
		}
	}

	return nil
}

// listCaseScans returns all scans for a case using queryutil list API
func (e *AdminExtension) listCaseScans(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Get case ID from path
	vars := mux.Vars(r)
	caseId, err := strconv.ParseUint(vars["caseId"], 10, 32)
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid case ID format")
		return
	}

	// Use injected service
	if e.caseService == nil {
		e.logger.Error("Case service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	if _, err := e.caseService.GetByID(uint(caseId)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sendErrorResponse(&ctx, http.StatusNotFound, "Case not found")
		} else {
			e.logger.Error("Failed to fetch case", zap.Error(err))
			sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to fetch case")
		}
		return
	}

	// Use injected service
	if e.scanService == nil {
		e.logger.Error("Scan service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Use standardized list processing
	if err := queryutilHttp.ProcessListRequest(
		ctx.Response, r,
		"scans",
		func(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.CaseScan, int64, error) {
			return e.scanService.GetScansForCase(uint(caseId), pagination)
		},
		func(scan models.CaseScan) dto.ScanResponse {
			var response dto.ScanResponse
			if err := response.FromModel(&scan); err != nil {
				e.logger.Error("Failed to convert scan", zap.Error(err))
				return dto.ScanResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list scans", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to list scans")
		return // Added return to prevent fallthrough
	}
}

// createScanRequest initiates a scan for an existing subject
func (e *AdminExtension) createScanRequest(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Get case ID from path
	vars := mux.Vars(r)
	caseId, err := strconv.ParseUint(vars["caseId"], 10, 32)
	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid case ID format")
		return
	}

	// Validate request DTO
	var req dto.ManualScanRequest
	if _, ok := httputil.DecodeAndValidateRequest[*models.CaseScan, *dto.ManualScanRequest](ctx, &req); !ok {
		return
	}
	// Use injected service
	if e.scanService == nil {
		e.logger.Error("Scan service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Get services
	caseService := core.GetService[typesSvc.CaseService](e.ctx, typesSvc.CASE_SERVICE)
	if caseService == nil {
		e.logger.Error("Case service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Case unavailable")
		return
	}

	_, err = caseService.GetByID(uint(caseId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sendErrorResponse(&ctx, http.StatusNotFound, "Case not found")
		} else {
			e.logger.Error("Failed to fetch case", zap.Error(err))
			sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to fetch case")
		}
		return
	}

	// Create scan request
	if err := e.scanService.CreateScanRequest(uint(caseId)); err != nil {
		e.logger.Error("Failed to create scan request", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to initiate scan")
		return
	}

	ctx.Response.WriteHeader(http.StatusAccepted)

}

// getScan retrieves a specific scan by ID
func (e *AdminExtension) getScan(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.scanService == nil {
		e.logger.Error("Scan service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Extract  caseId and scanId from path
	vars := mux.Vars(r)
	scanId, err := strconv.ParseUint(vars["scanId"], 10, 32)

	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid scan ID format")
		return
	}

	// Get the scan using the service
	scan, err := e.scanService.GetScanById(uint(scanId))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sendErrorResponse(&ctx, http.StatusNotFound, "Scan not found")
		} else {
			e.logger.Error("Failed to fetch scan", zap.Error(err))
			sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to fetch scan")
		}
		return
	}

	// Prepare and send response
	var responseDto dto.ScanResponse
	if err := httputil.EncodeResponse[*models.CaseScan, *dto.ScanResponse](ctx, scan, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// getScanResults returns results for a specific scan
func (e *AdminExtension) getScanResults(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Use injected service
	if e.scanService == nil {
		e.logger.Error("Scan service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Extract scanId from path
	vars := mux.Vars(r)
	scanId, err := strconv.ParseUint(vars["scanId"], 10, 32)

	if err != nil {
		sendErrorResponse(&ctx, http.StatusBadRequest, "Invalid scan ID format")
		return
	}

	// Use standardized list processing for scan results
	if err := queryutilHttp.ProcessListRequest(
		ctx.Response, r,
		"scan_results",
		func(_ []queryutil.Filter, _ []queryutil.Sort, _ queryutil.Pagination) ([]*core.ScanResult, int64, error) {
			results, err := e.scanService.GetScanResults(uint(scanId))
			if err != nil {
				return nil, 0, err
			}
			return results, 0, nil
		},
		func(result *core.ScanResult) dto.ScanResultResponse {
			var response dto.ScanResultResponse
			if err := response.FromModel(result); err != nil {
				e.logger.Error("Failed to convert scan result", zap.Error(err))
				return dto.ScanResultResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list scan results", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to list scan results")
	}
}
