package web

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
)

// NewDebugFileSystem returns the HTTP file system implementation
func newDebugFileSystem() (http.FileSystem, error) {
	assetsPath := "../../webassets/teleport"

	// Ensure we have the built assets available before continuing.
	for _, af := range []string{"index.html", "/app"} {
		_, err := os.Stat(filepath.Join(assetsPath, af))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	log.Infof("Using filesystem for serving web assets: %s.", assetsPath)

	return http.Dir(assetsPath), nil
}
