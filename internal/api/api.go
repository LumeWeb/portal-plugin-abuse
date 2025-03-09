// Package api implements the core API functionality for the template plugin.
// It provides the main API structure and configuration for HTTP routing.
package api

import (
	_ "embed"
	"github.com/gorilla/mux"
	"go.lumeweb.com/portal-plugin-template/internal/service"
	"go.lumeweb.com/portal-plugin-template/internal/webapp"
	"go.lumeweb.com/portal/config"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/middleware"
	"go.lumeweb.com/portal/middleware/swagger"
	"net/http"
	"strings"
)

//go:embed swagger.yaml
var swagSpec []byte

// Verify API implements the core.API interface
var _ core.API = (*API)(nil)

// API represents the main API structure for the template plugin.
// It holds references to core services and configuration needed for operation.
type API struct {
	ctx     core.Context   // Portal context for accessing core services
	config  config.Manager // Configuration manager
	logger  *core.Logger   // Logger instance for this API
	itemSvc service.ItemService
}

// NewAPI creates a new instance of the template plugin API.
// It returns the API instance and context builder options needed for initialization.
func NewAPI() (*API, []core.ContextBuilderOption, error) {
	api := &API{}

	// Define startup configuration
	opts := core.ContextOptions(
		core.ContextWithStartupFunc(func(ctx core.Context) error {
			// Initialize API with context and services
			api.ctx = ctx
			api.config = ctx.Config()
			api.logger = ctx.APILogger(api)
			api.itemSvc = core.GetService[service.ItemService](ctx, service.ITEM_SERVICE)
			return nil
		}),
	)

	return api, opts, nil
}

// Configure sets up all routes and middleware for the API.
// It handles both API endpoints and static file serving for the webapp.
func (a *API) Configure(router *mux.Router, accessSvc core.AccessService) error {
	// Set up Swagger documentation
	if err := swagger.Swagger(swagSpec, router); err != nil {
		return err
	}

	// Configure CORS middleware for all routes
	corsHandler := middleware.CorsMiddleware(nil)
	router.Use(corsHandler)

	// Register all API routes with access control
	a.registerItemHandlers(router, accessSvc)

	// Set up static file serving for the webapp
	httpHandler := http.FileServer(http.FS(webapp.Files))

	// Serve static assets
	router.PathPrefix("/assets/").Handler(httpHandler)

	// Serve the SPA index.html for all non-API routes
	router.PathPrefix("/").MatcherFunc(func(r *http.Request, rm *mux.RouteMatch) bool {
		return !strings.HasPrefix(r.URL.Path, "/api/")
	}).Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reset path to serve index.html
		r.URL.Path = "/"
		httpHandler.ServeHTTP(w, r)
	}))

	return nil
}
