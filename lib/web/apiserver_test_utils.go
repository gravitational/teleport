/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

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
