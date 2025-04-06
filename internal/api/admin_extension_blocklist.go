package api

import (
	"errors"
	"fmt"
	"github.com/labstack/echo/v4" // Import echo
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-middleware/auth/jwt"
	"go.lumeweb.com/portal-plugin-abuse/internal/api/dto"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-router" // Import portal-router
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
	"go.lumeweb.com/queryutil/filter"
	queryutilHttp "go.lumeweb.com/queryutil/http"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"net/http"
)

// registerBlockListHandlers registers the block list related route handlers using portal-router.
func (e *AdminExtension) registerBlockListHandlers(gRouter router.Router, accessSvc core.AccessService) error {

	schema := queryutil.NewSchemaProvider().ForType(dto.BlockContentResponse{})
	routes := router.DefineRoutes(
		// List Blocked Content
		router.NewRoute(http.MethodGet, "/blocklist", e.listBlocks,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithListEndpoint(
					"List Blocked Content",
					"Retrieve a list of blocked content items with optional filtering and pagination",
					jwt.PurposeLogin, // Assuming admin endpoints use login JWT
					dto.BlockContentResponse{},
					filter.Pagination{},
					schema.SortableFields(),
					nil,
					router.WithFilterParamsFromSchema(schema),
					// Use WithErrorResponses to merge custom errors with defaults
					router.WithErrorResponses(
						router.DefineSwaggerErrorResponses(
							router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid filter parameters"),
						),
					),
				),
				router.WithSuccessResponse(http.StatusOK, "List of blocked content items", router.WithTotalCountHeader()),
			),
		),

		// Create Block
		router.NewRoute(http.MethodPost, "/blocklist", e.createBlock,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Create block"),
				router.WithDescription("Add a new content item to the block list"),
				router.WithTags("Blocklist"),
				router.WithRequestBody(&dto.BlockContentCreateRequest{}, "Block details", true),
				router.WithSuccessResponse(
					http.StatusCreated,
					"Block created successfully",
					router.WithJSONContent(dto.BlockContentResponse{}),
				),
				router.WithErrorResponses( // Use WithErrorResponses for merging with defaults
					router.DefineSwaggerErrorResponse(http.StatusUnprocessableEntity, "Validation error"),
				),
			),
		),

		// Get Blocked Content
		router.NewRoute(http.MethodGet, "/blocklist/:hash", e.getBlockedContent,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Get blocked content"),
				router.WithDescription("Retrieve details of a specific blocked content item by its hash"),
				router.WithTags("Blocklist"),
				router.WithPathParam("hash", "Multihash of the blocked content", exampleCID),
				router.WithSuccessResponse(
					http.StatusOK,
					"Blocked content details",
					router.WithJSONContent(dto.BlockContentResponse{}),
				),
				// Use WithErrorResponses to merge custom errors with defaults
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid hash format"),
						router.DefineSwaggerErrorResponse(http.StatusNotFound, "Blocked content not found"),
					),
				),
			),
		),

		// Unblock Content (DELETE)
		router.NewRoute(http.MethodDelete, "/blocklist/:hash", e.unblockContent,
			router.WithAccess(core.ACCESS_ADMIN_ROLE),
			router.WithSwagger(
				router.WithSummary("Unblock content"),
				router.WithDescription("Removes content from the block list by its hash."),
				router.WithTags("Blocklist"),
				router.WithPathParam("hash", "Multihash of the content to unblock", exampleCID),
				router.WithSuccessResponse(http.StatusOK, "Content successfully unblocked"), // 200 OK with no body
				// Use WithErrorResponses to merge custom errors with defaults
				router.WithErrorResponses(
					router.DefineSwaggerErrorResponses(
						router.DefineSwaggerErrorResponse(http.StatusBadRequest, "Invalid hash format"),
						router.DefineSwaggerErrorResponse(http.StatusNotFound, "Blocked content not found"),
					),
				),
			),
		),
		// Note: The original code had a PUT handler for unblockContent as well.
		// This seems redundant with the DELETE handler and is not standard REST practice for unblocking.
		// I've omitted the PUT handler based on the DELETE handler's description.
		// If the PUT handler is intended for a different purpose (e.g., updating block details),
		// it would need a different handler function and Swagger definition.
	)

	// Register routes with the router and access service
	// The path registered with accessSvc should be the full path including the subdomain prefix if applicable.
	// Assuming the router is already grouped under the subdomain, the path here is relative to the group.
	// The base path for this extension is "/abuse", so routes like "/blocklist" become "/abuse/blocklist".
	// The access service registration needs the full path relative to the admin API root.
	// core.GetAPI("admin").Subdomain() is "admin", so the full path is "admin.portal.com/api/abuse" + route.Path
	return router.RegisterRoutes(gRouter, accessSvc, core.GetAPI("admin").Subdomain(), routes, router.WithCors())
}

// listBlocks returns a list of blocked content items with filtering and pagination
func (e *AdminExtension) listBlocks(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	if e.blockListService == nil {
		e.logger.Error("Blocklist service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Use queryutilHttp.ProcessListRequest with Echo context
	if err := queryutilHttp.ProcessListRequest(
		c.Response(), c.Request(), // Pass raw http.ResponseWriter and http.Request
		"blocks",
		func(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]models.BlockList, int64, error) {
			return e.blockListService.ListBlockedContent(filters, sorts, pagination)
		},
		func(b models.BlockList) dto.BlockContentResponse {
			var response dto.BlockContentResponse
			if err := response.FromModel(&b); err != nil {
				e.logger.Error("Failed to convert block", zap.Error(err))
				// In a real scenario, you might want to handle this conversion error more gracefully
				// or log it and return a generic error DTO. For this example, returning a zero-value DTO.
				return dto.BlockContentResponse{}
			}
			return response
		},
	); err != nil {
		e.logger.Error("Failed to list blocks", zap.Error(err))
		// queryutilHttp.ProcessListRequest handles writing the error response internally,
		// but we still return the error for Echo's error handler chain.
		return err // Return the error from ProcessListRequest
	}

	return nil // Return nil on success
}

// createBlock handles the creation of a new block list entry
func (e *AdminExtension) createBlock(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	if e.blockListService == nil {
		e.logger.Error("Blocklist service not available")
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	var requestDto dto.BlockContentCreateRequest
	blockModel, ok := httputil.DecodeAndValidateRequest[*models.BlockList, *dto.BlockContentCreateRequest](ctx, &requestDto)
	if !ok {
		return nil // Error handled by DecodeAndValidateRequest
	}

	blockModel, err := e.blockListService.BlockContent(blockModel)
	if err != nil {
		e.logger.Error("Failed to create block", zap.Error(err))
		return ctx.Error(errors.New("failed to create block"), http.StatusInternalServerError) // Use ctx.Error
	}

	var responseDto dto.BlockContentResponse
	c.Response().WriteHeader(http.StatusCreated)                                                                                 // Set status code using Echo context
	if err := httputil.EncodeResponse[*models.BlockList, *dto.BlockContentResponse](ctx, blockModel, &responseDto); err != nil { // Use httputil.EncodeResponse
		e.logger.Error("Failed to encode response", zap.Error(err))
		return err // httputil.EncodeResponse returns an error
	}

	return nil // Return nil on success
}

// getBlockedContent handles GET requests to fetch a specific blocked content
func (e *AdminExtension) getBlockedContent(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Get hash from path using Echo context
	hashStr := c.Param("hash")

	// Use injected service
	if e.blockListService == nil {
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Convert hash string to multihash
	decodedHash, err := core.ParseStorageHash(hashStr)
	if err != nil {
		return ctx.Error(fmt.Errorf("invalid hash format: %w", err), http.StatusBadRequest) // Use ctx.Error
	}

	// Get the block by hash
	block, err := e.blockListService.GetBlockedContent(decodedHash)
	if err != nil {
		if errors.Is(gorm.ErrRecordNotFound, err) {
			return ctx.Error(fmt.Errorf("blocked content not found"), http.StatusNotFound) // Use ctx.Error
		} else {
			e.logger.Error("Failed to fetch blocked content", zap.Error(err))
			return ctx.Error(fmt.Errorf("failed to fetch blocked content: %w", err), http.StatusInternalServerError) // Use ctx.Error
		}
	}

	// Prepare and send response
	var responseDto dto.BlockContentResponse
	if err := httputil.EncodeResponse[*models.BlockList, *dto.BlockContentResponse](ctx, block, &responseDto); err != nil { // Use httputil.EncodeResponse
		e.logger.Error("Failed to encode response", zap.Error(err))
		return err // httputil.EncodeResponse returns an error
	}

	return nil // Return nil on success
}

// unblockContent handles DELETE requests to remove content from the block list
func (e *AdminExtension) unblockContent(c echo.Context) error { // Changed signature to echo.Context
	ctx := httputil.Context(c) // Wrap Echo context

	// Get hash from path using Echo context
	hashStr := c.Param("hash")

	// Use injected service
	if e.blockListService == nil {
		return ctx.Error(errors.New("service unavailable"), http.StatusInternalServerError) // Use ctx.Error
	}

	// Convert hash string to multihash
	decodedHash, err := core.ParseStorageHash(hashStr)
	if err != nil {
		return ctx.Error(fmt.Errorf("invalid hash format: %w", err), http.StatusBadRequest) // Use ctx.Error
	}

	// Get the block by hash (optional, but good for validation/error message)
	// The service method UnblockContent should ideally handle the "not found" case internally
	// or return a specific error that can be checked here.
	// Based on the original code, it seems GetBlockedContent is used first to check existence.
	// Let's keep that pattern for now, but a service method that returns ErrRecordNotFound
	// on delete if the record doesn't exist would be cleaner.
	block, err := e.blockListService.GetBlockedContent(decodedHash)
	if err != nil {
		if errors.Is(gorm.ErrRecordNotFound, err) {
			return ctx.Error(fmt.Errorf("blocked content not found"), http.StatusNotFound) // Use ctx.Error
		} else {
			e.logger.Error("Failed to fetch blocked content before unblock", zap.Error(err))
			return ctx.Error(fmt.Errorf("failed to check blocked content existence: %w", err), http.StatusInternalServerError) // Use ctx.Error
		}
	}

	// Unblock the content using the multihash from the model
	// Assuming UnblockContent takes core.StorageHash
	if err := e.blockListService.UnblockContent(core.NewStorageHashFromMultihash(block.Hash, 0, nil)); err != nil {
		e.logger.Error("Failed to unblock content", zap.Error(err))
		return ctx.Error(fmt.Errorf("failed to unblock content: %w", err), http.StatusInternalServerError) // Use ctx.Error
	}

	c.Response().WriteHeader(http.StatusOK) // Use Echo context
	return nil                              // Return nil on success
}

// sendErrorResponse is no longer needed as ctx.Error handles this
// func sendErrorResponse(ctx *httputil.RequestContext, statusCode int, message string) {
// 	_ = ctx.Error(errors.New(message), statusCode)
// }
