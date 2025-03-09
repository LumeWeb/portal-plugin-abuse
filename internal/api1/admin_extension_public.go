package api1

import (
	"fmt"
	"github.com/gorilla/mux"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-abuse/internal/db/models"
	"go.lumeweb.com/portal-plugin-abuse/internal/service"
	typesSvc "go.lumeweb.com/portal-plugin-abuse/internal/types/service"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// registerPublicHandlers registers all public API routes
func (e *AdminExtension) registerPublicHandlers(router *mux.Router, accessSvc core.AccessService) error {
	// Create a subrouter for public endpoints
	publicRouter := router.PathPrefix("/api/abuse/public").Subrouter()

	// Add middleware to validate tokens
	publicRouter.Use(e.validateTokenMiddleware)

	// Define routes for public access
	routes := []struct {
		Path    string
		Method  string
		Handler http.HandlerFunc
		Access  string
	}{
		{"/cases/{reference}", "GET", e.getPublicCase, "PUBLIC"},
		{"/cases/{reference}/comment", "POST", e.addPublicComment, "PUBLIC"},
		{"/cases/{reference}/upload", "POST", e.uploadPublicFile, "PUBLIC"},
		{"/validate-token", "POST", e.validateToken, "PUBLIC"}, // This endpoint doesn't require the middleware
	}

	// Register routes
	for _, route := range routes {
		// For token validation, we use a direct route without the middleware
		if route.Path == "/validate-token" {
			router.HandleFunc("/api/abuse/public"+route.Path, route.Handler).Methods(route.Method)
		} else {
			publicRouter.HandleFunc(route.Path, route.Handler).Methods(route.Method)
		}

		// Register route with access service
		fullPath := "/api/abuse/public" + route.Path
		if err := accessSvc.RegisterRoute("abuse", fullPath, route.Method, route.Access); err != nil {
			return fmt.Errorf("failed to register route %s: %w", fullPath, err)
		}
	}

	// Register refresh token endpoint
	router.HandleFunc("/api/abuse/public/refresh-token", e.refreshToken).Methods("POST")
	if err := accessSvc.RegisterRoute("abuse", "/api/abuse/public/refresh-token", "POST", "PUBLIC"); err != nil {
		return fmt.Errorf("failed to register route: %w", err)
	}

	return nil
}


// parseInt safely parses an integer from string
func parseInt(s string) (int, error) {
	var value int
	_, err := fmt.Sscanf(s, "%d", &value)
	return value, err
}
