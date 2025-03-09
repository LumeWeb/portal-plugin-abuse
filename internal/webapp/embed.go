// Package webapp provides the frontend web application for the template plugin
package webapp

import "embed"

//go:embed *.html *.js
var Files embed.FS // Embedded frontend files served by the HTTP handler
