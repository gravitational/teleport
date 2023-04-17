// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package web

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gravitational/trace"
)

// NewDebugFileSystem returns the HTTP file system implementation
func newDebugFileSystem() (http.FileSystem, error) {
	// If the location of the UI changes on disk then this will need to be updated.
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
