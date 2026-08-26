//go:build webassets_embed && !webassets_ent

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

package teleport

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gravitational/trace"
)

//go:embed webassets/teleport
var embedded embed.FS

// NewWebAssetsFilesystem returns the initialized implementation of
// http.FileSystem interface which can be used to serve Teleport Proxy Web UI
func NewWebAssetsFilesystem() (http.FileSystem, error) {
	wfs, err := fs.Sub(embedded, "webassets/teleport")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return http.FS(wfs), nil
}
