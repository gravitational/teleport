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
	"path"
	"slices"
	"time"

	"github.com/gravitational/teleport/lib/httplib"
)

// makeCacheHandler sets cache headers for cacheable file types.
func makeCacheHandler(handler http.Handler, etag string) http.Handler {
	cachedFileTypes := []string{".woff", ".woff2", ".ttf"}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// We can cache fonts "permanently" because we don't expect them to change. The rest of our
		// assets will have an ETag associated with them (teleport version) that will allow us
		// to conditionally send the updated assets or a 304 status (Not Modified) response
		if slices.Contains(cachedFileTypes, path.Ext(r.URL.Path)) {
			httplib.SetCacheHeaders(w.Header(), time.Hour*24*365 /* one year */)
		} else {
			httplib.SetEntityTagCacheHeaders(w.Header(), etag)
		}

		handler.ServeHTTP(w, r)
	})
}
