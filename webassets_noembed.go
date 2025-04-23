//go:build !webassets_embed

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
	"net/http"

	"github.com/gravitational/trace"
)

const webAssetsMissingError = "the teleport binary was built without web assets, try building with `make release`"

// NewWebAssetsFilesystem is a no-op in this build mode.
func NewWebAssetsFilesystem() (http.FileSystem, error) { //nolint:staticcheck // suppress 'never returns nil' as this is value is platform dependent
	return nil, trace.NotFound("%s", webAssetsMissingError)
}
