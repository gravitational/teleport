package teleport

import (
	"embed"
	"path/filepath"
)

//go:embed embed.assets
var embedFS embed.FS

func EmbedFS() embed.FS {
	return embedFS
}

var (
	EmbedAssetsBuildDir       = "embed.assets"
	EnhancedRecordingBuildDir = filepath.Join(EmbedAssetsBuildDir, "enhancedrecording")
	RestrictedSessionBuildDir = filepath.Join(EmbedAssetsBuildDir, "restrictedsession")
	WebAssetsBuildDir         = filepath.Join(EmbedAssetsBuildDir, "web")
)
