// Package build provides version and build information for the template plugin.
// It integrates with the Portal build system to expose version, git commit,
// build time and other metadata about the plugin build.
package build

import (
	"go.lumeweb.com/portal/build"
)

// Build information variables that are populated during compilation
var (
	Version      string // Semantic version of the plugin
	GitCommit    string // Git commit hash of the build
	GitBranch    string // Git branch name
	BuildTime    string // Timestamp when the plugin was built
	GoVersion    string // Go version used to compile
	Platform     string // Operating system platform
	Architecture string // CPU architecture
)

// GetInfo returns build information about this plugin.
// It implements the Portal BuildInfo interface to provide standardized
// access to version and build metadata across all plugins.
func GetInfo() build.BuildInfo {
	return build.New(Version, GitCommit, GitBranch, BuildTime, GoVersion, Platform, Architecture)
}
