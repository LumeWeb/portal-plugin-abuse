package api

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	queryutilHttp "go.lumeweb.com/queryutil/http"
	"go.uber.org/zap"
)

// registerBlockListHandlers registers the block list related route handlers
func (e *AdminExtension) registerBlockListHandlers(router *mux.Router, accessSvc core.AccessService) error {
	routes := []struct {
		Path    string
		Method  string
		Handler http.HandlerFunc
		Access  string
	}{
		{"/blocklist", "GET", e.listBlocks, core.ACCESS_ADMIN_ROLE},
		{"/blocklist", "POST", e.createBlock, core.ACCESS_ADMIN_ROLE},
		{"/blocklist/{hash}", "GET", e.getBlockedContent, core.ACCESS_ADMIN_ROLE},
		{"/blocklist/{hash}", "PUT", e.unblockContent, core.ACCESS_ADMIN_ROLE},
		{"/blocklist/{hash}", "DELETE", e.unblockContent, core.ACCESS_ADMIN_ROLE},
	}

	for _, route := range routes {
		router.HandleFunc(route.Path, route.Handler).Methods(route.Method)
		if err := accessSvc.RegisterRoute("admin", "/admin/abuse"+route.Path, route.Method, route.Access); err != nil {
			return fmt.Errorf("failed to register route %s: %w", route.Path, err)
		}
	}

	return nil
}

func (e *AdminExtension) listBlocks(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	if e.blockListService == nil {
		e.logger.Error("Blocklist service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	if err := queryutilHttp.ProcessListRequest(
		ctx.Response, r,
		"blocks",
		func(filters []queryutil.Filter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.BlockList, int64, error) {
			return e.blockListService.ListBlockedContent(filters, sorts, pagination)
		},
		func(b models.BlockList) dto.BlockContentResponse {
			var response dto.BlockContentResponse
			if err := response.FromModel(&b); err != nil {
				e.logger.Error("Failed to convert block", zap.Error(err))
				return dto.BlockContentResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list blocks", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to list blocks")
	}
}

func (e *AdminExtension) createBlock(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	if e.blockListService == nil {
		e.logger.Error("Blocklist service not available")
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	var requestDto dto.BlockContentCreateRequest
	blockModel, ok := httputil.DecodeAndValidateRequest[*models.BlockList, *dto.BlockContentCreateRequest](ctx, &requestDto)
	if !ok {
		return
	}

	blockModel, err := e.blockListService.BlockContent(blockModel)
	if err != nil {
		e.logger.Error("Failed to create block", zap.Error(err))
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Failed to create block")
		return
	}

	var responseDto dto.BlockContentResponse
	ctx.Response.WriteHeader(http.StatusCreated)
	if err := httputil.EncodeResponse[*models.BlockList, *dto.BlockContentResponse](ctx, blockModel, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// getBlockedContent handles GET requests to fetch a specific blocked content
func (e *AdminExtension) getBlockedContent(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Get hash from path
	vars := mux.Vars(r)
	hashStr := vars["hash"]

	// Use injected service
	if e.blockListService == nil {
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Convert hash string to multihash
	decodedHash, err := core.ParseStorageHash(hashStr)
	if err != nil {
		_ = ctx.Error(fmt.Errorf("invalid hash format: %w", err), http.StatusBadRequest)
		return
	}

	// Get the block by hash
	block, err := e.blockListService.GetBlockedContent(decodedHash)
	if err != nil {
		if errors.Is(gorm.ErrRecordNotFound, err) {
			_ = ctx.Error(fmt.Errorf("blocked content not found"), http.StatusNotFound)
		} else {
			e.logger.Error("Failed to fetch blocked content", zap.Error(err))
			_ = ctx.Error(fmt.Errorf("failed to fetch blocked content: %w", err), http.StatusInternalServerError)
		}
		return
	}

	// Prepare and send response
	var responseDto dto.BlockContentResponse
	if err := httputil.EncodeResponse[*models.BlockList, *dto.BlockContentResponse](ctx, block, &responseDto); err != nil {
		e.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// unblockContent handles DELETE requests to remove content from the block list
func (e *AdminExtension) unblockContent(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	// Get hash from path
	vars := mux.Vars(r)
	hashStr := vars["hash"]

	// Use injected service
	if e.blockListService == nil {
		sendErrorResponse(&ctx, http.StatusInternalServerError, "Service unavailable")
		return
	}

	// Convert hash string to multihash
	decodedHash, err := core.ParseStorageHash(hashStr)
	if err != nil {
		_ = ctx.Error(fmt.Errorf("invalid hash format: %w", err), http.StatusBadRequest)
		return
	}

	// Get the block by hash
	block, err := e.blockListService.GetBlockedContent(decodedHash)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			_ = ctx.Error(fmt.Errorf("blocked content not found"), http.StatusNotFound)
		} else {
			e.logger.Error("Failed to fetch blocked content", zap.Error(err))
			_ = ctx.Error(fmt.Errorf("failed to fetch blocked content: %w", err), http.StatusInternalServerError)
		}
		return
	}

	// Unblock the content using the multihash from the model
	if err := e.blockListService.UnblockContent(core.NewStorageHashFromMultihash(block.Hash, 0, nil)); err != nil {
		e.logger.Error("Failed to unblock content", zap.Error(err))
		_ = ctx.Error(fmt.Errorf("failed to unblock content: %w", err), http.StatusInternalServerError)
		return
	}

	ctx.Response.WriteHeader(http.StatusOK)
}
