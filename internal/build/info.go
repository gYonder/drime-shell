package build

// These variables are set by GoReleaser at build time via -ldflags
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
