/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package app

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestNeedsPathRedirect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		uri          string
		requestPath  string
		wantRedirect bool
		wantLocation string
	}{
		{
			name:         "no path",
			uri:          "http://backend:9000",
			requestPath:  "/",
			wantRedirect: false,
		},
		{
			name:         "root path",
			uri:          "http://backend:9000/",
			requestPath:  "/",
			wantRedirect: false,
		},
		{
			name:         "path without trailing slash",
			uri:          "http://backend:9000/app",
			requestPath:  "/",
			wantRedirect: true,
			wantLocation: "https://public:443/app",
		},
		{
			name:         "path with trailing slash",
			uri:          "http://backend:9000/app/",
			requestPath:  "/",
			wantRedirect: true,
			wantLocation: "https://public:443/app/",
		},
		{
			name:         "deep path with trailing slash",
			uri:          "http://backend:9000/a/b/c/",
			requestPath:  "/",
			wantRedirect: true,
			wantLocation: "https://public:443/a/b/c/",
		},
		{
			name:         "non-root request",
			uri:          "http://backend:9000/app/",
			requestPath:  "/app/",
			wantRedirect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			parsedURI, err := url.Parse(tt.uri)
			require.NoError(t, err)

			tr := &transport{
				transportConfig: &transportConfig{
					app: &types.AppV3{
						Spec: types.AppSpecV3{
							PublicAddr: "public",
						},
					},
					publicPort: "443",
				},
				uri: parsedURI,
			}

			req := &http.Request{URL: &url.URL{Path: tt.requestPath}}
			location, ok := tr.needsPathRedirect(req)
			require.Equal(t, tt.wantRedirect, ok)
			if tt.wantRedirect {
				require.Equal(t, tt.wantLocation, location)
			} else {
				require.Empty(t, location)
			}
		})
	}
}
