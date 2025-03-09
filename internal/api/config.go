// Package api implements configuration-related functionality for the API
package api

import (
	"go.lumeweb.com/portal-plugin-template/internal"
	pluginConfig "go.lumeweb.com/portal-plugin-template/internal/config"
	"go.lumeweb.com/portal/config"
	"go.lumeweb.com/portal/core"
)

// Config returns the plugin's configuration structure
func (a *API) Config() config.APIConfig {
	return &pluginConfig.APIConfig{}
}

// Name returns the plugin's name identifier
func (a *API) Name() string {
	return internal.PLUGIN_NAME
}

// Subdomain returns the plugin name as the subdomain
func (a *API) Subdomain() string {
	return internal.PLUGIN_NAME
}

// AuthTokenName returns the name of the authentication token used by the plugin
func (a *API) AuthTokenName() string {
	return core.AUTH_COOKIE_NAME
}
