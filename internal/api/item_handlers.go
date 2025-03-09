// Package api implements the REST API handlers for the template plugin
package api

import (
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-template/internal"
	"go.lumeweb.com/portal-plugin-template/internal/api/messages"
	pluginConfig "go.lumeweb.com/portal-plugin-template/internal/config"
	"go.lumeweb.com/portal-plugin-template/internal/db/models"
	"go.lumeweb.com/portal-plugin-template/internal/protocol"
	"go.lumeweb.com/portal-plugin-template/internal/service"
	"go.lumeweb.com/portal-plugin-template/internal/templates"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/middleware"
	"go.uber.org/zap"
	"net/http"
	"strconv"
)

// registerItemHandlers sets up all item-related routes and their access control.
// It demonstrates both public and protected endpoints with proper middleware.
// Public endpoints can be accessed without authentication, while protected endpoints
// require a valid JWT token and appropriate access role.
func (a *API) registerItemHandlers(router *mux.Router, accessSvc core.AccessService) {
	// Define routes with their access roles
	routes := []struct {
		Path    string
		Method  string
		Handler http.HandlerFunc
		Access  string
	}{
		{"/api/items", "GET", a.listItems, ""},
		{"/api/items", "POST", a.createItem, core.ACCESS_USER_ROLE},
		{"/api/items/{id:[0-9]+}", "GET", a.getItem, ""},
		{"/api/items/{id:[0-9]+}", "PUT", a.updateItem, core.ACCESS_USER_ROLE},
		{"/api/items/{id:[0-9]+}", "DELETE", a.deleteItem, core.ACCESS_USER_ROLE},
		{"/api/items/search", "GET", a.searchItems, ""},
		{"/api/items/protected", "GET", a.listProtectedItems, core.ACCESS_USER_ROLE},
	}

	// Add upload status route
	routes = append(routes, struct {
		Path    string
		Method  string
		Handler http.HandlerFunc
		Access  string
	}{
		"/api/uploads/{id}", "GET", a.getUploadStatus, core.ACCESS_USER_ROLE,
	})

	// Register routes
	for _, route := range routes {
		r := router.HandleFunc(route.Path, route.Handler).Methods(route.Method)

		if route.Access != "" {
			r.Use(middleware.AuthMiddleware(middleware.AuthMiddlewareOptions{
				Context: a.ctx,
				Purpose: core.JWTPurposeLogin,
			}))
			r.Use(middleware.AccessMiddleware(a.ctx))
		}

		if err := accessSvc.RegisterRoute(a.Subdomain(), route.Path, route.Method, route.Access); err != nil {
			a.logger.Error("failed to register route", zap.Error(err))
		}
	}
}

// listItems handles GET /api/items
// Returns a paginated list of items with total count
func (a *API) listItems(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	pagination := &service.Pagination{
		Page:  1,
		Limit: a.config.GetAPI(internal.PLUGIN_NAME).(*pluginConfig.APIConfig).ItemsPerPage,
	}

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			pagination.Page = p
		}
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			pagination.Limit = l
		}
	}

	items, total, err := a.itemSvc.ListItems(pagination)
	if err != nil {
		_ = ctx.Error(err, http.StatusInternalServerError)
		return
	}

	response := messages.ListItemsResponse{
		Items: items,
		Total: total,
		Page:  pagination.Page,
		Limit: pagination.Limit,
	}
	ctx.Encode(response)
}

// createItem handles POST /api/items
// Creates a new item from the provided request data
func (a *API) createItem(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	var request messages.CreateItemRequest
	if err := ctx.Decode(&request); err != nil {
		return
	}

	item, err := a.itemSvc.CreateItem(request.Name, request.Description)
	if err != nil {
		_ = ctx.Error(err, http.StatusInternalServerError)
		return
	}

	// Get the authenticated user from the request context
	if userID, err := middleware.GetUserFromContext(r.Context()); err == nil {
		// Get user service to lookup user details
		userSvc := core.GetService[core.UserService](a.ctx, core.USER_SERVICE)
		if exists, user, err := userSvc.AccountExists(userID); err == nil && exists {
			// Send notification email
			mailerSvc := a.ctx.Service(core.MAILER_SERVICE).(core.MailerService)

			templateData := core.MailerTemplateData{
				"UserName":    fmt.Sprintf("%s %s", user.FirstName, user.LastName),
				"PortalName":  "Portal",
				"Name":        item.Name,
				"Description": item.Description,
				"ItemURL":     fmt.Sprintf("https://%s/items/%d", a.Subdomain(), item.ID),
			}

			err = mailerSvc.TemplateSend(templates.MAILER_TPL_ITEM_CREATED, templateData, templateData, user.Email)
			if err != nil {
				a.logger.Error("failed to send item created email", zap.Error(err))
				// Don't return error to client, item was still created successfully
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// getItem handles GET /api/items/{id}
// Retrieves a single item by its ID
func (a *API) getItem(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)
	vars := mux.Vars(r)

	id, err := strconv.ParseUint(vars["id"], 10, 64)
	if err != nil {
		_ = ctx.Error(err, http.StatusBadRequest)
		return
	}

	_, err = a.itemSvc.GetItem(id)
	if err != nil {
		_ = ctx.Error(err, http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// updateItem handles PUT /api/items/{id}
// Updates an existing item with the provided data
func (a *API) updateItem(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)
	vars := mux.Vars(r)

	id, err := strconv.ParseUint(vars["id"], 10, 64)
	if err != nil {
		_ = ctx.Error(err, http.StatusBadRequest)
		return
	}

	var request messages.UpdateItemRequest
	if err := ctx.Decode(&request); err != nil {
		return
	}

	err = a.itemSvc.UpdateItem(id, request.Name, request.Description)
	if err != nil {
		_ = ctx.Error(err, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// deleteItem handles DELETE /api/items/{id}
// Removes an item from the database
func (a *API) deleteItem(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)
	vars := mux.Vars(r)

	id, err := strconv.ParseUint(vars["id"], 10, 64)
	if err != nil {
		_ = ctx.Error(err, http.StatusBadRequest)
		return
	}

	if err := a.itemSvc.DeleteItem(id); err != nil {
		_ = ctx.Error(err, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// searchItems handles GET /api/items/search
// Performs a text search on item names and descriptions
func (a *API) searchItems(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	query := r.URL.Query().Get("q")
	if query == "" {
		_ = ctx.Error(errors.New("search query required"), http.StatusBadRequest)
		return
	}

	limit := a.config.GetAPI(internal.PLUGIN_NAME).(*pluginConfig.APIConfig).SearchLimit
	items, total, err := a.itemSvc.SearchItems(query, limit)
	if err != nil {
		_ = ctx.Error(err, http.StatusInternalServerError)
		return
	}

	response := messages.SearchItemsResponse{
		Items: items,
		Total: total,
	}
	ctx.Encode(response)
}

// listProtectedItems handles GET /api/items/protected
// Demonstrates a protected endpoint requiring authentication
func (a *API) listProtectedItems(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)

	var items []models.Item
	if err := a.ctx.DB().Find(&items).Error; err != nil {
		_ = ctx.Error(err, http.StatusInternalServerError)
		return
	}

	response := messages.ListItemsResponse{
		Items: items,
		Total: int64(len(items)),
		Page:  1,
		Limit: len(items),
	}
	ctx.Encode(response)
}

// getUploadStatus handles GET /api/uploads/{id}
// Returns the current status of an upload operation
func (a *API) getUploadStatus(w http.ResponseWriter, r *http.Request) {
	ctx := httputil.Context(r, w)
	vars := mux.Vars(r)
	uploadID := vars["id"]

	if uploadID == "" {
		_ = ctx.Error(errors.New("upload ID required"), http.StatusBadRequest)
		return
	}

	// Get protocol service
	proto := core.GetProtocol(internal.PLUGIN_NAME).(*protocol.Protocol)
	if proto == nil {
		_ = ctx.Error(errors.New("protocol not found"), http.StatusInternalServerError)
		return
	}

	// Get upload state
	state, err := proto.GetUploadStatus(uploadID)
	if err != nil {
		_ = ctx.Error(err, http.StatusNotFound)
		return
	}

	// Convert internal state to API response
	response := messages.UploadStatusResponse{
		State: &messages.UploadState{
			ID:        state.ID,
			Size:      state.Size,
			Uploaded:  state.Uploaded,
			Started:   state.Started,
			Completed: state.Completed,
			Hash:      state.Hash.Multihash().B58String(),
		},
	}

	ctx.Encode(response)
}
