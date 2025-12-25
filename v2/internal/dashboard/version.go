package dashboard

// These variables are set at build time via ldflags
// Source of truth: VERSION file at repo root
// Example: go build -ldflags "-X github.com/markus-barta/nixfleet/v2/internal/dashboard.Version=$(cat VERSION)"
var (
	// Version is the semantic version, set via ldflags at build time
	// Default "dev" for local builds without ldflags
	Version = "dev"

	// GitCommit is the git commit hash, set via ldflags at build time
	GitCommit = "unknown"

	// BuildTime is the build timestamp, set via ldflags at build time
	BuildTime = "unknown"
)

// VersionInfo returns a formatted version string for display
func VersionInfo() string {
	if GitCommit != "unknown" && len(GitCommit) > 7 {
		return Version + " (" + GitCommit[:7] + ")"
	}
	return Version
}

